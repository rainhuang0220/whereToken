# 数据源实测（2026-08-15，本机 macOS）

规格里的适配器合同以本文件为准。数字是**这一天这一台机器**的快照，用来做夹具和核验基线，不是产品承诺的「你也有这么多」。

扫描原则：只读；跳过 `auth.json` / `credentials` / Keychain；不把 prompt 正文写入 whereToken 自己的库（v1）。Cursor / Trae 用量接口是用户授权的例外：只使用它们本机已写入的登录态，只打它们自己的主机，永不打印或提交 token。发现路径一律经 `Home`（`os.UserHomeDir()`、XDG、Application Support、`%APPDATA%`），缺目录静默跳过。额外家目录：`WHERETOKEN_EXTRA_ROOTS`。

工具和厂家不是同一件事。本机 Claude Code 的 assistant 模型分布含 `claude-opus-4.6`、`claude-haiku-4.5`、`MiniMax-M3`、`claude-opus-5`：前两者/后者是 Anthropic，`MiniMax-M3` 必须进厂家 MiniMax、工具仍是 Claude Code。Cursor 里的 `grok-*` 走厂家 `xai`（标签 xAI），不是未知厂家。Grok CLI（`~/.grok/sessions`）是单独的**工具**适配器，厂家仍是 xAI。

---

## 发现器要看的根

用户点名：`~/.claude`、`~/.kimi`、`~/.codex`、`~/.gpt`、`~/.opencode`，「有什么算什么」。

本机实际存在、且与 coding agent 相关的根：

| 路径 | 工具 | 用量是否可解析 | v1 |
|------|------|----------------|----|
| `~/.claude/` | Claude Code | **是**（projects JSONL） | P0 |
| `~/.codex/` | Codex CLI | **是**（sessions rollout JSONL） | P0 |
| `~/.kimi-code/` | Kimi Code | **是**（`usage.record`） | P0 |
| `~/.grok/sessions/` | Grok CLI | **是**（`updates.jsonl` `turn_completed.usage`） | P0 |
| `~/.local/share/opencode/` | OpenCode | **是**（`opencode.db` session/message tokens） | P0 |
| `~/.opencode/` | OpenCode 安装目录 | 否（只有 npm 包装） | 忽略 |
| `~/.config/opencode/` | OpenCode 配置 | 否（无用量） | 忽略 |
| `~/.cursor/` | Cursor | 目录在；ai-tracking **无 token 列**（请求次数也不能当模型调用） | 发现回退 |
| `~/Library/Application Support/Cursor/User/globalStorage/state.vscdb` | Cursor | **是**（bubble 请求/回合 + 本机登录态调 Cursor DashboardService） | P0；token 来自账号 API |
| `~/Library/Application Support/{Trae,Trae CN,Trae-CN,TRAE SOLO CN}/User/globalStorage/state.vscdb` | Trae | **是**（本机会话 id + 本机 JWT 调 Trae `get_session_usage`） | P0；token 来自账号 API |
| `~/.minimax/` | MiniMax agent | sqlite 仅 `agents` 表，未见 token | P1 探测 |
| `~/.copilot/` | Copilot CLI | 未见 otel jsonl | P1 探测 |
| `~/.openclaw/` | OpenClaw | 目录存在，本轮未深挖字段 | P1 |
| `~/.trae` / `~/.trae-cn` | Trae | 技能、MCP、**jwt 文件**（`trae-jwt-token`）；不是 JSONL 账本 | jwt 仅作登录态；技能目录忽略 |
| `~/Library/Application Support/{Claude,Codex,Kimi,kimi-desktop,ai.opencode.desktop}` | 各桌面壳 | Electron 缓存为主 | 不作为 P0 账本 |
| `~/.gpt` / `~/.kimi`（无 `-code`） | 用户点名 | **本机不存在** | 发现器仍要查；Kimi CLI 文档路径是 `~/.kimi/sessions` |

本机**没有**的常见根（适配器仍应探测，缺席则静默）：`~/.gemini`、`~/.qwen`、`~/.factory`、`~/.hermes`、`~/.pi`、`~/.amp`、`~/.config/claude`。

---

## P0-1 Claude Code

**根：** `~/.claude/projects/<workspace-slug>/*.jsonl`（含子 agent 路径 `*/subagents/*.jsonl`）

**本机：** 5 个 project 目录，41 个 jsonl。

**事件：** `type=assistant` 的 `message.usage`：

```json
{
  "input_tokens": 17979,
  "output_tokens": 872,
  "cache_creation_input_tokens": 1161,
  "cache_read_input_tokens": 43599,
  "cache_creation": {
    "ephemeral_5m_input_tokens": 1161,
    "ephemeral_1h_input_tokens": 0
  }
}
```

**映射：**

| whereToken | Claude 字段 |
|------------|-------------|
| miss | `input_tokens` |
| cache_create | `cache_creation_input_tokens` |
| cache_read | `cache_read_input_tokens` |
| output | `output_tokens`（已知可能偏小，见质量） |
| requests | 按 `requestId` 去重后的 assistant 条数；无 `requestId` 则用 `message.id`。不用每行唯一的 `uuid`（会把流式占位加成多次请求） |
| user_turns | `type=user` 且 content **不是** `tool_result` |

**本机原始求和（未按 requestId 去重，仅作量级）：**

| 项 | token | M |
|----|------:|--:|
| miss | 97,763,998 | 97.76 |
| cache_create | 8,952,459 | 8.95 |
| cache_read | 252,128,914 | 252.13 |
| output | 1,264,514 | 1.26 |
| **total** | **360,109,885** | **360.11** |
| 命中率 | 70.26% | |
| assistant 行 | 4118 | |
| type=user | 3394 | |
| 其中 tool_result | 3289 | |
| **真用户回合** | **~105** | |

**质量陷阱（必须做）：**

1. 社区与 Anthropic issue 证实：大量行的 `input_tokens`/`output_tokens` 是流式占位（0/1），最终值不写回 JSONL；cache 字段相对可信。
2. 同一 `requestId` 可能出现多次。策略：同一 requestId **取各字段最大值**，不要求和。
3. `~/.claude/stats-cache.json` 本机 `lastComputedDate=2026-03-07`，已过期，**不能**当权威总量。
4. `settings.json` 含 `ANTHROPIC_AUTH_TOKEN` —— 扫描器不得打开此文件。用量只走 projects JSONL。

**工作区：** 目录名 `-Users-rainhuang-Desktop-Flow` → 还原 `~/Desktop/Flow`（存在性校验）。

---

## P0-2 Kimi Code

**根：** `~/.kimi-code/sessions/<workDirKey>/<sessionId>/agents/*/wire.jsonl`

**权威事件：** `type=usage.record`

```json
{
  "type": "usage.record",
  "model": "kimi-code/k3",
  "usage": {
    "inputOther": 5199,
    "output": 173,
    "inputCacheRead": 18944,
    "inputCacheCreation": 0
  },
  "usageScope": "turn",
  "time": 1786722944364
}
```

**映射：**

| whereToken | Kimi 字段 |
|------------|-----------|
| miss | `usage.inputOther` |
| cache_read | `usage.inputCacheRead` |
| cache_create | `usage.inputCacheCreation` |
| output | `usage.output` |
| requests | `usage.record` 条数（本机均为 `usageScope=turn`） |
| user_turns | `type=turn.prompt` 且 `origin.kind==user` |
| 时间 | `time` 毫秒时间戳 |

**本机合计（12 个 wire，1661 条 usage.record）：**

| 项 | token | M |
|----|------:|--:|
| miss | 3,931,677 | 3.93 |
| cache_read | 325,211,363 | 325.21 |
| cache_create | 0 | 0.00 |
| output | 898,658 | 0.90 |
| **total** | **330,041,698** | **330.04** |
| 命中率 | 98.81% | |
| 用户回合 | 44 | |

这是 v1 **第一个黄金夹具**：字段完整、无流式占位、总量可复算。

**不要用：** `telemetry/*.jsonl`（本机 309 个文件，几乎无 token 字段，且含工具名/策略）。`state.json` 无用量。`config.toml` / `credentials/` 禁止读。

---

## P0-Grok CLI

**根：** `~/.grok/sessions/<url-encoded workspace>/<sessionId>/updates.jsonl`

**权威事件：** `params.update.sessionUpdate=turn_completed` 且带 `usage` 与 `prompt_id`。`chat_history.jsonl` / `events.jsonl` / `summary.json` / `auth.json` 不读。永不映射 `costUsdTicks`。

**映射：**

| whereToken | Grok 字段 |
|------------|-----------|
| miss | `inputTokens - cachedReadTokens - cacheCreationTokens`（小于 0 记 0；`inputTokens` 含缓存） |
| cache_read | `cachedReadTokens` |
| cache_create | `cacheCreationTokens` |
| output | `outputTokens` |
| reasoning | `reasoningTokens`（不进合计） |
| requests | 有 `prompt_id` 的 `turn_completed`；`modelUsage` 多于一个模型时按 `prompt_id:model` 拆 |
| user_turns | `user_message_chunk` 且正文非空、不是 `<system-reminder>` |
| 时间 | `_meta.agentTimestampMs`，否则 `timestamp` 秒 |

**不要用：** `~/.grok/auth.json`、会话里的 `terminal/` 日志、compaction 正文。

---

## P0-3 OpenCode

**根：** `~/.local/share/opencode/opencode.db`（也探测 `opencode-stable.db`）

**只读 URI：** `file:<path>?mode=ro`

**两层数据，本机一致，优先 message 级以便按时间切：**

`session` 列：`tokens_input`, `tokens_output`, `tokens_reasoning`, `tokens_cache_read`, `tokens_cache_write`

`message.data` JSON：`tokens.input` / `tokens.output` / `tokens.cache.read` / `tokens.cache.write`

`part.data` 里 `type=step-finish` 也带 `tokens` —— **不要和 message 再加一遍**。

**映射：** miss=`tokens.input`，output=`tokens.output + tokens.reasoning`（reasoning 另列展示），cache_read=`tokens.cache.read`，cache_create=`tokens.cache.write`。

**本机 session 合计 = message 合计：**

| 项 | token | M |
|----|------:|--:|
| miss | 185,576 | 0.19 |
| cache_read | 1,533,184 | 1.53 |
| cache_create | 0 | 0.00 |
| output | 22,751 | 0.02 |
| **total** | **1,741,511** | **1.74** |
| 会话数 | 7 | |

**禁止：** 读取 `account` / `control_account` / `credential` 表（列名就叫 access_token）。

---

## P0-4 Codex CLI

**根：** `${CODEX_HOME:-~/.codex}/sessions/YYYY/MM/DD/rollout-*.jsonl`，并探测 `archived_sessions/`。

**本机：** 29 个 rollout；单文件最大约 24 MB。必须**逐行流式**，禁止 `ReadFile` 整文件进内存。

**结构：** `{ timestamp, type, payload }`

用量在 `type=event_msg && payload.type=token_count`：

```json
{
  "info": {
    "last_token_usage": {
      "input_tokens": 0,
      "cached_input_tokens": 0,
      "output_tokens": 0,
      "reasoning_output_tokens": 0,
      "total_tokens": 6036
    },
    "total_token_usage": { "...cumulative..." }
  }
}
```

**解析规则（锁死，来自 ccusage issue #884 与本机重复 snapshot）：**

1. 以 `info.total_token_usage` 为累计值。
2. 仅当累计比上一事件**前进**时，把 **delta** 记为一次请求用量。
3. 累计不变 → 丢弃（重复 snapshot）。
4. 没有累计时才退回 `last_token_usage`，且仍要去重。
5. miss = `input_tokens - cached_input_tokens`（若减出负值则 miss=0，cached 记满）。
6. cache_read = `cached_input_tokens`；cache_create = 0（Codex 不暴露 write）。
7. output = `output_tokens + reasoning_output_tokens`。
8. 模型来自最近的 `turn_context.payload.model`。
9. user_turns：`response_item` 里 role=user 的 message，或 `input_item` 用户输入，两者定义在适配器测试里钉死，禁止双计。

**不要读：** `auth.json`、`logs_2.sqlite`（61 MB，用途不明且可能含敏感日志）、Keychain。

---

## P0-5 Cursor（2026-08-15 复测，本机 macOS）

**根：** `~/Library/Application Support/Cursor/User/globalStorage/state.vscdb`（2.3 GB + 4.5 MB WAL）。`~/.cursor` 只作发现回退。

**用户授权（2026-08-15 23:00）：** 可用本机 Cursor 已经写入的登录态，向 **Cursor 自己的** 用量接口拉取 **当前这个账号** 的 token。不上其它厂商云、不上传 chats、不把 token/会话写进 git。

**登录态（只读键名，永不打印值）：** `ItemTable` 键 `cursorAuth/accessToken`（Bearer）。缺席时再看同目录 `storage.json` 的同名键。401 时才读 `cursorAuth/refreshToken`，仅内存刷新，不写回 Cursor 库。禁止 Cookie 商店 / Keychain / 把 JWT 写进夹具。

**用量接口（Cursor 桌面端已在调用的 DashboardService，Connect JSON）：**

| 方法 | 路径 |
|------|------|
| 带时间戳的明细（日历优先） | `POST https://api2.cursor.sh/aiserver.v1.DashboardService/GetFilteredUsageEvents` |
| 按模型合计（明细空时回退） | `POST https://api2.cursor.sh/aiserver.v1.DashboardService/GetAggregatedUsageEvents` |

请求窗：本地时区往前 53 周。`Authorization: Bearer <accessToken>`，主机只允许 `api2.cursor.sh` / `cursor.com`。测试用 `httptest`，夹具 JWT 为 `test-token`。

**字段映射：**

| whereToken | 来源 |
|------------|------|
| miss | API `inputTokens` |
| cache_read | API `cacheReadTokens` |
| cache_create | API `cacheWriteTokens` |
| output | API `outputTokens` |
| 时间 / 窑墙 | `GetFilteredUsageEvents` 的 `timestamp`（unix ms）。若只有 aggregations、没有逐条时间戳，token 记在扫描当日，窑墙没有真实日分布 |
| 模型 / 厂家 | `model` / `modelIntent` → `vendor.Lookup` |
| 质量 | API 有 token → **authoritative** |
| requests | **本机 bubble**：`type=2` 且 `capabilityType != 30` |
| user_turns | **本机 bubble**：`type=1` 且不是 subagent |
| 时间（请求/回合） | bubble `createdAt` |

**守恒：** token 合计只来自账号 API 事件；本机 0-token 请求行不进入 token 总和（SkipRequest 的 API 行不计入 requests）。不要把 API token 和 bubble `tokenCount` 再加一遍。

**本机 bubble 仍扫（键前缀，不取正文）：** 请求/回合定义同上。`tokenCount` 本机仍几乎全 0，有 API 时忽略本地 tokenCount，避免双计。

**明确不算用量：** `promptTokenBreakdown` / `contextTokensUsed` / `contextWindowStatusAtCreation` / `ai-code-tracking.db` 无 token 列。

**缺登录态：** 仍发出本机请求/回合；`errors[]` 含 `cursor: 未找到本机登录态`；token 列保持 degraded/0。

**核验：** `python3 scripts/sum_cursor.py` 只对照 **requests / user_turns**。token 四列以 `scan --json` 的 Cursor 账号 API 为准。

**本机库结构（请求/回合，键前缀查询）：** `cursorDiskKV`、`composerHeaders`、`ItemTable`（登录态键，不是用量）。禁止 `SELECT *` / 把 `cursorDiskKV` value blob 整行读进内存。`composerData:%` 的 `usageData` 本机全空；`bubbleId:%` 的 `tokenCount` 本机全 0，不能当账单。`ItemTable` 只按键名读 `cursorAuth/accessToken`（及 401 时的 `refreshToken`）。

**本机请求/回合快照（2026-08-15 22:57，前缀查询）：** 真用户回合 1,827；请求 43,550；tokenCount 非 0 行 0。厂家拆分是请求数不是 token。

**明确不算用量（会虚增数百万）：** `promptTokenBreakdown.totalUsedTokens`、`contextTokensUsed`、`bubble.contextWindowStatusAtCreation.tokensUsed`、`ItemTable` `aiCodeTracking.dailyStats.*`、`~/.cursor/ai-tracking/ai-code-tracking.db`（无 token 列）。

**workspaceStorage/\*/state.vscdb：** 不必再扫；全局 `composerHeaders` 已覆盖工作区路径。

---

## P0-6 Trae（ByteDance VS Code fork；国内版 Trae CN / TRAE SOLO CN）

**根（有哪个扫哪个，缺则静默）：**

- macOS：`~/Library/Application Support/{Trae,Trae CN,Trae-CN,TRAE SOLO,TRAE SOLO CN}/User/globalStorage/state.vscdb`
- Linux：`~/.config/Trae*` 同样的 `User/globalStorage/state.vscdb`
- Windows：`%APPDATA%\Trae*`（路径已写，未在真实 Windows 上测）
- 登录态文件：`~/.trae-cn/trae-jwt-token` 或 `~/.trae/trae-jwt-token`（明文 JWT）。`storage.json` 键 `iCubeAuthInfo://icube.cloudide` 若是 JSON 明文也读 `token`；CN 本机该值是加密串，**不解密**。

**不要读：** Cookies、Keychain、`ModularData/ai-agent/database.db`（SQLCipher）、技能目录、prompt 正文。永不打印 JWT。

**本机会话 id（只取 id，不取消息）：** `memento/icube-ai-agent-storage` 的 `sessionId`、`icube_session_agent_map` 的键、`all_session_badges_*`。workspaceStorage 下的 `state.vscdb` 同样只抽这些键。

**用量接口（Trae 桌面端已在调用的 commercial API）：**

`POST {host}/api/v1/commercial/get_session_usage`  
body：`{"session_id":"<id>"}`  
`Authorization: Cloud-IDE-JWT <token>`  
CN 主机 `trae-api-cn.mchost.guru`；国际版路径不含 `CN` 时用 `coresg-normal.trae.ai`。测试用 `httptest`，夹具 JWT 为 `test-token`。

**字段映射（`user_usage_group_by_session.extra_info`）：**

| whereToken | Trae 字段 |
|------------|-----------|
| miss | `input_token - cache_read_token`（负则 0；`input_token` 含缓存读） |
| cache_read | `cache_read_token` |
| cache_create | `cache_write_token` |
| output | `output_token` |
| 模型 / 厂家 | `model_name` → `vendor.Lookup`（DeepSeek / Doubao / GLM / Qwen…，**不是** trae） |
| 质量 | API 有 token → **authoritative** |
| user_turns | 每个 billing `session_id` 一次（Trae 把该字段当 user message id） |
| 请求 | 对应 API 事件数（无本地 bubble 可双计） |

**缺登录态 / 登录态失效：** `errors[]` 含 `trae: 未找到本机登录态` 或 `trae: 本机登录态已失效`；token 列为 **degraded** 且 0，不编造。在 Trae 里重新登录后点 whereToken 的 **刷新**（不要只重载浏览器）。whereToken 不接收粘贴的 JWT。

---

## 发现器启发式（P1）

对 `$HOME/.*` 以及 macOS `~/Library/Application Support/*`：

1. 跳过：`.Trash`、`.cache`、`.npm`、`.nvm`、`.docker`、`node_modules`、`GPUCache`、`Cache`、`Crashpad`、体积 > 阈值且无已知 schema 的 sqlite。
2. 命中：目录内存在 `**/wire.jsonl`、`**/rollout-*.jsonl`、含 `usage`/`token` 列的 sqlite、`projects/**/*.jsonl` 且行内有 `cache_read_input_tokens`。
3. 未知命中进「未适配」列表：给路径、文件数、抽样到的字段名，**不**把正文当用量加进总表。

---

## 核验时用的对照命令（实现阶段）

这些不是产品功能，是开发者核验：

```bash
# Kimi：usage.record 再求和，应等于仪表盘 P0-Kimi
# Claude：requestId 去重后的 max(usage)，并输出质量旗标比例
# OpenCode：SELECT 四列 SUM，应等于 message.tokens 聚合
# Codex：每个 rollout 最后一个前进的 total_token_usage，应等于 delta 之和
# Cursor：sum_cursor.py 对照 requests/user_turns；token 四列以账号 API 为准
# Trae：夹具见 internal/adapter/trae；本机 JWT 永不打印
# python3 scripts/sum_cursor.py
```

夹具放 `testdata/adapters/<source>/`：从真实文件**脱敏**后的 20–50 行样本（去掉 prompt 正文，保留 usage 与 type）。
