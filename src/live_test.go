package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v7/sdk/pluginapi"
)

// netHTTPDoer executes plugin HTTP requests with net/http so the exact plugin
// code path (URL building, headers, error parsing, decoding, derivation) can be
// driven without a CPA host. Proxy environment variables are honoured.
func netHTTPDoer() httpDoer {
	client := &http.Client{Timeout: 30 * time.Second}
	return func(req pluginapi.HTTPRequest) (pluginapi.HTTPResponse, error) {
		var body io.Reader
		if len(req.Body) > 0 {
			body = bytes.NewReader(req.Body)
		}
		httpReq, errNewRequest := http.NewRequest(req.Method, req.URL, body)
		if errNewRequest != nil {
			return pluginapi.HTTPResponse{}, errNewRequest
		}
		for name, values := range req.Headers {
			for _, value := range values {
				httpReq.Header.Add(name, value)
			}
		}
		resp, errDo := client.Do(httpReq)
		if errDo != nil {
			return pluginapi.HTTPResponse{}, errDo
		}
		defer func() { _ = resp.Body.Close() }()
		raw, errRead := io.ReadAll(resp.Body)
		if errRead != nil {
			return pluginapi.HTTPResponse{}, errRead
		}
		return pluginapi.HTTPResponse{StatusCode: resp.StatusCode, Headers: resp.Header, Body: raw}, nil
	}
}

// TestLiveCommandCodeAPI performs a real end-to-end poll against the
// CommandCode /alpha/* API. It is skipped unless COMMANDCODE_LIVE_KEY is set,
// so CI and normal test runs stay offline:
//
//	COMMANDCODE_LIVE_KEY=user_... go test ./src -run TestLiveCommandCodeAPI -v
func TestLiveCommandCodeAPI(t *testing.T) {
	key := strings.TrimSpace(os.Getenv("COMMANDCODE_LIVE_KEY"))
	if key == "" {
		t.Skip("set COMMANDCODE_LIVE_KEY to run the live CommandCode API check")
	}

	cfg := decodeSettings(nil)
	acct := &account{Name: "live", KeySuffix: keySuffix(key), Providers: []string{"live-test"}, apiKey: key}
	p := resetPool(cfg, []*account{acct})
	refreshAccount(p, cfg, newAPIClientWithDoer(cfg, netHTTPDoer()), acct)

	status := buildStatus()
	if len(status.Accounts) != 1 {
		t.Fatalf("accounts = %d, want 1", len(status.Accounts))
	}
	live := status.Accounts[0]
	raw, errMarshal := json.MarshalIndent(status, "", "  ")
	if errMarshal != nil {
		t.Fatal(errMarshal)
	}
	t.Logf("live status:\n%s", raw)

	if len(live.Errors) > 0 {
		t.Fatalf("endpoint errors: %v", live.Errors)
	}
	if !live.Plan.Known || live.Plan.ID == "" {
		t.Errorf("plan not resolved: %+v", live.Plan)
	}
	if live.Balance.MonthlyRemaining == nil {
		t.Errorf("credits not read: %+v", live.Balance)
	}
	if live.Windows[windowFiveHour].Cap == nil || live.Windows[windowWeekly].Cap == nil {
		t.Errorf("windows not read: %+v", live.Windows)
	}
	if live.PeriodSummary == nil {
		t.Errorf("usage summary not read")
	}
	if live.Stale {
		t.Errorf("fresh reading reported as stale")
	}
	if strings.Contains(string(raw), key) {
		t.Fatal("status output leaked the API key")
	}
}
