# CommandCode Pool for CLIProxyAPI

[English](README.md)

`commandcode-pool` 是一个只读的 [CLIProxyAPI](https://github.com/router-for-me/CLIProxyAPI)
插件，用于展示 CPA 中每一个 [CommandCode](https://commandcode.ai) 订阅的状态：
套餐状态与计费周期、剩余额度，以及 5 小时 / 每周滚动窗口的用量与重置时间。

它是**纯观测型**的：不参与选号、不封禁账号、不修改 `config.yaml`、不持久化任何
API Key。目的是让你直接在 CPA 面板里回答“哪个 CommandCode key 还有余额、什么时候
重置”，而不用打开 CommandCode Studio。

## 功能

- **凭据自动发现：** 所有 `base-url` 指向 `commandcode.ai` 的 key 都会被自动识别，
  无需逐账号配置。
- **套餐状态：** `planId` 解析为可读名称、订阅状态、数量、是否周期末取消，以及当前
  计费周期的已用比例和剩余天数。
- **额度余额：** 剩余月度额度、套餐总额度、推导出的已消耗额度与百分比、预付额度、
  赠送额度、低额度告警状态。
- **重置周期用量：** 5 小时与每周窗口的 `used`、`cap`、剩余额度、百分比、重置时间戳
  与实时倒计时。
- **计费周期花费：** 来自 `/alpha/usage/summary` 的额度成本；请求数/token/成功率仅作为
  次要诊断信息保留 —— CommandCode 的限额是按额度金额而非请求数计算的。
- **共享额度池识别：** 同一订阅下签发的多个 API Key 共享一个额度池，会被归组展示。
- **管理 API + 面板页：** 可在 CPA Manager Plus 插件页或直接用 JSON 查看。
- **失败安全：** 单个接口失败时保留上一次读数并记录错误；过期数据会被标记为
  `stale`，而不是当作实时数据展示。

## 依赖要求

- CLIProxyAPI v7.2.67 或兼容的 C ABI 插件版本
- Linux amd64 / arm64，glibc 2.36 及以上
- Go 1.26、开启 CGO 且本机有 C 编译器（仅编译时需要）
- 具备 Provider API 权限的 CommandCode 套餐（除 Go 之外的所有套餐；Go 套餐的
  key 会返回 `403 upgrade_required`）

## 目录结构

- `src/` —— 插件实现与测试。
- `.github/scripts/` —— 仅发布使用的打包脚本。
- `registry.json` —— 自定义 CPA 插件商店 registry。
- `config.example.yaml` —— 相关 CPA 配置片段。

本插件所依据的路由结构、字段语义、错误结构与套餐对照表，来自一份本地抓取的
CommandCode 用量 API 规范。该抓取内容包含账号标识信息，因此刻意不纳入本仓库，
已在 `.gitignore` 中忽略。

## 安装

### CPA Manager Plus 插件商店

在 CPA 的 `config.yaml` 中加入本仓库的 registry，并确保插件目录在容器替换后仍然保留：

```yaml
plugins:
  enabled: true
  dir: plugins
  store-sources:
    - https://raw.githubusercontent.com/asdf12303116/cpa-plugin-commandcode-pool/main/registry.json
```

重载或重启 CPA，打开 **CPA Manager Plus → Plugin Store**，刷新商店并安装
**CommandCode Pool**。安装器会校验发布包的 checksum，并按主机架构写入
`plugins/linux/amd64/` 或 `plugins/linux/arm64/`。

### 手动编译安装

```sh
make test
make build VERSION=0.2.3
make build VERSION=0.2.3 GOARCH=arm64 CC=aarch64-linux-gnu-gcc  # arm64 交叉编译
make package VERSION=0.2.3                                       # 生成插件商店 zip 与校验和
make package VERSION=0.2.3 ARCHS="amd64 arm64"                   # 两种架构
```

将 `dist/commandcode-pool-v0.2.3.so` 拷贝到 CPA 的 `plugins/linux/amd64/`
（arm64 使用 `dist/commandcode-pool-v0.2.3-arm64.so` 与 `plugins/linux/arm64/`）。
插件 ID 由文件名去掉版本后缀得到，因此打包出的 `commandcode-pool.so` 注册为
`commandcode-pool`。

## 配置

插件直接从 CPA 配置文件读取 CommandCode key。任何 `base-url` 含 `commandcode.ai`
的凭据都会被识别，无论它位于 `openai-compatibility` 还是 `claude-api-key`：

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

插件设置位于 `plugins.configs.commandcode-pool`：

| 设置项 | 默认值 | 说明 |
| --- | --- | --- |
| `cpa-config-path` | `config.yaml` | 用于发现 key 的 CPA 配置文件 |
| `api-base-url` | `https://api.commandcode.ai` | CommandCode API 地址 |
| `base-url-match` | `[commandcode.ai]` | 用于识别 CommandCode base URL 的子串 |
| `usage-refresh-interval` | `3m` | 每个 key 的轮询间隔 |
| `usage-stale-after` | `20m` | 超过该时长标记为过期数据 |
| `include-usage-summary` | `true` | 每轮是否调用 `/alpha/usage/summary` |
| `warn-percent` | `80` | 面板警告阈值 |
| `critical-percent` | `95` | 面板严重阈值 |
| `user-agent` | `curl/8.7.1` | `/alpha/*` 请求使用的 User-Agent |
| `accounts` | — | 按 `key-suffix` 匹配的单账号覆盖项 |

单账号覆盖项：

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

## 用量采集

每个启用的 key 按 CommandCode CLI 的调用顺序轮询：

1. `GET /alpha/whoami?limits=1` —— 账号身份，以及组织账号用于查询参数的 `orgId`
   （个人账号必须省略该参数）。
2. `GET /alpha/billing/subscriptions` —— `planId`、状态、计费周期、订阅 ID。
3. `GET /alpha/billing/credits` —— 剩余月度额度、预付/赠送额度、两个滚动窗口。
4. `GET /alpha/usage/summary?since=<计费周期开始>` —— 计费周期汇总
   （`include-usage-summary: false` 时跳过）。注意该接口按 UTC 天分桶，因此真实生效的
   范围从所请求日期的 UTC 零点开始。

面板页面地址：

```text
/v0/resource/plugins/commandcode-pool/status
```

页面每个账号一行（紧凑单行表格）；**点击该行**（或聚焦后按 Enter/Space）会在行下方展开详情
面板，完整读数都在那里，不需要鼠标悬停。列的组成取决于套餐类型，因为 CommandCode 对两类套餐的
限流方式不同：

| 套餐类型 | 列 |
| --- | --- |
| 订阅制（Go/GOAT/Pro/Max/Ultra/Teams） | Account, Plan, 5h, Weekly, Period, Health |
| 按量付费（`individual-provider`） | Account, Plan, Credits, Spend, Health |

订阅制由滚动额度窗口限流，所以行内以 `5h`、`Weekly` 为主（已用金额对比上限，重置时间与
倒计时在展开的详情面板里），并且**刻意不显示 credits 与 spend 列** —— 订阅制没有充值/预付
余额，
日常限制就是窗口，这两列只会造成干扰。按量付费账号没有窗口、由预付余额限流，因此显示
`Credits`（剩余金额及充值金额）和 `Spend`，并隐藏窗口列。

分组标题直接用该字段本身标注：`windowLimits.limited = true`（**windows enforced**）
与 `= false` —— 这个字段就是 CommandCode 自己的 plan / 非 plan 判据。当 API 尚未返回该
字段时，回退用 `planId` 判断，并在详情面板中明确标注是推断值，不会把猜测当成事实。

每一列都是**金额口径** —— CommandCode 的限流基于额度/USD 等值而非请求数；请求数、token、
成功率只出现在展开面板的 diagnostics 分区里。

页面外观直接继承 CPA Manager Plus：cpamp 用 iframe 内嵌插件页，并注入它的设计 token、
`data-theme` 以及宿主字体（`--cpamp-plugin-font-family`，即 `Inter, -apple-system,
BlinkMacSystemFont, "Segoe UI", sans-serif`），因此页面会自动跟随面板的明/暗主题、色板、
圆角与字体，数字等宽部分取自 `--font-family-mono`。单独打开时则回退到 cpamp 自身的浅色/
暗色色板（通过 `prefers-color-scheme`）。

**页面会自动加载，无需任何点击。** 打开时会依次从 CPA Manager Plus 持久化的鉴权存储、
本页之前记住的 key、当前标签页中自动解析管理密钥并立即拉取数据；在输入框里粘贴或输入
key 也会自动触发加载（回车同理），Load 按钮只是兜底。key 会按浏览器记住，之后访问无需
任何交互即可自动加载。

## 管理 API

所有管理接口都需要 CPA 管理密钥。

| 路由 | 用途 |
| --- | --- |
| `GET /v0/management/plugins/commandcode-pool/status` | 所有已发现 key 的完整快照 |
| `GET /v0/management/plugins/commandcode-pool/plans` | 静态 `planId` 对照表 |
| `POST /v0/management/plugins/commandcode-pool/refresh` | 立即触发一次刷新 |

资源路由 `/v0/resource/plugins/commandcode-pool/status` 只提供页面壳，不含任何账号数据。

`status` 响应示例（节选）：

```json
{
  "version": "0.2.3",
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

## 必须知道的字段语义

CommandCode 用量 API 有几个坑，插件已显式处理：

- `credits.monthlyCredits` 是**剩余**额度而非已消耗量。已消耗量由
  `plan_total(planId) - monthlyCredits` 推导，这也是静态 `planId` 对照表存在的原因。
- 窗口的 `used`/`cap` 单位是**额度/USD 等值**，不是百分比；百分比与剩余额度是推导值。
- `windowLimits.exceeded` 可为 null，且 null 表示“无信号”而不是 `false`。因此封顶
  判断回退为 `used >= cap`。
- `resetAt` 是**毫秒**时间戳。
- 预付额度（`purchasedCredits`）不受滚动窗口限制，因此窗口封顶时会显示
  `blocked: true` 但 `blocked_effective: false`（仍有预付余额）。
- 多个 API Key 可能共享同一个 `subscriptions.data.id`，也就共享一个额度池，这类账号
  会列在 `shared_subscriptions` 中。
- `/alpha/usage/summary` 按 **UTC 天**分桶：`since` 会被向下取整到 UTC 零点；未来的
  “某一天”返回 0（但今天之内的未来时刻返回今天的数据）；早于首个请求的日期返回全部可用
  数据，**不会**被 clamp 到计费周期。因此插件同时给出请求值 `since`、实际生效的
  `since_effective` 与 `granularity: "utc-day"`。
- `periodBasis` 恒为字面量 `billing-period`，与真实返回范围无关，不能作为覆盖范围的依据。
- `/alpha/usage/summary` 的汇总与余额推导出的消耗并不完全一致（实测 `$0.4371` vs
  `$0.4537`）；插件两者都展示，不做臆测修正。
- 通过 API Key 无法获取逐请求明细；逐请求数据只存在于使用 session cookie 认证的
  CommandCode Studio 网页端。

### `planId` 对照表

| planId | 名称 | 月度额度 | 5h / 每周上限 |
| --- | --- | --- | --- |
| `individual-go` | Go | 10 | 3 / 6 |
| `individual-goat` | GOAT | 70 | 14 / 35 |
| `individual-pro` | Pro（旧版） | 30 | — |
| `individual-pro-v1` | Pro | 80 | 16 / 40 |
| `individual-max` | Max 10× | 150 | 45 / 90 |
| `individual-ultra` | Max 20× | 300 | 90 / 180 |
| `individual-provider` | Provider | 按量付费 | 无 |
| `teams-pro` | Teams Pro | 40 | 12 / 24 |

匹配时对 `planId` 转小写并按最长前缀优先，因此 `individual-pro-v1` 不会被
`individual-pro` 抢先匹配。未知 planId 只展示 API 原始值（`"known": false`）。

## 故障排查

- **没有发现任何账号：** 确认 `cpa-config-path` 指向 CPA 实际加载的配置，且凭据
  `base-url` 含 `commandcode.ai`。读取/解析失败会体现在状态响应的 `config_error`。
- **HTTP 403 `error code 1010`：** Cloudflare 拒绝了 User-Agent。`user-agent` 默认
  为 curl 风格，是因为最初抓取时 Go 默认 UA 被拦截；目前边缘节点也已放行 Go 默认
  UA，因此该覆盖是防御性的而非必需。
- **HTTP 400 `Invalid UUID at "orgId"`：** 插件只在 `whoami` 返回组织时才发送
  `orgId`，出现该错误说明 API 侧行为有变。
- **`stale: true`：** 权威的 `credits` 读数已超过 `usage-stale-after`；查看各接口的
  `errors` 字段。
- **`403 upgrade_required`：** Go 套餐没有 Provider API 权限，其 `/alpha/*` 数据可能
  不可用。

## 开发

```sh
make test     # 格式检查在 CI 中；此处运行 go vet + go test
make build
make package VERSION=0.2.3
make clean
```

`TestLiveCommandCodeAPI` 会用插件生产路径上同一套 client、解析与推导代码，真实调用
`/alpha/*`。未提供 key 时会跳过，因此 CI 始终离线运行：

```sh
COMMANDCODE_LIVE_KEY=user_... go test ./src -run TestLiveCommandCodeAPI -v
```

CI 还会检查格式，并为 amd64 与 arm64 编译 C ABI 动态库。推送 `v<version>` tag 会将
安装包发布到 GitHub Release。

## 安全说明

- API Key 仅在内存中用于 `/alpha/*` 认证，绝不写入日志、状态输出或面板。
- 账号运行状态以 API Key 的 SHA-256 哈希作为 key。
- 面板/API 中的邮箱会被打码（`a6***3@gmail.com`）。
- 资源页无需认证即可访问，但只包含静态页面壳：所有账号读数仍然需要 CPA 管理密钥。
- 为便于下次自动加载，页面会把管理密钥记住在本浏览器的 `localStorage` 中；不需要保留时，
  用浏览器的站点数据/清除存储功能删掉即可。

## 许可证

MIT，见 [LICENSE](LICENSE)。

用量 API 的字段语义与套餐对照表来自本地抓取的 CommandCode 用量 API 规范；
CommandCode API 未公开文档，可能随时变化。
