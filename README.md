# CommandCode Pool for CLIProxyAPI

[中文说明](README.zh-CN.md)

`commandcode-pool` is a read-only [CLIProxyAPI](https://github.com/router-for-me/CLIProxyAPI)
plugin that reports the state of every [CommandCode](https://commandcode.ai)
subscription configured in CPA: plan status and billing period, remaining
credits, and the rolling 5-hour / weekly usage windows with their reset times.

It is **observational by design**. It never selects credentials, never blocks an
account, never writes to `config.yaml`, and never persists an API key. It exists
so you can answer "which CommandCode key still has headroom, and when does it
reset?" from the CPA dashboard instead of the CommandCode Studio.

## Features

- **Credential auto-discovery:** every key whose `base-url` points at
  `commandcode.ai` is found automatically — no per-account configuration.
- **Plan status:** `planId` resolved to a human name, subscription status,
  quantity, cancel-at-period-end, and the current billing period with elapsed
  percentage and days remaining.
- **Credit balance:** remaining monthly allowance, included allowance,
  derived consumption and consumption percentage, prepaid and free credits,
  low-credit threshold state.
- **Reset-window usage:** 5-hour and weekly windows with `used`, `cap`,
  headroom, percentage, reset timestamp, and a live countdown.
- **Billing-period spend:** credit cost from `/alpha/usage/summary`, with
  request/token/success-rate figures kept as secondary diagnostics because
  CommandCode limits are credit-value based, not request quotas.
- **Shared-pool detection:** multiple API keys issued under one subscription are
  grouped, because they share a single credit pool.
- **Management API + dashboard page:** inspect everything from the CPA Manager
  Plus plugin page or from JSON.
- **Fail-safe polling:** a failed endpoint keeps the previous reading and is
  reported as an error; stale readings are flagged instead of being presented as
  current.

## Requirements

- CLIProxyAPI v7.2.67 or a compatible release with the C ABI plugin system
- Linux amd64 or arm64 with glibc 2.36 or newer
- Go 1.26 with CGO enabled and a local C compiler toolchain (build only)
- A CommandCode plan with Provider API access (every plan except Go; Go keys
  receive `403 upgrade_required`)

## Repository layout

- `src/` — plugin implementation and tests.
- `.github/scripts/` — release-only packaging tooling.
- `registry.json` — custom CPA plugin-store registry.
- `config.example.yaml` — relevant CPA configuration sections.

The route shapes, field semantics, error envelopes, and plan table this plugin
is built against come from a locally captured spec of the CommandCode usage API.
That capture contains account identifiers, so it is deliberately kept out of
this repository and listed in `.gitignore`.

## Installation

### CPA Manager Plus plugin store

Add this custom registry to CPA's `config.yaml` and make sure the plugin
directory is persisted across container replacements:

```yaml
plugins:
  enabled: true
  dir: plugins
  store-sources:
    - https://raw.githubusercontent.com/asdf12303116/cpa-plugin-commandcode-pool/main/registry.json
```

Reload or restart CPA, open **CPA Manager Plus → Plugin Store**, refresh the
store, and install **CommandCode Pool**. The installer verifies the release
checksum and writes the versioned library under `plugins/linux/amd64/` or
`plugins/linux/arm64/` matching the host architecture.

### Manual build and installation

```sh
make test
make build VERSION=0.2.1
make build VERSION=0.2.1 GOARCH=arm64 CC=aarch64-linux-gnu-gcc  # arm64 cross build
make package VERSION=0.2.1                                       # plugin-store zip + checksums
make package VERSION=0.2.1 ARCHS="amd64 arm64"                   # both architectures
```

Copy `dist/commandcode-pool-v0.2.1.so` into CPA's `plugins/linux/amd64/`
directory (use `dist/commandcode-pool-v0.2.1-arm64.so` and
`plugins/linux/arm64/` on arm64). The plugin ID is derived from the filename by
removing the version suffix, so the packaged `commandcode-pool.so` registers as
`commandcode-pool`.

## Configuration

The plugin reads CommandCode keys from the CPA config file itself. Any
credential whose `base-url` contains `commandcode.ai` is discovered, whether it
is an `openai-compatibility` provider entry or a `claude-api-key` entry:

```yaml
openai-compatibility:
  - name: commandcode
    base-url: https://api.commandcode.ai/provider/v1
    api-key-entries:
      - api-key: YOUR_COMMANDCODE_API_KEY_1

claude-api-key:
  - api-key: YOUR_COMMANDCODE_API_KEY_1
    base-url: https://api.commandcode.ai
```

Plugin settings live under `plugins.configs.commandcode-pool`:

| Setting | Default | Description |
| --- | --- | --- |
| `cpa-config-path` | `config.yaml` | CPA config file used for discovery |
| `api-base-url` | `https://api.commandcode.ai` | CommandCode API origin |
| `base-url-match` | `[commandcode.ai]` | Substrings identifying a CommandCode base URL |
| `usage-refresh-interval` | `3m` | Poll interval per key |
| `usage-stale-after` | `20m` | Age at which a reading is flagged stale |
| `include-usage-summary` | `true` | Call `/alpha/usage/summary` each cycle |
| `warn-percent` | `80` | Dashboard warning threshold |
| `critical-percent` | `95` | Dashboard critical threshold |
| `user-agent` | `curl/8.7.1` | User agent for `/alpha/*` requests |
| `accounts` | — | Per-account overrides matched by `key-suffix` |

Per-account overrides:

```yaml
plugins:
  configs:
    commandcode-pool:
      accounts:
        - key-suffix: KEY_1
          name: primary
        - key-suffix: KEY_2
          disabled: true
```

## Usage monitoring

Each enabled key is polled in the order the CommandCode CLI uses:

1. `GET /alpha/whoami?limits=1` — account identity and the `orgId` used as a
   query parameter for organization accounts (personal accounts must omit it).
2. `GET /alpha/billing/subscriptions` — `planId`, status, billing period,
   subscription id.
3. `GET /alpha/billing/credits` — remaining monthly credits, prepaid/free
   credits, and both rolling windows.
4. `GET /alpha/usage/summary?since=<billing period start>` — billing-period
   aggregates (skipped when `include-usage-summary: false`). Note that the API
   buckets this endpoint by UTC day, so the effective range starts at UTC
   midnight of the requested date.

Open the plugin page at:

```text
/v0/resource/plugins/commandcode-pool/status
```

The page shows one compact row per account and puts the full reading into each
cell's native tooltip. Columns depend on the plan kind, because CommandCode
limits the two kinds differently:

| Plan kind | Columns |
| --- | --- |
| Subscription (Go/GOAT/Pro/Max/Ultra/Teams) | Account, Plan, 5h, Weekly, Period, Spend, Health |
| Pay-as-you-go (`individual-provider`) | Account, Plan, Credits, Spend, Health |

Subscription plans are throttled by the rolling credit-value windows, so their
row leads with `5h` and `Weekly` (value used against the cap, with reset
timestamp and countdown in the tooltip). They deliberately have **no credits
column**: a subscription has no prepaid/top-up balance, so credit fields would
only add noise. Pay-as-you-go accounts have no windows and are limited by their
prepaid balance instead, so they show `Credits` (`$remaining` plus what was
topped up) and hide the window columns.

The two groups are labelled with the flag itself — `windowLimits.limited = true`
(**windows enforced**) vs `= false` — because that field is CommandCode's own
plan/non-plan discriminator. When the API has not reported it yet, the `planId`
is used as a fallback and the tooltip marks the value as inferred rather than
presenting a guess as fact.

Every column is **amount-based** — CommandCode throttles on credit/USD-equivalent
value rather than request quotas. Request counts, tokens and success rate appear
only as tooltip diagnostics.

**The page loads by itself — no clicking required.** On open it resolves the
management key automatically from CPA Manager Plus' persisted auth store, from
the key this page remembered earlier, or from the current tab, and immediately
fetches the data. A key typed or pasted into the field also triggers the load on
its own (Enter works too); the Load button is only a fallback. Keys are
remembered per browser, and **Forget key** removes the stored value.

## Management API

All management endpoints require the CPA management key.

| Route | Purpose |
| --- | --- |
| `GET /v0/management/plugins/commandcode-pool/status` | Full snapshot for every discovered key |
| `GET /v0/management/plugins/commandcode-pool/plans` | Static `planId` catalog |
| `POST /v0/management/plugins/commandcode-pool/refresh` | Trigger an immediate refresh pass |

The resource route `/v0/resource/plugins/commandcode-pool/status` serves the
page shell only and carries no account data.

Abbreviated `status` response:

```json
{
  "version": "0.2.1",
  "generated_at": "2026-09-10T14:20:00Z",
  "api_base_url": "https://api.commandcode.ai",
  "refresh_interval": "3m0s",
  "stale_after": "20m0s",
  "warn_percent": 80,
  "critical_percent": 95,
  "accounts": [
    {
      "name": "cc-1",
      "key_suffix": "AAAAAA",
      "providers": ["openai-compatibility:commandcode", "claude-api-key"],
      "stale": false,
      "refreshed_at": "2026-09-10T14:19:58Z",
      "data_age_seconds": 2,
      "identity": {"user_name": "asdf12303116", "email": "a6***3@gmail.com"},
      "plan": {
        "id": "individual-goat",
        "name": "GOAT",
        "known": true,
        "status": "active",
        "current_period_start": "2026-08-26T08:59:51Z",
        "current_period_end": "2026-09-26T08:59:51Z",
        "days_remaining": 15,
        "period_elapsed_percent": 49.1
      },
      "balance": {
        "monthly_remaining": 49.489633,
        "monthly_included": 70,
        "monthly_consumed": 20.510367,
        "monthly_consumed_percent": 29.3,
        "purchased_credits": 0,
        "free_credits": 0,
        "total_remaining": 49.489633,
        "below_threshold": false,
        "limited": true
      },
      "windows": {
        "5h": {"name": "5h", "used": 0.115138, "cap": 14, "headroom": 13.884862, "used_percent": 0.82, "blocked": false, "blocked_effective": false, "reset_at": "2026-09-10T15:54:59Z", "resets_in": "1h 35m"},
        "weekly": {"name": "weekly", "used": 0.401069, "cap": 35, "headroom": 34.598931, "used_percent": 1.15, "blocked": false, "blocked_effective": false, "reset_at": "2026-10-07T07:20:33Z", "resets_in": "26d 17h"}
      },
      "period_summary": {
        "since": "2026-08-26T08:59:51.000Z",
        "since_effective": "2026-08-26T00:00:00Z",
        "granularity": "utc-day",
        "period_basis": "billing-period",
        "requests": 6104, "success_rate": 100, "tokens_total": 817460082, "total_cost": 22.051453
      }
    }
  ]
}
```

## Field semantics worth knowing

The CommandCode usage API has a few traps that this plugin handles explicitly:

- `credits.monthlyCredits` is the **remaining** allowance, not consumption.
  Consumption is derived as `plan_total(planId) - monthlyCredits`, which is why
  the static `planId` catalog exists.
- Window `used`/`cap` are **credit/USD-equivalent values**, not percentages.
  Percentages and headroom are derived.
- `windowLimits.exceeded` is nullable and `null` means "no signal", never
  `false`. Blocking state therefore falls back to `used >= cap`.
- `resetAt` is epoch **milliseconds**.
- Prepaid credits (`purchasedCredits`) bypass rolling windows, so a capped
  window is reported with `blocked: true` but `blocked_effective: false` while
  prepaid balance remains.
- Multiple API keys can share one `subscriptions.data.id`; they consume one
  credit pool. Such accounts are listed under `shared_subscriptions`.
- `/alpha/usage/summary` buckets by **UTC day**: `since` is floored to UTC
  midnight, a future day returns zeros (a future time within today returns
  today's data), and a date before the first request returns all available data
  — it is *not* clamped to the billing period. The plugin therefore reports the
  requested `since`, the effective `since_effective`, and
  `granularity: "utc-day"`.
- `periodBasis` is always the literal `billing-period` regardless of the range
  actually returned, so it is not a reliable indicator of coverage.
- `/alpha/usage/summary` totals do not reconcile exactly with the balance-derived
  consumption (live check: `$0.4371` vs `$0.4537`); the plugin reports both
  without inventing a correction.
- There is no per-request history via API key. Per-request rows exist only in
  the CommandCode Studio web app, which uses session-cookie auth.

### `planId` catalog

| planId | Name | Monthly credits | 5h / weekly cap |
| --- | --- | --- | --- |
| `individual-go` | Go | 10 | 3 / 6 |
| `individual-goat` | GOAT | 70 | 14 / 35 |
| `individual-pro` | Pro (legacy) | 30 | — |
| `individual-pro-v1` | Pro | 80 | 16 / 40 |
| `individual-max` | Max 10× | 150 | 45 / 90 |
| `individual-ultra` | Max 20× | 300 | 90 / 180 |
| `individual-provider` | Provider | pay-as-you-go | none |
| `teams-pro` | Teams Pro | 40 | 12 / 24 |

Matching uses the lowercased `planId` with longest-prefix-first resolution, so
`individual-pro-v1` is never captured by `individual-pro`. Unknown plan ids are
shown with raw API values only (`"known": false`).

## Troubleshooting

- **No accounts discovered:** confirm `cpa-config-path` points at the config CPA
  actually loaded, and that the credential base URL contains `commandcode.ai`.
  `config_error` in the status response reports read/parse failures.
- **HTTP 403 with `error code 1010`:** Cloudflare rejected the request user
  agent. `user-agent` defaults to a curl-like value because the Go default was
  rejected during the original capture; current edges have also accepted the Go
  default, so the override is defensive rather than mandatory.
- **HTTP 400 `Invalid UUID at "orgId"`:** the plugin only sends `orgId` when
  `whoami` reported an organization, so this indicates an API-side change.
- **`stale: true`:** the authoritative `credits` reading is older than
  `usage-stale-after`; check the per-endpoint `errors` map.
- **`403 upgrade_required`:** the Go plan has no Provider API access, so its
  `/alpha/*` data may be unavailable.

## Development

```sh
make test     # gofmt check is in CI; runs go vet + go test
make build
make package VERSION=0.2.1
make clean
```

`TestLiveCommandCodeAPI` drives the real `/alpha/*` API through the same client,
parsing, and derivation code the plugin uses in production. It is skipped unless
a key is supplied, so CI stays offline:

```sh
COMMANDCODE_LIVE_KEY=user_... go test ./src -run TestLiveCommandCodeAPI -v
```

CI additionally checks formatting and builds the C ABI shared library for amd64
and arm64. Pushing a `v<version>` tag publishes the installer artifacts as a
GitHub Release.

## Security notes

- API keys are held in memory only for `/alpha/*` authentication and are never
  written to logs, status output, or the dashboard.
- Account state is keyed by a SHA-256 hash of the API key.
- Email addresses in the dashboard/API are masked (`a6***3@gmail.com`).
- The resource page is served unauthenticated, but it contains only the static
  page shell: every account reading still requires the CPA management key.
- The page remembers the management key in this browser's `localStorage` so it
  can auto-load on later visits. Use **Forget key** on the page to remove it.

## License

MIT — see [LICENSE](LICENSE).

The usage-API field semantics and plan catalog are derived from a local capture
of the CommandCode usage API; the CommandCode API is undocumented and may change
without notice.
