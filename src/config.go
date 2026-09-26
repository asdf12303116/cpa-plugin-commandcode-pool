package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	defaultCPAConfigPath = "config.yaml"
	defaultAPIBaseURL    = "https://api.commandcode.ai"
	defaultRefreshEvery  = 3 * time.Minute
	defaultStaleAfter    = 20 * time.Minute
	defaultWarnPercent   = 80
	defaultCriticalPct   = 95
	// CommandCode is fronted by Cloudflare, which rejects some non-browser
	// user agents (notably Go's default) with "error code 1010". A curl-like
	// user agent is accepted, so it is used unless overridden.
	defaultUserAgent = "curl/8.7.1"
)

var defaultBaseURLMatch = []string{"commandcode.ai"}

// accountOverride carries optional per-account settings from the plugin
// config, matched to a discovered API key by suffix.
type accountOverride struct {
	KeySuffix string `yaml:"key-suffix"`
	Name      string `yaml:"name"`
	Disabled  bool   `yaml:"disabled"`
}

type pluginConfig struct {
	CPAConfigPath   string            `yaml:"cpa-config-path"`
	APIBaseURL      string            `yaml:"api-base-url"`
	RefreshInterval string            `yaml:"usage-refresh-interval"`
	StaleAfter      string            `yaml:"usage-stale-after"`
	IncludeSummary  *bool             `yaml:"include-usage-summary"`
	PublicStatus    *bool             `yaml:"public-status"`
	WarnPercent     int               `yaml:"warn-percent"`
	CriticalPercent int               `yaml:"critical-percent"`
	UserAgent       string            `yaml:"user-agent"`
	BaseURLMatch    []string          `yaml:"base-url-match"`
	Accounts        []accountOverride `yaml:"accounts"`
}

type settings struct {
	CPAConfigPath   string
	APIBaseURL      string
	RefreshInterval time.Duration
	StaleAfter      time.Duration
	IncludeSummary  bool
	PublicStatus    bool
	WarnPercent     int
	CriticalPercent int
	UserAgent       string
	BaseURLMatch    []string
	Overrides       []accountOverride
}

func parseDurationOr(raw string, fallback time.Duration) time.Duration {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fallback
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d <= 0 {
		return fallback
	}
	return d
}

func normalizePercent(raw, fallback int) int {
	if raw <= 0 || raw > 100 {
		return fallback
	}
	return raw
}

func decodeSettings(configYAML []byte) settings {
	cfg := pluginConfig{}
	if len(configYAML) > 0 {
		_ = yaml.Unmarshal(configYAML, &cfg)
	}
	out := settings{
		CPAConfigPath:   strings.TrimSpace(cfg.CPAConfigPath),
		APIBaseURL:      strings.TrimSpace(cfg.APIBaseURL),
		RefreshInterval: parseDurationOr(cfg.RefreshInterval, defaultRefreshEvery),
		StaleAfter:      parseDurationOr(cfg.StaleAfter, defaultStaleAfter),
		IncludeSummary:  true,
		PublicStatus:    true,
		WarnPercent:     normalizePercent(cfg.WarnPercent, defaultWarnPercent),
		CriticalPercent: normalizePercent(cfg.CriticalPercent, defaultCriticalPct),
		UserAgent:       strings.TrimSpace(cfg.UserAgent),
		BaseURLMatch:    cfg.BaseURLMatch,
		Overrides:       cfg.Accounts,
	}
	if out.CPAConfigPath == "" {
		out.CPAConfigPath = defaultCPAConfigPath
	}
	if out.APIBaseURL == "" {
		out.APIBaseURL = defaultAPIBaseURL
	}
	if out.UserAgent == "" {
		out.UserAgent = defaultUserAgent
	}
	if cfg.IncludeSummary != nil {
		out.IncludeSummary = *cfg.IncludeSummary
	}
	if cfg.PublicStatus != nil {
		out.PublicStatus = *cfg.PublicStatus
	}
	if len(out.BaseURLMatch) == 0 {
		out.BaseURLMatch = append([]string(nil), defaultBaseURLMatch...)
	}
	for i := range out.BaseURLMatch {
		out.BaseURLMatch[i] = strings.ToLower(strings.TrimSpace(out.BaseURLMatch[i]))
	}
	if out.CriticalPercent < out.WarnPercent {
		out.CriticalPercent = out.WarnPercent
	}
	return out
}

// cpaConfig mirrors just the parts of the CPA config file needed to discover
// CommandCode credentials.
type cpaConfig struct {
	OpenAICompatibility []struct {
		Name          string `yaml:"name"`
		BaseURL       string `yaml:"base-url"`
		Disabled      bool   `yaml:"disabled"`
		APIKeyEntries []struct {
			APIKey string `yaml:"api-key"`
		} `yaml:"api-key-entries"`
	} `yaml:"openai-compatibility"`
	ClaudeKey []struct {
		APIKey  string `yaml:"api-key"`
		BaseURL string `yaml:"base-url"`
	} `yaml:"claude-api-key"`
}

// account is one logical CommandCode credential.
type account struct {
	// KeySuffix is the trailing part of the API key, used for display and
	// override matching. It is safe to publish.
	KeySuffix string
	// Name is the dashboard display name (defaults to cc-N).
	Name string
	// Providers lists where the key was discovered in the CPA config.
	Providers []string
	// Disabled skips polling for this account.
	Disabled bool

	// apiKey is kept in memory only to authenticate /alpha/* requests; it must
	// never be serialized into status output or logs.
	apiKey string
}

// identity is the stable state key of an account. It is a hash of the API key
// so runtime state survives reconfigure without persisting the secret.
func (a *account) identity() string {
	return keyHash(a.apiKey)
}

func keySuffix(key string) string {
	if len(key) <= 6 {
		return key
	}
	return key[len(key)-6:]
}

func matchesCommandCodeBaseURL(baseURL string, patterns []string) bool {
	baseURL = strings.ToLower(strings.TrimSpace(baseURL))
	if baseURL == "" {
		return false
	}
	for _, pattern := range patterns {
		if pattern != "" && strings.Contains(baseURL, pattern) {
			return true
		}
	}
	return false
}

// discoverAccounts reads the CPA config file and collects every CommandCode API
// key from openai-compatibility and claude-api-key entries whose base URL
// points at CommandCode. The same key appearing under both protocols yields one
// logical account.
func discoverAccounts(cfg settings) ([]*account, error) {
	raw, errRead := os.ReadFile(cfg.CPAConfigPath)
	if errRead != nil {
		return nil, fmt.Errorf("read CPA config %s: %w", cfg.CPAConfigPath, errRead)
	}
	var cpa cpaConfig
	if errUnmarshal := yaml.Unmarshal(raw, &cpa); errUnmarshal != nil {
		return nil, fmt.Errorf("parse CPA config: %w", errUnmarshal)
	}

	byKey := make(map[string]*account)
	var ordered []*account
	ensure := func(key string) *account {
		if acct, ok := byKey[key]; ok {
			return acct
		}
		acct := &account{KeySuffix: keySuffix(key), apiKey: key}
		byKey[key] = acct
		ordered = append(ordered, acct)
		return acct
	}

	for i := range cpa.OpenAICompatibility {
		compat := cpa.OpenAICompatibility[i]
		if compat.Disabled || !matchesCommandCodeBaseURL(compat.BaseURL, cfg.BaseURLMatch) {
			continue
		}
		providerName := strings.TrimSpace(compat.Name)
		if providerName == "" {
			providerName = "openai-compatibility"
		}
		label := "openai-compatibility:" + strings.ToLower(providerName)
		for j := range compat.APIKeyEntries {
			key := strings.TrimSpace(compat.APIKeyEntries[j].APIKey)
			if key == "" {
				continue
			}
			acct := ensure(key)
			acct.Providers = appendUnique(acct.Providers, label)
		}
	}

	for i := range cpa.ClaudeKey {
		entry := cpa.ClaudeKey[i]
		if !matchesCommandCodeBaseURL(entry.BaseURL, cfg.BaseURLMatch) {
			continue
		}
		key := strings.TrimSpace(entry.APIKey)
		if key == "" {
			continue
		}
		acct := ensure(key)
		acct.Providers = appendUnique(acct.Providers, "claude-api-key")
	}

	for i, acct := range ordered {
		acct.Name = fmt.Sprintf("cc-%d", i+1)
		for _, override := range cfg.Overrides {
			suffix := strings.TrimSpace(override.KeySuffix)
			if suffix == "" || !strings.HasSuffix(acct.apiKey, suffix) {
				continue
			}
			if strings.TrimSpace(override.Name) != "" {
				acct.Name = strings.TrimSpace(override.Name)
			}
			if override.Disabled {
				acct.Disabled = true
			}
		}
	}
	return ordered, nil
}

func appendUnique(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}
