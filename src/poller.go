package main

import (
	"sync"
	"time"
)

// pollTick is how often the poll loop wakes up to check for due work. It only
// bounds the reaction time of the configured refresh interval and the
// management kick.
const pollTick = 5 * time.Second

// refreshAccount polls every /alpha/* route for one account and applies the
// results. Network calls happen outside the pool lock; only the final state
// update is serialized.
func refreshAccount(p *pool, cfg settings, client *apiClient, acct *account) {
	p.mu.Lock()
	cachedOrgID := p.snapshotFor(acct).orgID()
	p.mu.Unlock()

	errors := make(map[string]string)
	var (
		whoami  *whoamiResponse
		sub     *subscriptionData
		credits *creditsResponse
		summary *usageSummary
	)

	if resp, errWhoami := client.whoami(acct.apiKey); errWhoami != nil {
		errors[endpointWhoami] = errWhoami.Error()
	} else {
		whoami = resp
	}
	orgID := cachedOrgID
	if whoami != nil && whoami.Org != nil {
		orgID = whoami.Org.ID
	}

	if data, errSub := client.subscription(acct.apiKey, orgID); errSub != nil {
		errors[endpointSubscriptions] = errSub.Error()
	} else {
		sub = data
	}

	if resp, errCredits := client.credits(acct.apiKey, orgID); errCredits != nil {
		errors[endpointCredits] = errCredits.Error()
	} else {
		credits = resp
	}

	since := ""
	if sub != nil {
		since = sub.CurrentPeriodStart
	}
	if cfg.IncludeSummary {
		if resp, errSummary := client.summary(acct.apiKey, orgID, since); errSummary != nil {
			errors[endpointUsageSummary] = errSummary.Error()
		} else {
			summary = resp
		}
	}

	now := time.Now()
	p.mu.Lock()
	defer p.mu.Unlock()
	snap := p.snapshotFor(acct)
	snap.AttemptedAt = now
	snap.Errors = errors
	if whoami != nil {
		snap.Whoami = whoami
	}
	if sub != nil || errors[endpointSubscriptions] == "" {
		// A clean response with no subscription clears stale plan data.
		snap.Subscription = sub
	}
	if credits != nil {
		snap.Credits = credits
		snap.RefreshedAt = now
	}
	if summary != nil {
		snap.Summary = summary
		snap.SummarySince = since
	}

	if len(errors) > 0 {
		fields := map[string]any{"account": acct.Name}
		for endpoint, message := range errors {
			fields[endpoint] = message
		}
		hostLog("warn", "usage refresh completed with errors", fields)
	}
}

// poller owns the background refresh goroutine. It is started idempotently on
// register/reconfigure and stopped on plugin shutdown.
type poller struct {
	mu      sync.Mutex
	stop    chan struct{}
	kick    chan struct{}
	running bool
}

var globalPoller = &poller{}

func startPoller() {
	globalPoller.mu.Lock()
	defer globalPoller.mu.Unlock()
	if globalPoller.running {
		return
	}
	globalPoller.stop = make(chan struct{})
	globalPoller.kick = make(chan struct{}, 1)
	globalPoller.running = true
	go pollLoop(globalPoller.stop, globalPoller.kick)
}

func stopPoller() {
	globalPoller.mu.Lock()
	defer globalPoller.mu.Unlock()
	if !globalPoller.running {
		return
	}
	close(globalPoller.stop)
	globalPoller.running = false
}

// kickPoller requests an immediate refresh pass (used by the management API).
func kickPoller() {
	globalPoller.mu.Lock()
	kick := globalPoller.kick
	running := globalPoller.running
	globalPoller.mu.Unlock()
	if !running {
		return
	}
	select {
	case kick <- struct{}{}:
	default:
	}
}

func pollLoop(stop <-chan struct{}, kick <-chan struct{}) {
	var lastRefresh time.Time
	for {
		p := currentPool()
		p.mu.Lock()
		cfg := p.cfg
		targets := p.enabledAccounts()
		p.mu.Unlock()

		if len(targets) > 0 && time.Since(lastRefresh) >= cfg.RefreshInterval {
			lastRefresh = time.Now()
			client := newAPIClient(cfg)
			for _, acct := range targets {
				refreshAccount(p, cfg, client, acct)
			}
		}

		select {
		case <-stop:
			return
		case <-kick:
			lastRefresh = time.Time{}
		case <-time.After(pollTick):
		}
	}
}
