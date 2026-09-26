package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const discoveryFixture = `
openai-compatibility:
  - name: commandcode
    base-url: https://api.commandcode.ai/provider/v1
    api-key-entries:
      - api-key: cmd-key-AAAAAA
      - api-key: cmd-key-BBBBBB
  - name: unrelated
    base-url: https://api.example.com/v1
    api-key-entries:
      - api-key: other-key-CCCCCC
claude-api-key:
  - api-key: cmd-key-AAAAAA
    base-url: https://api.commandcode.ai
  - api-key: cmd-key-DDDDDD
    base-url: https://api.commandcode.ai/
  - api-key: anthropic-key-EEEEEE
    base-url: https://api.anthropic.com
`

func writeConfigFixture(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if errWrite := os.WriteFile(path, []byte(body), 0o600); errWrite != nil {
		t.Fatal(errWrite)
	}
	return path
}

func TestDiscoverAccountsByBaseURL(t *testing.T) {
	cfg := decodeSettings(nil)
	cfg.CPAConfigPath = writeConfigFixture(t, discoveryFixture)

	accounts, errDiscover := discoverAccounts(cfg)
	if errDiscover != nil {
		t.Fatal(errDiscover)
	}
	if len(accounts) != 3 {
		t.Fatalf("expected 3 accounts, got %d (%+v)", len(accounts), accounts)
	}
	if accounts[0].KeySuffix != "AAAAAA" || accounts[1].KeySuffix != "BBBBBB" || accounts[2].KeySuffix != "DDDDDD" {
		t.Fatalf("unexpected key suffixes: %q %q %q", accounts[0].KeySuffix, accounts[1].KeySuffix, accounts[2].KeySuffix)
	}
	// The same key configured for both protocols must collapse into one account.
	if len(accounts[0].Providers) != 2 {
		t.Fatalf("AAAAAA providers = %v, want both protocols", accounts[0].Providers)
	}
	if got := accounts[0].Providers[0]; got != "openai-compatibility:commandcode" {
		t.Errorf("providers[0] = %q", got)
	}
	if got := accounts[0].Providers[1]; got != "claude-api-key" {
		t.Errorf("providers[1] = %q", got)
	}
	if accounts[0].Name != "cc-1" || accounts[2].Name != "cc-3" {
		t.Errorf("default names wrong: %q %q", accounts[0].Name, accounts[2].Name)
	}
}

func TestDiscoverAccountsOverrides(t *testing.T) {
	cfg := decodeSettings(nil)
	cfg.CPAConfigPath = writeConfigFixture(t, discoveryFixture)
	cfg.Overrides = []accountOverride{
		{KeySuffix: "BBBBBB", Name: "beta"},
		{KeySuffix: "DDDDDD", Disabled: true},
	}

	accounts, errDiscover := discoverAccounts(cfg)
	if errDiscover != nil {
		t.Fatal(errDiscover)
	}
	if accounts[1].Name != "beta" {
		t.Errorf("override name not applied: %+v", accounts[1])
	}
	if !accounts[2].Disabled {
		t.Errorf("override disabled not applied: %+v", accounts[2])
	}
	if accounts[0].Disabled {
		t.Errorf("unrelated account must stay enabled: %+v", accounts[0])
	}
}

func TestDiscoverAccountsMissingConfig(t *testing.T) {
	cfg := decodeSettings(nil)
	cfg.CPAConfigPath = filepath.Join(t.TempDir(), "does-not-exist.yaml")
	if _, errDiscover := discoverAccounts(cfg); errDiscover == nil {
		t.Fatal("expected an error for a missing config file")
	}
}

func TestMatchesCommandCodeBaseURL(t *testing.T) {
	patterns := defaultBaseURLMatch
	cases := map[string]bool{
		"https://api.commandcode.ai/provider/v1": true,
		"https://api.commandcode.ai":             true,
		"https://api.commandcode.ai/":            true,
		"HTTPS://API.COMMANDCODE.AI/x":           true,
		"https://api.anthropic.com":              false,
		"":                                       false,
	}
	for baseURL, want := range cases {
		if got := matchesCommandCodeBaseURL(baseURL, patterns); got != want {
			t.Errorf("matchesCommandCodeBaseURL(%q) = %v, want %v", baseURL, got, want)
		}
	}
}

func TestDecodeSettingsDefaults(t *testing.T) {
	cfg := decodeSettings(nil)
	if cfg.CPAConfigPath != defaultCPAConfigPath {
		t.Errorf("cpa-config-path = %q", cfg.CPAConfigPath)
	}
	if cfg.APIBaseURL != defaultAPIBaseURL {
		t.Errorf("api-base-url = %q", cfg.APIBaseURL)
	}
	if cfg.RefreshInterval != defaultRefreshEvery || cfg.StaleAfter != defaultStaleAfter {
		t.Errorf("durations = %v / %v", cfg.RefreshInterval, cfg.StaleAfter)
	}
	if !cfg.IncludeSummary {
		t.Error("include-usage-summary must default to true")
	}
	if cfg.WarnPercent != defaultWarnPercent || cfg.CriticalPercent != defaultCriticalPct {
		t.Errorf("thresholds = %d / %d", cfg.WarnPercent, cfg.CriticalPercent)
	}
	if cfg.UserAgent != defaultUserAgent {
		t.Errorf("user-agent = %q", cfg.UserAgent)
	}
	if len(cfg.BaseURLMatch) != 1 || cfg.BaseURLMatch[0] != "commandcode.ai" {
		t.Errorf("base-url-match = %v", cfg.BaseURLMatch)
	}
}

func TestDecodeSettingsOverridesAndClamping(t *testing.T) {
	raw := []byte(`
cpa-config-path: /etc/cpa/config.yaml
api-base-url: https://staging-api.commandcode.ai/
usage-refresh-interval: 90s
usage-stale-after: 5m
include-usage-summary: false
warn-percent: 90
critical-percent: 50
user-agent: my-agent/1.0
base-url-match:
  - commandcode.ai
  - staging-api
`)
	cfg := decodeSettings(raw)
	if cfg.CPAConfigPath != "/etc/cpa/config.yaml" || cfg.APIBaseURL != "https://staging-api.commandcode.ai/" {
		t.Errorf("paths = %q / %q", cfg.CPAConfigPath, cfg.APIBaseURL)
	}
	if cfg.RefreshInterval.String() != "1m30s" || cfg.StaleAfter.String() != "5m0s" {
		t.Errorf("durations = %v / %v", cfg.RefreshInterval, cfg.StaleAfter)
	}
	if cfg.IncludeSummary {
		t.Error("include-usage-summary override not applied")
	}
	if cfg.UserAgent != "my-agent/1.0" {
		t.Errorf("user-agent = %q", cfg.UserAgent)
	}
	if len(cfg.BaseURLMatch) != 2 {
		t.Errorf("base-url-match = %v", cfg.BaseURLMatch)
	}
	// critical < warn must be raised to warn so the dashboard stays sane.
	if cfg.WarnPercent != 90 || cfg.CriticalPercent != 90 {
		t.Errorf("threshold clamping: %d / %d", cfg.WarnPercent, cfg.CriticalPercent)
	}
}

func TestDecodeSettingsInvalidValuesFallBack(t *testing.T) {
	cfg := decodeSettings([]byte("usage-refresh-interval: not-a-duration\nwarn-percent: 0\ncritical-percent: 900\n"))
	if cfg.RefreshInterval != defaultRefreshEvery {
		t.Errorf("invalid duration must fall back, got %v", cfg.RefreshInterval)
	}
	if cfg.WarnPercent != defaultWarnPercent || cfg.CriticalPercent != defaultCriticalPct {
		t.Errorf("invalid percents must fall back, got %d / %d", cfg.WarnPercent, cfg.CriticalPercent)
	}
}

func TestKeySuffix(t *testing.T) {
	if got := keySuffix("short"); got != "short" {
		t.Errorf("keySuffix(short) = %q", got)
	}
	if got := keySuffix("cmd-key-AAAAAA"); got != "AAAAAA" {
		t.Errorf("keySuffix = %q", got)
	}
	if strings.Contains(keyHash("cmd-key-AAAAAA"), "cmd-key") {
		t.Error("keyHash must not embed the raw key")
	}
}
