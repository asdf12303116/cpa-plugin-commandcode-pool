package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v7/sdk/pluginapi"
)

func resetPool(cfg settings, accounts []*account) *pool {
	globalPool = &pool{snaps: make(map[string]*snapshot)}
	p := currentPool()
	p.reconfigure(cfg, accounts, nil)
	return p
}

func testAccount(name, suffix string, providers ...string) *account {
	return &account{
		Name:      name,
		KeySuffix: suffix,
		Providers: providers,
		apiKey:    "cmd-key-" + suffix,
	}
}

func TestBuildWindowNullExceededFallsBackToUsedVsCap(t *testing.T) {
	now := time.Now()
	got := buildWindow(windowFiveHour, &limitWindow{Used: 14, Cap: 14, Exceeded: nil, ResetAt: now.Add(2 * time.Hour).UnixMilli()}, 0, now)
	if !got.Blocked {
		t.Fatal("used == cap must be treated as blocked even when exceeded is null")
	}
	if !got.BlockedEffective {
		t.Fatal("without prepaid credits the block is effective")
	}
	if got.UsedPercent == nil || *got.UsedPercent != 100 {
		t.Fatalf("used_percent = %v", got.UsedPercent)
	}
	if got.ResetsInSeconds == nil || *got.ResetsInSeconds < 7000 || *got.ResetsInSeconds > 7300 {
		t.Fatalf("resets_in_seconds = %v", got.ResetsInSeconds)
	}
	if got.ResetAt == "" {
		t.Fatal("reset_at must be rendered")
	}
}

func TestBuildWindowPrepaidCreditsBypass(t *testing.T) {
	now := time.Now()
	got := buildWindow(windowWeekly, &limitWindow{Used: 40, Cap: 35, Exceeded: boolPtr(true)}, 5, now)
	if !got.Blocked {
		t.Fatal("exceeded=true must mark the window blocked")
	}
	if got.BlockedEffective {
		t.Fatal("prepaid credits bypass window caps, so the block is not effective")
	}
}

func TestBuildWindowMissingData(t *testing.T) {
	got := buildWindow(windowWeekly, nil, 0, time.Now())
	if got.Used != nil || got.Blocked {
		t.Fatalf("missing window must stay empty: %+v", got)
	}
}

func almostEqual(got *float64, want, tolerance float64) bool {
	return got != nil && *got >= want-tolerance && *got <= want+tolerance
}

func TestBuildBalanceDerivesConsumptionFromPlanTotal(t *testing.T) {
	var cr creditsResponse
	if errUnmarshal := json.Unmarshal([]byte(liveCreditsPayload), &cr); errUnmarshal != nil {
		t.Fatal(errUnmarshal)
	}
	sub := &subscriptionData{PlanID: "individual-goat", Status: "active"}
	got := buildBalanceStatus(&cr, sub)

	if !almostEqual(got.MonthlyIncluded, 70, 0.000001) {
		t.Fatalf("monthly_included = %v, want 70", got.MonthlyIncluded)
	}
	// Responses are rounded to 6 decimals for display.
	if !almostEqual(got.MonthlyRemaining, 49.489632881, 0.000001) {
		t.Fatalf("monthly_remaining = %v", got.MonthlyRemaining)
	}
	// 70 - 49.489632881 = 20.510367119
	if !almostEqual(got.MonthlyConsumed, 20.510367119, 0.000002) {
		t.Fatalf("monthly_consumed = %v", got.MonthlyConsumed)
	}
	if !almostEqual(got.MonthlyConsumedPercent, 29.3, 0.1) {
		t.Fatalf("monthly_consumed_percent = %v", got.MonthlyConsumedPercent)
	}
	if !almostEqual(got.TotalRemaining, 49.489632881, 0.000001) {
		t.Fatalf("total_remaining = %v", got.TotalRemaining)
	}
}

func TestBuildBalanceSkipsDerivationForUnknownOrInactivePlan(t *testing.T) {
	var cr creditsResponse
	if errUnmarshal := json.Unmarshal([]byte(liveCreditsPayload), &cr); errUnmarshal != nil {
		t.Fatal(errUnmarshal)
	}
	for _, sub := range []*subscriptionData{
		nil,
		{PlanID: "individual-goat", Status: "canceled"},
		{PlanID: "mystery-plan", Status: "active"},
		{PlanID: "individual-provider", Status: "active"},
	} {
		got := buildBalanceStatus(&cr, sub)
		if got.MonthlyIncluded != nil || got.MonthlyConsumedPercent != nil {
			t.Errorf("sub %+v must not derive an included allowance: %+v", sub, got)
		}
	}
}

func TestBuildPlanStatusPeriodMath(t *testing.T) {
	start := time.Date(2026, 8, 26, 8, 59, 51, 0, time.UTC)
	end := start.AddDate(0, 1, 0)
	now := start.Add(10 * 24 * time.Hour)
	sub := &subscriptionData{
		ID:                 "sub_1U8cgyDSZgxV3MJKfPSSpm9f",
		Status:             "active",
		PlanID:             "individual-goat",
		CurrentPeriodStart: start.Format(time.RFC3339),
		CurrentPeriodEnd:   end.Format(time.RFC3339),
		CancelAtPeriodEnd:  true,
	}
	got := buildPlanStatus(sub, now, 2)
	if got.Name != "GOAT" || !got.Known {
		t.Fatalf("plan resolution = %+v", got)
	}
	if got.DaysRemaining == nil || *got.DaysRemaining != 21 {
		t.Fatalf("days_remaining = %v", got.DaysRemaining)
	}
	if got.PeriodElapsedPercent == nil || *got.PeriodElapsedPercent < 32 || *got.PeriodElapsedPercent > 33 {
		t.Fatalf("period_elapsed_percent = %v", got.PeriodElapsedPercent)
	}
	if got.SharedKeys != 2 {
		t.Fatalf("shared_keys = %d", got.SharedKeys)
	}
}

func TestMaskEmailAndHumanDuration(t *testing.T) {
	if got := maskEmail("a675750333@gmail.com"); got != "a6***3@gmail.com" {
		t.Errorf("maskEmail = %q", got)
	}
	if got := maskEmail("ab@x.com"); got != "***@x.com" {
		t.Errorf("maskEmail short local = %q", got)
	}
	if got := maskEmail("not-an-email"); got != "***" {
		t.Errorf("maskEmail malformed = %q", got)
	}
	if got := maskEmail(""); got != "" {
		t.Errorf("maskEmail empty = %q", got)
	}
	cases := map[int64]string{
		0:     "now",
		-10:   "now",
		45:    "45s",
		90:    "1m",
		3661:  "1h 1m",
		90000: "1d 1h",
	}
	for seconds, want := range cases {
		if got := humanDuration(seconds); got != want {
			t.Errorf("humanDuration(%d) = %q, want %q", seconds, got, want)
		}
	}
}

func TestBuildStatusShapeAndSecretRedaction(t *testing.T) {
	cfg := decodeSettings(nil)
	acct := testAccount("cc-1", "AAAAAA", "openai-compatibility:commandcode", "claude-api-key")
	p := resetPool(cfg, []*account{acct})

	var cr creditsResponse
	if errUnmarshal := json.Unmarshal([]byte(liveCreditsPayload), &cr); errUnmarshal != nil {
		t.Fatal(errUnmarshal)
	}
	now := time.Now()
	p.mu.Lock()
	snap := p.snapshotFor(acct)
	snap.AttemptedAt = now
	snap.RefreshedAt = now
	snap.Whoami = &whoamiResponse{
		Success: true,
		User:    whoamiUser{ID: "09dd0bf8-97ab-4edc-8b32-dfb4824097af", Name: "Chen", Email: "a675750333@gmail.com", UserName: "asdf12303116"},
	}
	snap.Subscription = &subscriptionData{
		ID:                 "sub_1U8cgyDSZgxV3MJKfPSSpm9f",
		Status:             "active",
		PlanID:             "individual-goat",
		CurrentPeriodStart: now.Add(-10 * 24 * time.Hour).Format(time.RFC3339),
		CurrentPeriodEnd:   now.Add(20 * 24 * time.Hour).Format(time.RFC3339),
	}
	snap.Credits = &cr
	snap.Summary = &usageSummary{TotalCount: 6104, CompletedCount: 6104, SuccessRate: 100, TotalCost: 22.05, TotalTokens: 817460082, PeriodBasis: "billing-period"}
	snap.SummarySince = snap.Subscription.CurrentPeriodStart
	p.mu.Unlock()

	status := buildStatus()
	if len(status.Accounts) != 1 {
		t.Fatalf("accounts = %d", len(status.Accounts))
	}
	out := status.Accounts[0]
	if out.Name != "cc-1" || out.KeySuffix != "AAAAAA" || out.Stale || out.Disabled {
		t.Fatalf("account header = %+v", out)
	}
	if out.Plan.Name != "GOAT" || out.Plan.Status != "active" {
		t.Fatalf("plan = %+v", out.Plan)
	}
	if out.Identity.Email != "a6***3@gmail.com" || out.Identity.UserName != "asdf12303116" {
		t.Fatalf("identity = %+v", out.Identity)
	}
	if out.Balance.MonthlyIncluded == nil || *out.Balance.MonthlyIncluded != 70 {
		t.Fatalf("balance = %+v", out.Balance)
	}
	five := out.Windows[windowFiveHour]
	if five.UsedPercent == nil || *five.UsedPercent < 0.8 || *five.UsedPercent > 0.9 {
		t.Fatalf("5h used_percent = %v", five.UsedPercent)
	}
	if out.PeriodSummary == nil || out.PeriodSummary.Requests != 6104 || out.PeriodSummary.TokensTotal != 817460082 {
		t.Fatalf("period summary = %+v", out.PeriodSummary)
	}

	raw, errMarshal := json.Marshal(status)
	if errMarshal != nil {
		t.Fatal(errMarshal)
	}
	for _, secret := range []string{"cmd-key-AAAAAA", acct.apiKey} {
		if strings.Contains(string(raw), secret) {
			t.Fatalf("status output leaked the API key %q", secret)
		}
	}
	if !strings.Contains(string(raw), `"api_base_url":"`+defaultAPIBaseURL+`"`) {
		t.Fatalf("status output missing api base url: %s", raw)
	}
}

func TestBuildStatusGroupsSharedSubscriptions(t *testing.T) {
	cfg := decodeSettings(nil)
	first := testAccount("cc-1", "AAAAAA")
	second := testAccount("cc-2", "BBBBBB")
	p := resetPool(cfg, []*account{first, second})

	now := time.Now()
	p.mu.Lock()
	for _, acct := range []*account{first, second} {
		snap := p.snapshotFor(acct)
		snap.RefreshedAt = now
		snap.AttemptedAt = now
		snap.Subscription = &subscriptionData{ID: "sub_shared", Status: "active", PlanID: "individual-max"}
	}
	p.mu.Unlock()

	status := buildStatus()
	if len(status.SharedSubscriptions) != 1 {
		t.Fatalf("shared_subscriptions = %+v", status.SharedSubscriptions)
	}
	group := status.SharedSubscriptions[0]
	if group.SubscriptionID != "sub_shared" || group.PlanID != "individual-max" || len(group.Accounts) != 2 {
		t.Fatalf("group = %+v", group)
	}
	for _, acct := range status.Accounts {
		if acct.Plan.SharedKeys != 2 {
			t.Errorf("account %s shared_keys = %d", acct.Name, acct.Plan.SharedKeys)
		}
	}
}

func TestBuildStatusMarksStaleReadings(t *testing.T) {
	cfg := decodeSettings([]byte("usage-stale-after: 1m\n"))
	acct := testAccount("cc-1", "AAAAAA")
	p := resetPool(cfg, []*account{acct})

	p.mu.Lock()
	snap := p.snapshotFor(acct)
	snap.RefreshedAt = time.Now().Add(-2 * time.Minute)
	snap.AttemptedAt = time.Now()
	p.mu.Unlock()

	out := buildStatus().Accounts[0]
	if !out.Stale {
		t.Fatal("reading older than usage-stale-after must be stale")
	}
	if out.DataAgeSeconds == nil || *out.DataAgeSeconds < 100 {
		t.Fatalf("data_age_seconds = %v", out.DataAgeSeconds)
	}
}

func managementCall(t *testing.T, method, path string, body []byte) (pluginapi.ManagementResponse, int) {
	t.Helper()
	raw, errMarshal := json.Marshal(pluginapi.ManagementRequest{Method: method, Path: path, Body: body})
	if errMarshal != nil {
		t.Fatal(errMarshal)
	}
	out, errHandle := handleManagement(raw)
	if errHandle != nil {
		t.Fatal(errHandle)
	}
	var env envelope
	if errUnmarshal := json.Unmarshal(out, &env); errUnmarshal != nil {
		t.Fatal(errUnmarshal)
	}
	if !env.OK {
		t.Fatalf("management envelope not ok: %+v", env.Error)
	}
	var resp pluginapi.ManagementResponse
	if errUnmarshal := json.Unmarshal(env.Result, &resp); errUnmarshal != nil {
		t.Fatal(errUnmarshal)
	}
	return resp, resp.StatusCode
}

func TestHandleManagementRoutes(t *testing.T) {
	cfg := decodeSettings(nil)
	acct := testAccount("cc-1", "AAAAAA")
	p := resetPool(cfg, []*account{acct})
	now := time.Now()
	p.mu.Lock()
	snap := p.snapshotFor(acct)
	snap.RefreshedAt = now
	snap.AttemptedAt = now
	snap.Credits = &creditsResponse{}
	p.mu.Unlock()

	resp, status := managementCall(t, http.MethodGet, "/v0/management/plugins/commandcode-pool/status", nil)
	if status != http.StatusOK {
		t.Fatalf("status route = %d", status)
	}
	var statusBody statusResponse
	if errUnmarshal := json.Unmarshal(resp.Body, &statusBody); errUnmarshal != nil {
		t.Fatal(errUnmarshal)
	}
	if len(statusBody.Accounts) != 1 || statusBody.Accounts[0].Name != "cc-1" {
		t.Fatalf("status body = %+v", statusBody)
	}

	resp, status = managementCall(t, http.MethodGet, "/v0/management/plugins/commandcode-pool/plans", nil)
	if status != http.StatusOK {
		t.Fatalf("plans route = %d", status)
	}
	var plans planCatalogResponse
	if errUnmarshal := json.Unmarshal(resp.Body, &plans); errUnmarshal != nil {
		t.Fatal(errUnmarshal)
	}
	if len(plans.Plans) != len(planCatalog) {
		t.Fatalf("plans = %d, want %d", len(plans.Plans), len(planCatalog))
	}

	if _, status = managementCall(t, http.MethodPost, "/v0/management/plugins/commandcode-pool/refresh", []byte("{}")); status != http.StatusAccepted {
		t.Fatalf("refresh route = %d", status)
	}

	if _, status = managementCall(t, http.MethodGet, "/v0/management/plugins/commandcode-pool/nope", nil); status != http.StatusNotFound {
		t.Fatalf("unknown route = %d", status)
	}
}

func TestHandleManagementResourcePage(t *testing.T) {
	resp, status := managementCall(t, http.MethodGet, "/v0/resource/plugins/commandcode-pool/status", nil)
	if status != http.StatusOK {
		t.Fatalf("resource page = %d", status)
	}
	if contentType := resp.Headers.Get("Content-Type"); !strings.HasPrefix(contentType, "text/html") {
		t.Fatalf("content type = %q", contentType)
	}
	page := string(resp.Body)
	if !strings.Contains(page, "CommandCode Pool") || !strings.Contains(page, "/v0/management/plugins/commandcode-pool") {
		t.Fatal("resource page shell is missing expected content")
	}
	if strings.Contains(page, "cmd-key-") {
		t.Fatal("resource page must not contain credential material")
	}
}

func TestHandleManagementRegisterDeclaresRoutes(t *testing.T) {
	raw, errRegister := handleManagementRegister()
	if errRegister != nil {
		t.Fatal(errRegister)
	}
	var env envelope
	if errUnmarshal := json.Unmarshal(raw, &env); errUnmarshal != nil {
		t.Fatal(errUnmarshal)
	}
	var resp pluginapi.ManagementRegistrationResponse
	if errUnmarshal := json.Unmarshal(env.Result, &resp); errUnmarshal != nil {
		t.Fatal(errUnmarshal)
	}
	wantRoutes := map[string]bool{
		"GET /plugins/commandcode-pool/status":   false,
		"GET /plugins/commandcode-pool/plans":    false,
		"POST /plugins/commandcode-pool/refresh": false,
	}
	for _, route := range resp.Routes {
		key := route.Method + " " + route.Path
		if _, ok := wantRoutes[key]; !ok {
			t.Errorf("unexpected route %q", key)
			continue
		}
		wantRoutes[key] = true
	}
	for key, seen := range wantRoutes {
		if !seen {
			t.Errorf("missing route %q", key)
		}
	}
	if len(resp.Resources) != 1 || resp.Resources[0].Path != "/status" {
		t.Fatalf("resources = %+v", resp.Resources)
	}
}

func TestNormalizeManagementPath(t *testing.T) {
	cases := []struct {
		path     string
		wantPath string
		wantRes  bool
	}{
		{"/v0/management/plugins/commandcode-pool/status", "/status", false},
		{"/v0/resource/plugins/commandcode-pool/status", "/status", true},
		{"/plugins/commandcode-pool/plans", "/plans", false},
		{"/v0/management/plugins/commandcode-pool", "/", false},
	}
	for _, tc := range cases {
		path, isResource := normalizeManagementPath(tc.path)
		if path != tc.wantPath || isResource != tc.wantRes {
			t.Errorf("normalizeManagementPath(%q) = %q, %v; want %q, %v", tc.path, path, isResource, tc.wantPath, tc.wantRes)
		}
	}
}

func TestPluginRegistrationIsManagementOnly(t *testing.T) {
	reg := pluginRegistration()
	raw, errMarshal := json.Marshal(reg.Capabilities)
	if errMarshal != nil {
		t.Fatal(errMarshal)
	}
	if string(raw) != `{"management_api":true}` {
		t.Fatalf("capabilities = %s; the plugin must not claim scheduler/usage capabilities", raw)
	}
	if reg.Metadata.Name != "CommandCode Pool" || reg.Metadata.Version != pluginVersion {
		t.Fatalf("metadata = %+v", reg.Metadata)
	}
	if len(reg.Metadata.ConfigFields) == 0 {
		t.Fatal("config fields must be advertised")
	}
}
