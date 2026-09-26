package main

import (
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"time"
)

// keyHash is the stable state key of an account. Hashing keeps the API key out
// of memory-resident maps that could be dumped in diagnostics.
func keyHash(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])[:12]
}

// endpoint labels used in snapshot.Errors and log fields.
const (
	endpointWhoami        = "whoami"
	endpointSubscriptions = "subscriptions"
	endpointCredits       = "credits"
	endpointUsageSummary  = "usage_summary"
)

// snapshot is the last known CommandCode reading for one account. Stored values
// are replaced only by newer successful readings, so a transient API failure
// does not erase good data.
type snapshot struct {
	// AttemptedAt is when the last poll cycle for this account finished.
	AttemptedAt time.Time
	// RefreshedAt is when the authoritative quota source (credits) last
	// succeeded. Staleness is measured from this timestamp.
	RefreshedAt time.Time

	Whoami       *whoamiResponse
	Subscription *subscriptionData
	Credits      *creditsResponse
	Summary      *usageSummary
	// SummarySince records the billing-period start used for Summary.
	SummarySince string

	// Errors maps an endpoint label to the most recent failure message.
	Errors map[string]string
}

func newSnapshot() *snapshot {
	return &snapshot{Errors: make(map[string]string)}
}

// orgID returns the cached organization id used as the orgId query parameter.
func (s *snapshot) orgID() string {
	if s == nil || s.Whoami == nil || s.Whoami.Org == nil {
		return ""
	}
	return s.Whoami.Org.ID
}

// pool is the plugin's global state. Every field is guarded by mu.
type pool struct {
	mu sync.Mutex

	cfg         settings
	accounts    []*account
	snaps       map[string]*snapshot
	configError string
}

var globalPool = &pool{snaps: make(map[string]*snapshot)}

func currentPool() *pool {
	return globalPool
}

// reconfigure rebuilds the account set from settings, keeping snapshots for
// accounts whose API key is unchanged.
func (p *pool) reconfigure(cfg settings, accounts []*account, configErr error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.cfg = cfg
	p.accounts = accounts
	if configErr != nil {
		p.configError = configErr.Error()
	} else {
		p.configError = ""
	}
	seen := make(map[string]bool, len(accounts))
	for _, acct := range accounts {
		id := acct.identity()
		seen[id] = true
		if _, ok := p.snaps[id]; !ok {
			p.snaps[id] = newSnapshot()
		}
	}
	for id := range p.snaps {
		if !seen[id] {
			delete(p.snaps, id)
		}
	}
}

// snapshotFor returns the mutable snapshot of an account. Caller must hold p.mu.
func (p *pool) snapshotFor(acct *account) *snapshot {
	id := acct.identity()
	snap, ok := p.snaps[id]
	if !ok {
		snap = newSnapshot()
		p.snaps[id] = snap
	}
	return snap
}

// enabledAccounts returns the pollable accounts. Caller must hold p.mu.
func (p *pool) enabledAccounts() []*account {
	out := make([]*account, 0, len(p.accounts))
	for _, acct := range p.accounts {
		if !acct.Disabled {
			out = append(out, acct)
		}
	}
	return out
}
