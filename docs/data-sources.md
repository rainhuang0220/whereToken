# 数据源实测（2026-08-15，本机 macOS）

规格里的适配器合同以本文件为准。数字是**这一天这一台机器**的快照，用来做夹具和核验基线，不是产品承诺的「你也有这么多」。

扫描原则：只读；跳过 `auth.json` / `credentials` / Keychain；不把 prompt 正文写入 whereToken 自己的库（v1）。

工具和厂家不是同一件事。本机 Claude Code 的 assistant 模型分布含 `claude-opus-4.6`、`claude-haiku-4.5`、`MiniMax-M3`、`claude-opus-5`：前两者/后者是 Anthropic，`MiniMax-M3` 必须进厂家 MiniMax、工具仍是 Claude Code。

---

## 发现器要看的根

用户点名：`~/.claude`、`~/.kimi`、`~/.codex`、`~/.gpt`、`~/.opencode`，「有什么算什么」。

本机实际存在、且与 coding agent 相关的根：

| 路径 | 工具 | 用量是否可解析 | v1 |
|------|------|----------------|----|
| `~/.claude/` | Claude Code | **是**（projects JSONL） | P0 |
| `~/.codex/` | Codex CLI | **是**（sessions rollout JSONL） | P0 |
| `~/.kimi-code/` | Kimi Code | **是**（`usage.record`） | P0 |
| `~/.local/share/opencode/` | OpenCode | **是**（`opencode.db` session/message tokens） | P0 |
| `~/.opencode/` | OpenCode 安装目录 | 否（只有 npm 包装） | 忽略 |
| `~/.config/opencode/` | OpenCode 配置 | 否（无用量） | 忽略 |
| `~/.cursor/` | Cursor | 目录在；ai-tracking **无 token 列**（请求次数也不能当模型调用） | 发现回退 |
| `~/Library/Application Support/Cursor/User/globalStorage/state.vscdb` | Cursor | **是**（bubble 请求/回合；tokenCount 本机全 0） | P0 账本，键前缀查询 |
| `~/.minimax/` | MiniMax agent | sqlite 仅 `agents` 表，未见 token | P1 探测 |
| `~/.copilot/` | Copilot CLI | 未见 otel jsonl | P1 探测 |
| `~/.openclaw/` | OpenClaw | 目录存在，本轮未深挖字段 | P1 |
| `~/.trae` / `~/.trae-cn` | Trae | 技能与工作区，未见 token 账本 | P1 |
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
| requests | 按 `requestId` 去重后的 assistant 条数；无 requestId 则退回 `uuid` |
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

**根：** `~/Library/Application Support/Cursor/User/globalStorage/state.vscdb`（2.3 GB + 4.5 MB WAL）。`~/.cursor` 只作发现回退。禁止 Cookie / Keychain / `cursor.com` CSV。

**表：** `cursorDiskKV`（KV）、`composerHeaders`（会话索引，含 `isSubagent` 与工作区路径）、`ItemTable`（UI 状态，不用作用量）。

**只允许键前缀查询**（生产代码禁止 `SELECT *` / `SELECT value FROM cursorDiskKV`）：

| 前缀 | 本机行数 | 用途 |
|------|--------:|------|
| `composerData:%` | 328 | 会话默认 `modelConfig.modelName`；`usageData` **全部空对象** |
| `bubbleId:%` | ~55,458 | 消息：`type` 1=用户 / 2=助手或工具；`tokenCount.inputTokens` / `outputTokens`；`capabilityType`；`modelInfo.modelName`（只出现在用户泡） |
| `agentKv:%` | ~107k | 不读（blob，无用量列） |
| `composer.content.%` / `checkpointId:%` / `inlineDiff:%` | 大 | 不读 |

**bubble 字段（json_extract，不取 `text` / `thinking` / `toolFormerData` 正文）：**

| whereToken | Cursor 字段 |
|------------|-------------|
| miss | `tokenCount.inputTokens`（本机 **全部为 0**） |
| output | `tokenCount.outputTokens`（本机 **全部为 0**） |
| cache_read / cache_create | 无本地列；恒 0 |
| requests | `type=2` 且 `capabilityType != 30`（30=thinking，与随后的工具/正文同一代，不另计请求） |
| user_turns | `type=1` 且 composer **不是** `composerHeaders.isSubagent` |
| 时间 | bubble `createdAt` RFC3339 |
| 模型 | 用户泡 `modelInfo.modelName`，沿用到随后的助手泡；否则会话 `modelConfig.modelName` |
| 厂家 | `vendor.Lookup(model)`（claude→Anthropic，gpt→OpenAI，MiniMax→MiniMax，kimi→Moonshot，gemini→Google，grok/composer/default→Unknown） |
| 质量 | token 字段非 0 → authoritative；全 0 → **degraded**（账本在，用量列是占位） |

**本机合计（2026-08-15，前缀查询，不把上下文快照当账单）：**

| 项 | 值 |
|----|---:|
| type=1 用户泡 | 1,970 |
| 其中 subagent | 143 |
| **真用户回合** | **1,827** |
| type=2 | ~53.5k（会话还在涨） |
| 其中 thinking (`capabilityType=30`) | ~10k |
| **请求** | **43,550**（2026-08-15 22:57 `scan --json` 与 `scripts/sum_cursor.py` 一致） |
| tokenCount 非 0 行 | **0** |
| miss / cache / output | **0** |
| 厂家拆分（请求，token 均为 0） | Unknown 29,840 · MiniMax 6,527 · Anthropic 5,783 · Moonshot 770 · OpenAI 342 · Google 288 |

**明确不算用量（会虚增数百万）：**

- `composerData.promptTokenBreakdown.totalUsedTokens`（126 个会话有；是**当前上下文窗口快照**，类别还标着 `estimatedTokens`）
- `composerData.contextTokensUsed`（149 个会话）
- `bubble.contextWindowStatusAtCreation.tokensUsed`（357 个泡，sum≈34.5M，同样是窗口占用不是请求增量）
- `ItemTable` `aiCodeTracking.dailyStats.*`（行数，不是 token）
- `~/.cursor/ai-tracking/ai-code-tracking.db`：`ai_code_hashes` 有 `requestId`/`model`/`conversationId`，**没有 token 列**；13,234 行里 distinct `requestId` 只有 69，不能当请求次数

**workspaceStorage/\*/state.vscdb：** 本机 34 个。全局 `composerHeaders.workspaceIdentifier.uri.fsPath` 已覆盖 259/267 会话，不必再扫工作区库。

**产品决定：** Cursor 是真实工具源（六列都在）。token 列诚实为 0 + `quality=degraded`。不上云。

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
# Cursor：bubble tokenCount 求和（本机应为 0）+ 请求/回合
# python3 scripts/sum_cursor.py
```

夹具放 `testdata/adapters/<source>/`：从真实文件**脱敏**后的 20–50 行样本（去掉 prompt 正文，保留 usage 与 type）。
