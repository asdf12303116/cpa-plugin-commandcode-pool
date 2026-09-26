package main

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v7/sdk/pluginapi"
)

const (
	pluginID           = "commandcode-pool"
	managementBasePath = "/plugins/" + pluginID

	// windowFiveHour is CommandCode's rolling 5-hour window; windowWeekly is
	// its weekly window. Both are measured in credit/USD-equivalent value.
	windowFiveHour = "5h"
	windowWeekly   = "weekly"
)

var windowNames = [...]string{windowFiveHour, windowWeekly}

func handleManagementRegister() ([]byte, error) {
	return okEnvelope(pluginapi.ManagementRegistrationResponse{
		Routes: []pluginapi.ManagementRoute{
			{Method: http.MethodGet, Path: managementBasePath + "/status"},
			{Method: http.MethodGet, Path: managementBasePath + "/plans"},
			{Method: http.MethodPost, Path: managementBasePath + "/refresh"},
		},
		Resources: []pluginapi.ResourceRoute{
			{
				Path:        "/status",
				Menu:        "CommandCode Pool",
				Description: "CommandCode plan status, credit balance, and 5-hour/weekly window usage.",
			},
		},
	})
}

// ---- response schema ----

type statusResponse struct {
	Version             string               `json:"version"`
	GeneratedAt         string               `json:"generated_at"`
	APIBaseURL          string               `json:"api_base_url"`
	RefreshInterval     string               `json:"refresh_interval"`
	StaleAfter          string               `json:"stale_after"`
	WarnPercent         int                  `json:"warn_percent"`
	CriticalPercent     int                  `json:"critical_percent"`
	IncludeUsageSummary bool                 `json:"include_usage_summary"`
	Accounts            []accountStatus      `json:"accounts"`
	SharedSubscriptions []sharedSubscription `json:"shared_subscriptions,omitempty"`
	ConfigError         string               `json:"config_error,omitempty"`
}

type accountStatus struct {
	Name        string   `json:"name"`
	KeySuffix   string   `json:"key_suffix"`
	Providers   []string `json:"providers"`
	Disabled    bool     `json:"disabled,omitempty"`
	Stale       bool     `json:"stale"`
	RefreshedAt string   `json:"refreshed_at,omitempty"`
	AttemptedAt string   `json:"attempted_at,omitempty"`
	// DataAgeSeconds is the age of the authoritative quota reading.
	DataAgeSeconds *int64 `json:"data_age_seconds,omitempty"`

	Identity      accountIdentity         `json:"identity"`
	Plan          planStatus              `json:"plan"`
	Balance       balanceStatus           `json:"balance"`
	Windows       map[string]windowStatus `json:"windows"`
	PeriodSummary *summaryStatus          `json:"period_summary,omitempty"`
	Errors        map[string]string       `json:"errors,omitempty"`
}

type accountIdentity struct {
	UserID   string          `json:"user_id,omitempty"`
	UserName string          `json:"user_name,omitempty"`
	Name     string          `json:"name,omitempty"`
	Email    string          `json:"email,omitempty"`
	OrgID    string          `json:"org_id,omitempty"`
	Limits   json.RawMessage `json:"limits,omitempty"`
}

type planStatus struct {
	ID                   string          `json:"id,omitempty"`
	Name                 string          `json:"name,omitempty"`
	MarketingName        string          `json:"marketing_name,omitempty"`
	Known                bool            `json:"known"`
	Status               string          `json:"status,omitempty"`
	SubscriptionID       string          `json:"subscription_id,omitempty"`
	PriceID              string          `json:"price_id,omitempty"`
	Quantity             int             `json:"quantity,omitempty"`
	CancelAtPeriodEnd    bool            `json:"cancel_at_period_end,omitempty"`
	CreatedAt            string          `json:"created_at,omitempty"`
	CurrentPeriodStart   string          `json:"current_period_start,omitempty"`
	CurrentPeriodEnd     string          `json:"current_period_end,omitempty"`
	DaysRemaining        *int            `json:"days_remaining,omitempty"`
	PeriodElapsedPercent *float64        `json:"period_elapsed_percent,omitempty"`
	EndedAt              string          `json:"ended_at,omitempty"`
	CancelAt             string          `json:"cancel_at,omitempty"`
	CanceledAt           string          `json:"canceled_at,omitempty"`
	PendingPhase         json.RawMessage `json:"pending_phase,omitempty"`
	// SharedKeys counts discovered API keys sharing this subscription id.
	SharedKeys int `json:"shared_keys,omitempty"`
}

type balanceStatus struct {
	MonthlyRemaining       *float64 `json:"monthly_remaining,omitempty"`
	MonthlyIncluded        *float64 `json:"monthly_included,omitempty"`
	MonthlyConsumed        *float64 `json:"monthly_consumed,omitempty"`
	MonthlyConsumedPercent *float64 `json:"monthly_consumed_percent,omitempty"`
	PurchasedCredits       *float64 `json:"purchased_credits,omitempty"`
	FreeCredits            *float64 `json:"free_credits,omitempty"`
	TotalRemaining         *float64 `json:"total_remaining,omitempty"`
	BelowThreshold         *bool    `json:"below_threshold,omitempty"`
	CreditThreshold        *float64 `json:"credit_threshold,omitempty"`
	Limited                *bool    `json:"limited,omitempty"`
	WindowExceeded         *bool    `json:"window_exceeded,omitempty"`
	SandboxAccess          *bool    `json:"sandbox_access,omitempty"`
	SandboxMinutes         *float64 `json:"sandbox_minutes,omitempty"`
}

type windowStatus struct {
	Name        string   `json:"name"`
	Used        *float64 `json:"used,omitempty"`
	Cap         *float64 `json:"cap,omitempty"`
	Headroom    *float64 `json:"headroom,omitempty"`
	UsedPercent *float64 `json:"used_percent,omitempty"`
	Exceeded    *bool    `json:"exceeded,omitempty"`
	Blocked     bool     `json:"blocked"`
	// BlockedEffective is false while prepaid credits bypass window caps.
	BlockedEffective bool   `json:"blocked_effective"`
	ResetAt          string `json:"reset_at,omitempty"`
	ResetsInSeconds  *int64 `json:"resets_in_seconds,omitempty"`
	ResetsIn         string `json:"resets_in,omitempty"`
}

type summaryStatus struct {
	Since                 string  `json:"since,omitempty"`
	PeriodBasis           string  `json:"period_basis,omitempty"`
	Requests              int64   `json:"requests"`
	Completed             int64   `json:"completed"`
	Failed                int64   `json:"failed"`
	SuccessRate           float64 `json:"success_rate"`
	TotalCost             float64 `json:"total_cost"`
	AverageCost           float64 `json:"average_cost"`
	TotalCredits          float64 `json:"total_credits"`
	TotalMonthlyCredits   float64 `json:"total_monthly_credits"`
	TotalPurchasedCredits float64 `json:"total_purchased_credits"`
	TotalFreeCredits      float64 `json:"total_free_credits"`
	TokensIn              int64   `json:"tokens_in"`
	TokensOut             int64   `json:"tokens_out"`
	TokensTotal           int64   `json:"tokens_total"`
}

type sharedSubscription struct {
	SubscriptionID string   `json:"subscription_id"`
	PlanID         string   `json:"plan_id,omitempty"`
	Accounts       []string `json:"accounts"`
}

type planCatalogResponse struct {
	GeneratedAt string     `json:"generated_at"`
	Plans       []planInfo `json:"plans"`
}

// ---- small helpers ----

func boolPtr(v bool) *bool    { return &v }
func intPtr(v int) *int       { return &v }
func int64Ptr(v int64) *int64 { return &v }

func round2(v float64) float64 { return math.Round(v*100) / 100 }
func round6(v float64) float64 { return math.Round(v*1e6) / 1e6 }

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}

func parseTime(raw string) time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		if parsed, errParse := time.Parse(layout, raw); errParse == nil {
			return parsed
		}
	}
	return time.Time{}
}

func humanDuration(seconds int64) string {
	if seconds <= 0 {
		return "now"
	}
	duration := time.Duration(seconds) * time.Second
	days := duration / (24 * time.Hour)
	duration -= days * 24 * time.Hour
	hours := duration / time.Hour
	duration -= hours * time.Hour
	minutes := duration / time.Minute
	switch {
	case days > 0:
		return fmt.Sprintf("%dd %dh", days, hours)
	case hours > 0:
		return fmt.Sprintf("%dh %dm", hours, minutes)
	case minutes > 0:
		return fmt.Sprintf("%dm", minutes)
	default:
		return fmt.Sprintf("%ds", seconds)
	}
}

// maskEmail keeps the domain and a hint of the local part so the dashboard can
// identify an account without publishing a full address.
func maskEmail(email string) string {
	email = strings.TrimSpace(email)
	if email == "" {
		return ""
	}
	at := strings.LastIndex(email, "@")
	if at <= 0 {
		return "***"
	}
	local := email[:at]
	switch {
	case len(local) <= 2:
		local = "***"
	case len(local) <= 4:
		local = local[:1] + "***"
	default:
		local = local[:2] + "***" + local[len(local)-1:]
	}
	return local + email[at:]
}

// ---- derivation ----

func buildPlanStatus(sub *subscriptionData, now time.Time, sharedCount int) planStatus {
	out := planStatus{}
	if sub == nil {
		return out
	}
	out.ID = sub.PlanID
	out.Status = sub.Status
	out.SubscriptionID = sub.ID
	out.PriceID = sub.PriceID
	out.Quantity = sub.Quantity
	out.CancelAtPeriodEnd = sub.CancelAtPeriodEnd
	out.CreatedAt = sub.CreatedAt
	out.CurrentPeriodStart = sub.CurrentPeriodStart
	out.CurrentPeriodEnd = sub.CurrentPeriodEnd
	out.PendingPhase = sub.PendingPhase
	if sub.EndedAt != nil {
		out.EndedAt = *sub.EndedAt
	}
	if sub.CancelAt != nil {
		out.CancelAt = *sub.CancelAt
	}
	if sub.CanceledAt != nil {
		out.CanceledAt = *sub.CanceledAt
	}
	info, known := lookupPlan(sub.PlanID)
	out.Name = info.Name
	out.MarketingName = info.MarketingName
	out.Known = known
	if sharedCount > 1 {
		out.SharedKeys = sharedCount
	}
	start := parseTime(sub.CurrentPeriodStart)
	end := parseTime(sub.CurrentPeriodEnd)
	if !start.IsZero() && !end.IsZero() && end.After(start) {
		elapsed := now.Sub(start)
		if elapsed < 0 {
			elapsed = 0
		}
		percent := float64(elapsed) / float64(end.Sub(start)) * 100
		if percent > 100 {
			percent = 100
		}
		out.PeriodElapsedPercent = floatPtr(round2(percent))
		days := int(end.Sub(now).Hours() / 24)
		if days < 0 {
			days = 0
		}
		out.DaysRemaining = intPtr(days)
	}
	return out
}

func buildBalanceStatus(cr *creditsResponse, sub *subscriptionData) balanceStatus {
	out := balanceStatus{}
	if cr == nil {
		return out
	}
	if cr.Credits != nil {
		out.MonthlyRemaining = floatPtr(round6(cr.Credits.MonthlyCredits))
		out.PurchasedCredits = floatPtr(round6(cr.Credits.PurchasedCredits))
		out.FreeCredits = floatPtr(round6(cr.Credits.FreeCredits))
		total := cr.Credits.MonthlyCredits + cr.Credits.PurchasedCredits + cr.Credits.FreeCredits
		out.TotalRemaining = floatPtr(round6(total))
		out.BelowThreshold = boolPtr(cr.Credits.BelowThreshold)
		out.CreditThreshold = floatPtr(round6(cr.Credits.CreditThreshold))
	}
	if cr.WindowLimits != nil {
		out.Limited = cr.WindowLimits.Limited
		out.WindowExceeded = cr.WindowLimits.Exceeded
	}
	out.SandboxAccess = cr.SandboxAccess
	out.SandboxMinutes = cr.SandboxMinutes

	// monthlyCredits is the REMAINING allowance, so consumption is derived from
	// the static plan total minus the remaining balance.
	if sub != nil && strings.EqualFold(strings.TrimSpace(sub.Status), "active") {
		if info, known := lookupPlan(sub.PlanID); known && info.IncludesCredits {
			included := info.MonthlyCredits
			out.MonthlyIncluded = floatPtr(included)
			remaining := 0.0
			if cr.Credits != nil {
				remaining = cr.Credits.MonthlyCredits
			}
			consumed := included - remaining
			if consumed < 0 {
				consumed = 0
			}
			out.MonthlyConsumed = floatPtr(round6(consumed))
			if included > 0 {
				percent := consumed / included * 100
				if percent > 100 {
					percent = 100
				}
				out.MonthlyConsumedPercent = floatPtr(round2(percent))
			}
		}
	}
	return out
}

func buildWindows(cr *creditsResponse, now time.Time) map[string]windowStatus {
	out := make(map[string]windowStatus, len(windowNames))
	for _, name := range windowNames {
		out[name] = windowStatus{Name: name}
	}
	if cr == nil || cr.WindowLimits == nil {
		return out
	}
	purchased := 0.0
	if cr.Credits != nil {
		purchased = cr.Credits.PurchasedCredits
	}
	out[windowFiveHour] = buildWindow(windowFiveHour, cr.WindowLimits.FiveHour, purchased, now)
	out[windowWeekly] = buildWindow(windowWeekly, cr.WindowLimits.Weekly, purchased, now)
	return out
}

func buildWindow(name string, w *limitWindow, purchased float64, now time.Time) windowStatus {
	out := windowStatus{Name: name}
	if w == nil {
		return out
	}
	out.Used = floatPtr(round6(w.Used))
	out.Cap = floatPtr(round6(w.Cap))
	if w.Cap > 0 {
		out.Headroom = floatPtr(round6(w.Cap - w.Used))
		out.UsedPercent = floatPtr(round2(w.Used / w.Cap * 100))
	}
	out.Exceeded = w.Exceeded
	// `exceeded` is nullable and null means "no signal", never "false", so the
	// used-vs-cap comparison is the authoritative fallback.
	blocked := (w.Exceeded != nil && *w.Exceeded) || (w.Cap > 0 && w.Used >= w.Cap)
	out.Blocked = blocked
	out.BlockedEffective = blocked && purchased <= 0
	if reset := epochMillisToTime(w.ResetAt); !reset.IsZero() {
		out.ResetAt = reset.Format(time.RFC3339)
		seconds := int64(reset.Sub(now).Seconds())
		out.ResetsInSeconds = int64Ptr(seconds)
		out.ResetsIn = humanDuration(seconds)
	}
	return out
}

func buildSummaryStatus(sum *usageSummary, since string) *summaryStatus {
	if sum == nil {
		return nil
	}
	return &summaryStatus{
		Since:                 strings.TrimSpace(since),
		PeriodBasis:           sum.PeriodBasis,
		Requests:              sum.TotalCount,
		Completed:             sum.CompletedCount,
		Failed:                sum.FailedCount,
		SuccessRate:           round2(sum.SuccessRate),
		TotalCost:             round6(sum.TotalCost),
		AverageCost:           sum.AverageCost,
		TotalCredits:          round6(sum.TotalCredits),
		TotalMonthlyCredits:   round6(sum.TotalMonthlyCredits),
		TotalPurchasedCredits: round6(sum.TotalPurchasedCredits),
		TotalFreeCredits:      round6(sum.TotalFreeCredits),
		TokensIn:              sum.TotalTokensIn,
		TokensOut:             sum.TotalTokensOut,
		TokensTotal:           sum.TotalTokens,
	}
}

func buildAccountStatus(acct *account, snap *snapshot, cfg settings, now time.Time, sharedCount int) accountStatus {
	out := accountStatus{
		Name:        acct.Name,
		KeySuffix:   acct.KeySuffix,
		Providers:   append([]string(nil), acct.Providers...),
		Disabled:    acct.Disabled,
		Plan:        buildPlanStatus(snap.Subscription, now, sharedCount),
		Balance:     buildBalanceStatus(snap.Credits, snap.Subscription),
		Windows:     buildWindows(snap.Credits, now),
		RefreshedAt: formatTime(snap.RefreshedAt),
		AttemptedAt: formatTime(snap.AttemptedAt),
	}
	if snap.Whoami != nil {
		out.Identity = accountIdentity{
			UserID:   snap.Whoami.User.ID,
			UserName: snap.Whoami.User.UserName,
			Name:     snap.Whoami.User.Name,
			Email:    maskEmail(snap.Whoami.User.Email),
			Limits:   snap.Whoami.Limits,
		}
		if snap.Whoami.Org != nil {
			out.Identity.OrgID = snap.Whoami.Org.ID
		}
	}
	if len(snap.Errors) > 0 {
		out.Errors = make(map[string]string, len(snap.Errors))
		for endpoint, message := range snap.Errors {
			out.Errors[endpoint] = message
		}
	}
	out.PeriodSummary = buildSummaryStatus(snap.Summary, snap.SummarySince)

	switch {
	case !snap.RefreshedAt.IsZero():
		age := int64(now.Sub(snap.RefreshedAt).Seconds())
		if age < 0 {
			age = 0
		}
		out.DataAgeSeconds = int64Ptr(age)
		out.Stale = cfg.StaleAfter > 0 && now.Sub(snap.RefreshedAt) > cfg.StaleAfter
	case !snap.AttemptedAt.IsZero():
		out.Stale = cfg.StaleAfter > 0 && now.Sub(snap.AttemptedAt) > cfg.StaleAfter
	}
	return out
}

func buildStatus() statusResponse {
	p := currentPool()
	p.mu.Lock()
	defer p.mu.Unlock()

	now := time.Now()
	cfg := p.cfg
	out := statusResponse{
		Version:             pluginVersion,
		GeneratedAt:         now.Format(time.RFC3339),
		APIBaseURL:          cfg.APIBaseURL,
		RefreshInterval:     cfg.RefreshInterval.String(),
		StaleAfter:          cfg.StaleAfter.String(),
		WarnPercent:         cfg.WarnPercent,
		CriticalPercent:     cfg.CriticalPercent,
		IncludeUsageSummary: cfg.IncludeSummary,
		ConfigError:         p.configError,
		Accounts:            make([]accountStatus, 0, len(p.accounts)),
	}

	sharing := make(map[string]int)
	for _, acct := range p.accounts {
		snap := p.snapshotFor(acct)
		if snap.Subscription != nil && snap.Subscription.ID != "" {
			sharing[snap.Subscription.ID]++
		}
	}
	for _, acct := range p.accounts {
		snap := p.snapshotFor(acct)
		sharedCount := 0
		if snap.Subscription != nil {
			sharedCount = sharing[snap.Subscription.ID]
		}
		out.Accounts = append(out.Accounts, buildAccountStatus(acct, snap, cfg, now, sharedCount))
	}

	for id, count := range sharing {
		if count < 2 {
			continue
		}
		group := sharedSubscription{SubscriptionID: id, Accounts: []string{}}
		for _, acct := range p.accounts {
			snap := p.snapshotFor(acct)
			if snap.Subscription == nil || snap.Subscription.ID != id {
				continue
			}
			group.Accounts = append(group.Accounts, acct.Name)
			if group.PlanID == "" {
				group.PlanID = snap.Subscription.PlanID
			}
		}
		sort.Strings(group.Accounts)
		out.SharedSubscriptions = append(out.SharedSubscriptions, group)
	}
	sort.Slice(out.SharedSubscriptions, func(i, j int) bool {
		return out.SharedSubscriptions[i].SubscriptionID < out.SharedSubscriptions[j].SubscriptionID
	})
	return out
}

// ---- HTTP plumbing ----

func jsonResponse(status int, v any) ([]byte, error) {
	body, errMarshal := json.Marshal(v)
	if errMarshal != nil {
		return nil, errMarshal
	}
	return okEnvelope(pluginapi.ManagementResponse{
		StatusCode: status,
		Headers:    http.Header{"Content-Type": []string{"application/json"}},
		Body:       body,
	})
}

func htmlResponse(body string) ([]byte, error) {
	return okEnvelope(pluginapi.ManagementResponse{
		StatusCode: http.StatusOK,
		Headers:    http.Header{"Content-Type": []string{"text/html; charset=utf-8"}},
		Body:       []byte(body),
	})
}

func normalizeManagementPath(path string) (string, bool) {
	isResource := false
	if idx := strings.Index(path, "/v0/resource/plugins/"+pluginID); idx >= 0 {
		path = path[idx+len("/v0/resource/plugins/"+pluginID):]
		isResource = true
	} else if idx := strings.Index(path, "/v0/management/plugins/"+pluginID); idx >= 0 {
		path = path[idx+len("/v0/management/plugins/"+pluginID):]
	} else if strings.HasPrefix(path, managementBasePath) {
		path = strings.TrimPrefix(path, managementBasePath)
	}
	if path == "" {
		path = "/"
	}
	return path, isResource
}

func handleManagement(raw []byte) ([]byte, error) {
	var req pluginapi.ManagementRequest
	if errUnmarshal := json.Unmarshal(raw, &req); errUnmarshal != nil {
		return nil, errUnmarshal
	}
	path, isResource := normalizeManagementPath(req.Path)

	if isResource {
		// Resource routes are not management-authenticated: serve only the
		// static page shell, never account data.
		if req.Method == http.MethodGet && (path == "/status" || path == "/") {
			return htmlResponse(statusPageHTML)
		}
		return jsonResponse(http.StatusNotFound, map[string]string{"error": "not found"})
	}

	switch {
	case req.Method == http.MethodGet && path == "/status":
		return jsonResponse(http.StatusOK, buildStatus())
	case req.Method == http.MethodGet && path == "/plans":
		return jsonResponse(http.StatusOK, planCatalogResponse{
			GeneratedAt: time.Now().Format(time.RFC3339),
			Plans:       planCatalog,
		})
	case req.Method == http.MethodPost && path == "/refresh":
		kickPoller()
		return jsonResponse(http.StatusAccepted, map[string]string{"status": "refresh scheduled"})
	default:
		return jsonResponse(http.StatusNotFound, map[string]string{"error": "not found"})
	}
}
