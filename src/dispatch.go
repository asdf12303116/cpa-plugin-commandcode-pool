package main

import (
	"encoding/json"

	"github.com/router-for-me/CLIProxyAPI/v7/sdk/pluginabi"
	"github.com/router-for-me/CLIProxyAPI/v7/sdk/pluginapi"
)

type lifecycleRequest struct {
	ConfigYAML []byte `json:"config_yaml"`
}

type registration struct {
	SchemaVersion uint32                 `json:"schema_version"`
	Metadata      pluginapi.Metadata     `json:"metadata"`
	Capabilities  registrationCapability `json:"capabilities"`
}

// registrationCapability lists the host integration points this plugin uses.
// commandcode-pool is observational: it only registers Management API routes.
type registrationCapability struct {
	ManagementAPI bool `json:"management_api"`
}

func handleMethod(method string, request []byte) ([]byte, error) {
	switch method {
	case pluginabi.MethodPluginRegister, pluginabi.MethodPluginReconfigure:
		if errConfigure := configure(request); errConfigure != nil {
			return nil, errConfigure
		}
		startPoller()
		return okEnvelope(pluginRegistration())
	case pluginabi.MethodManagementRegister:
		return handleManagementRegister()
	case pluginabi.MethodManagementHandle:
		return handleManagement(request)
	default:
		return errorEnvelope("unknown_method", "unknown method: "+method), nil
	}
}

func configure(raw []byte) error {
	var req lifecycleRequest
	if len(raw) > 0 {
		if errUnmarshal := json.Unmarshal(raw, &req); errUnmarshal != nil {
			return errUnmarshal
		}
	}
	cfg := decodeSettings(req.ConfigYAML)
	accounts, errDiscover := discoverAccounts(cfg)
	currentPool().reconfigure(cfg, accounts, errDiscover)

	fields := map[string]any{
		"accounts":     len(accounts),
		"api_base_url": cfg.APIBaseURL,
	}
	if errDiscover != nil {
		fields["config_error"] = errDiscover.Error()
	}
	hostLog("info", "configured", fields)
	return nil
}

func pluginRegistration() registration {
	return registration{
		SchemaVersion: pluginabi.SchemaVersion,
		Metadata: pluginapi.Metadata{
			Name:             "CommandCode Pool",
			Version:          pluginVersion,
			Author:           "asdf12303116",
			GitHubRepository: "https://github.com/asdf12303116/cpa-plugin-commandcode-pool",
			ConfigFields: []pluginapi.ConfigField{
				{
					Name:        "cpa-config-path",
					Type:        pluginapi.ConfigFieldTypeString,
					Description: "Path of the CPA config file used to discover CommandCode API keys (default config.yaml).",
				},
				{
					Name:        "api-base-url",
					Type:        pluginapi.ConfigFieldTypeString,
					Description: "CommandCode API origin polled for usage data (default https://api.commandcode.ai).",
				},
				{
					Name:        "base-url-match",
					Type:        pluginapi.ConfigFieldTypeArray,
					Description: "Substrings that identify a CommandCode credential base URL during discovery (default [commandcode.ai]).",
				},
				{
					Name:        "usage-refresh-interval",
					Type:        pluginapi.ConfigFieldTypeString,
					Description: "How often every key is polled through the /alpha/* usage API (default 3m).",
				},
				{
					Name:        "usage-stale-after",
					Type:        pluginapi.ConfigFieldTypeString,
					Description: "Readings older than this are flagged stale (default 20m).",
				},
				{
					Name:        "include-usage-summary",
					Type:        pluginapi.ConfigFieldTypeBoolean,
					Description: "Call /alpha/usage/summary (billing-period aggregates) on every cycle (default true).",
				},
				{
					Name:        "public-status",
					Type:        pluginapi.ConfigFieldTypeBoolean,
					Description: "Serve the read-only dashboard data on the unauthenticated resource route so the page needs no management key (default true; disable to require the key).",
				},
				{
					Name:        "warn-percent",
					Type:        pluginapi.ConfigFieldTypeInteger,
					Description: "Dashboard warning threshold in percent of a window cap (default 80).",
				},
				{
					Name:        "critical-percent",
					Type:        pluginapi.ConfigFieldTypeInteger,
					Description: "Dashboard critical threshold in percent of a window cap (default 95).",
				},
				{
					Name:        "user-agent",
					Type:        pluginapi.ConfigFieldTypeString,
					Description: "User agent used for /alpha/* requests; the default avoids Cloudflare's 1010 block.",
				},
				{
					Name:        "accounts",
					Type:        pluginapi.ConfigFieldTypeArray,
					Description: "Per-account overrides matched by key-suffix: name, disabled.",
				},
			},
		},
		Capabilities: registrationCapability{
			ManagementAPI: true,
		},
	}
}
