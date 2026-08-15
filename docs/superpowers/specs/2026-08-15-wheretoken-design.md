# whereToken Design Spec

Date: 2026-08-15
Status: 方案 B 已批准；按工具 × 按厂家拆分为一等需求
Companion: [`docs/data-sources.md`](../../data-sources.md) · [`opt.md`](../../../opt.md) · [`docs/superpowers/plans/2026-08-15-wheretoken.md`](../plans/2026-08-15-wheretoken.md) · [`2026-08-15-wheretoken-calendar-design.md`](./2026-08-15-wheretoken-calendar-design.md)（窑墙视觉 + 日历，第 3 轮）

---

## 1. 问题

一个人同时用 Claude Code、Kimi、Codex、OpenCode、Cursor 时，用量散落在家目录不同账本里。厂商 UI 各说各话，单位不统一，缓存往往被藏起来。用户要的不是再一个账单估算器，而是回答：

**我的 token 都花在哪？**

方法：扫描用户根目录下真实存在的 agent 数据根（`~/.claude`、`~/.codex`、`~/.kimi-code`、`~/.opencode` / XDG data、`~/.cursor` 等），**有什么算什么**，归一化成同一套指标。

成功标准（v1 可上线）：

1. 冷启动后 10 秒内（本机当前数据量级）给出**全量合计**，单位 M。
2. 同一套六列同时能按 **工具**（Claude Code / Kimi / Codex / OpenCode）和按 **厂家**（Anthropic / Moonshot / OpenAI / MiniMax / …）拆开；`sum(按工具) == 合计 == sum(按厂家)`。
3. 工具 ≠ 厂家。Claude Code 里跑 MiniMax 时，记在工具「Claude Code」和厂家「MiniMax」，不得记成 Anthropic。
4. 六列指标在每个已适配源上都有定义，缺字段则显式为 `—` 或 `0`，不编造。
5. Kimi / OpenCode 总量与本机再算脚本误差为 0。
6. Codex 不因重复 `token_count` 而翻倍。
7. Claude 标明数据质量，用户回合不含 tool_result。
8. 不上传任何会话；不读取密钥文件。

非目标（v1）：

- 美元估价、订阅 quota 条、leaderboard、多用户账号、云同步。
- 替代 ccusage / tokscale 的 TUI。
- 把 Cursor 云端 CSV 当账本。
- 修改任何 agent 的原始文件。

---

## 2. 同系列位置

工具箱现有四件：PlainList（时间）、Flow（会议）、Untitled（网盘）、docxeditor（文档）。whereToken 是第五件：**个人算力消耗的观测器**。

共同气质：简单、实用、本机可跑、中文工作台。whereToken 的交互更接近 Untitled（本机服务 + Vue），而不是 Flow 的实时媒体或 docxeditor 的文档内核。

---

## 3. 方案比较与选择

| | A. TS CLI | B. Go 内核 + 本机 HTTP + Vue | C. Tauri 桌面 |
|--|-----------|------------------------------|---------------|
| 扫 24MB JSONL / 2.3GB sqlite | 弱 | 强 | 强，但壳更重 |
| 与工具箱一致 | 差 | 与 Untitled 同构 | 与 docxeditor 同构 |
| 核验速度 | 快出表，慢做可视化 | 先数字后页面 | 打包干扰核验 |
| 分发 | npm | 单二进制 `wheretoken` | DMG |

**选择 B（用户 2026-08-15 确认）。** CLI 是同一个二进制：`wheretoken scan --json` 给脚本，`wheretoken serve` 给仪表盘。Tauri 列为 v2。

---

## 4. 指标合同

所有适配器输出同一事件，再聚合。内部永远是 `int64` 个 token。展示层唯一允许除以 `1_000_000`。

### 4.1 事件

```
UsageEvent
  source          string    // 工具：claude | kimi | opencode | codex | …
  vendor          string    // 厂家：anthropic | moonshot | openai | minimax | google | unknown
  source_root     string    // 绝对路径，用于去重同一物理目录
  session_id      string
  request_id      string    // 去重键；没有则生成稳定哈希
  model           string
  provider        string    // 适配器可选，如 OpenCode providerID；供厂家推断
  workspace       string    // 尽量还原真实路径
  timestamp       time      // UTC 存，本地时区展示
  miss            int64     // 未命中输入
  cache_read      int64
  cache_create    int64
  output          int64     // 含 reasoning
  reasoning       int64     // 子集，默认 0；不得再加进 output
  quality         enum      // authoritative | degraded | estimated | absent
```

`vendor` 由归一化层根据 `model` + `provider` 填写，适配器不准手写死成和 `source` 一样。本机反例：Claude Code JSONL 里有 `MiniMax-M3`。

用户回合不一定来自同一条 usage 事件（Claude/Kimi 都是旁路计数），因此另有：

```
TurnEvent { source, session_id, timestamp, workspace }
```

请求次数 = 去重后的 `UsageEvent` 数（一个 request_id 一次）。

厂家推断（大小写不敏感，先匹配先赢）：

| vendor | 命中 |
|--------|------|
| `minimax` | 含 `minimax`，或 `abab` |
| `anthropic` | 含 `claude` |
| `moonshot` | 含 `kimi` / `moonshot`，或模型为 `k3` / `kimi-code/k3` |
| `openai` | 含 `gpt` / `codex` / `chatgpt`，或以 `o1` `o3` `o4` 开头 |
| `google` | 含 `gemini` |
| `unknown` | 其余；UI 仍单列，标签用原始 model 前缀，不计入「丢掉」 |

### 4.1b 两轴拆分（用户锁死）

「额度」在本产品里 = **已经花掉的 token 归属**，不是套餐剩余配额（v1 不做订阅余额）。

任何一屏、任何一份 `scan --json` 必须同时给出：

1. **合计** `all` — 所有工具加总
2. **按工具** `by_source` — Claude Code 花了多少、Kimi 花了多少、…
3. **按厂家** `by_vendor` — Anthropic / Moonshot / OpenAI / MiniMax / …
4. **交叉** `by_source_vendor` — 例如 Claude Code × MiniMax，用来解释「工具和厂家为什么对不上」

不变量：`sum(by_source.total) == all.total == sum(by_vendor.total)`。回合按工具计（厂家轴上的回合可空或按该厂家事件所在会话近似，v1 厂家轴只保证 token 六列里的前四列 + 请求次数；用户回合以工具轴为准）。

### 4.2 聚合

```
total            = miss + cache_read + cache_create + output
cache_hit_rate   = cache_read / (cache_read + miss + cache_create)   // 分母 0 → —
```

「总 token（含缓存读取）」就是 `total`。缓存读取计入总量，这是用户点名的定义，即使它在计费上往往更便宜。

展示：

- `360.11 M`
- `< 0.01 M` 时用四位小数
- 命中率百分比，保留 1 位小数
- 请求、回合用整数，不加 M

切片：全局合计、按工具、按厂家、工具×厂家、按模型、按日、按工作区、按会话。过滤器可组合。默认着陆页必须能同时看见合计、按工具、按厂家，不能只给一个总数。

**按日（第 3 轮升为 P0）：** 日桶、峰值、连烧全部在 Go 里算完，经 `calendar` 字段下发。规则锁在 [`2026-08-15-wheretoken-calendar-design.md`](./2026-08-15-wheretoken-calendar-design.md)。前端不得用事件重算这些数。

### 4.3 质量枚举

| quality | 何时 | UI |
|---------|------|-----|
| authoritative | 源提供最终 usage（Kimi `usage.record`、OpenCode message tokens、Codex 前进的 cumulative） | 无标记 |
| degraded | Claude 等：cache 可信、input/output 可能是占位 | 黄标「输入/输出可能偏低」 |
| estimated | 只能用字符/4 之类（v1 不采用） | v1 不进总表 |
| absent | 检测到工具但没有 token 账本（Cursor P1） | 灰行「已发现，无用量」 |

---

## 5. 架构

```
                 ┌─────────────────────────────────────────┐
  只读磁盘        │  adapters (claude/kimi/opencode/codex/…) │
  ~/.claude …    │  discoverer                             │
                 └──────────────────┬──────────────────────┘
                                    │ UsageEvent / TurnEvent
                                    ▼
                 ┌─────────────────────────────────────────┐
                 │  normalize + dedupe + quality           │
                 │  SQLite WAL cache (~/.wheretoken/cache.db)
                 │  按 (source, request_id) 幂等 upsert     │
                 └──────────────────┬──────────────────────┘
                                    │
                    ┌───────────────┴───────────────┐
                    ▼                               ▼
           wheretoken scan --json            chi HTTP 127.0.0.1
           (脚本/核验)                       /api/summary
                                             all + by_source + by_vendor
                                                    │
                                                    ▼
                                           Vue 3 窑墙仪表盘
                                           calendar + 两轴表
```

### 5.1 进程

一个 Go 二进制：

- `wheretoken scan` 扫一遍，打印 JSON 汇总，退出码 0/1。
- `wheretoken serve [--port 8787]` 先扫（或增量）再提供 UI。默认只绑 `127.0.0.1`。
- `wheretoken sources` 列出探测到的根：已适配 / 未适配 / 跳过原因。

缓存目录：`~/.wheretoken/`（可 `WHERETOKEN_HOME` 覆盖）。缓存损坏则删库重扫，源文件不动。

### 5.2 增量

每个源文件记 `(path, size, mtime, inode)`。未变则跳过。Claude 会原地改写 jsonl（compact），mtime 变则重解析该文件；若实现「删除行仍计数」需要额外 message cache——**v1 不做**，以磁盘当前内容为准（诚实：compact 后总量可能下降）。在 UI 用一句话说明。

### 5.3 并发

按文件并行，限制 worker（默认 `GOMAXPROCS`）。单个 jsonl 内部串行（要做 cumulative delta）。sqlite 源每库一个只读连接。

### 5.4 错误

- 无权限 / 文件被锁：该文件记入 `scan_errors`，其它源继续。
- JSON 坏行：跳过该行，计数 `skipped_lines`。
- 未知 schema：进入未适配列表，不计总量。
- 端口占用：换端口并打印（学 Untitled）。

---

## 6. 适配器接口

```
type Adapter interface {
    ID() string
    Discover(home HomeLayout) []SourceRoot   // 可返回空
    Parse(root SourceRoot, emit func(Event)) error
}
```

`HomeLayout` 提供 `DotDir(name)`、`XDGData(name)`、`AppSupport(name)`，便于测时注入 fake home。

每个适配器一个包，带 `testdata` 脱敏夹具。新工具 = 新包 + 注册，不改聚合器。

P0 解析规则见 [`docs/data-sources.md`](../../data-sources.md)，这里只锁合同：

- Claude：requestId 取 max；user 排除 tool_result；不读 settings.json。
- Kimi：只信 `usage.record`；回合用 `turn.prompt` + `origin.kind==user`。
- OpenCode：message.tokens，不与 session 列、step-finish 双计；`mode=ro`。
- Codex：cumulative 前进才计 delta；流式读。

---

## 7. 信息架构（仪表盘）

一页为主，避免工具箱里再做一个「要学习的后台」。

第 3 轮把「时间」从 ECharts 日折线提升为 **窑墙（贡献日历力学）**，并且它是英雄区，不是表下面的附件。完整视觉与格子规则见日历规格。本文件只锁信息优先级：

**墙（按日消耗）= 峰值 / 连烧 = 两轴切换 > 合计六列复核 > 工具/厂家表 > 交叉 > 模型下钻。**

1. **顶栏（瘦）：** 产品名、最后扫描时间、刷新。
2. **窑墙 + 铸造数字：** 53 周 × 7 日方格；峰值（日期 + M）、当前连烧、最长连烧。轴切换：合计 / 单工具 / 单厂家，墙和三块数字一起换序列。
3. **合计六列：** 总 token、命中率、未命中、输出、请求、用户回合。仍用后端 `*_m`。墙已经是主角，这里不再做成六张卡片。
4. **按工具表：** 同一六列。一行 Claude Code、一行 Kimi、一行 Codex、一行 OpenCode。行可切到该工具的墙。
5. **按厂家表：** 同一 token 四列 + 请求。行可切到该厂家的墙。
6. **交叉（可折叠）：** 工具 × 厂家，解释混用（Claude Code 调 MiniMax）。
7. **下钻（仍非本轮）：** 模型、工作区、会话。会话表不展示 prompt。
8. **质量条：** 若 Claude 占比高且 degraded，常驻一句人话。扫描错误进 `errors[]`，**不**把失败日画成用量零。

视觉（第 3 轮锁死，不再「实现时再定」）：暗色窑墙，焦褐→白热，Chiron Hei HK + Big Shoulders Display + Martian Mono。禁止新闻纸账本、GitHub 绿、浅色 admin 卡片。细节在日历规格 §3。

CLI 汇总必须与页面 KPI **同一函数**算出，禁止两套公式。日桶 / 峰值 / 连烧同样只在 `internal/metric` 算一次。

---

## 8. 仓库结构（实现时创建，现在只锁定名字）

```
whereToken/
  README.md
  opt.md
  docs/
    data-sources.md
    superpowers/specs/…
    superpowers/plans/…          # 规格批准后
  cmd/wheretoken/main.go
  internal/
    adapter/{claude,kimi,opencode,codex,discover}/
    event/                       # UsageEvent / TurnEvent
    vendor/                      # model+provider → 厂家
    normalize/
    store/                       # sqlite cache
    metric/                      # total / hit rate / format M / 两轴聚合
    httpapi/
  testdata/adapters/…
  web/                           # Vue 3 + Vite
    src/{app,features,shared}/
  scripts/verify-local.sh        # 对本机实盘跑对照（不提交用户数据）
```

Go module 名：`github.com/rainhuang0220/whereToken`。
前端 workspace 可后加；v1 不必 monorepo 工具化到 PlainList 那种程度。

---

## 9. 隐私

- 只读源；whereToken 自己的库只存聚合事件（无 prompt 正文）。
- 绑定 127.0.0.1。
- 日志默认不打印 message 内容。
- 夹具提交前人工检查无 prompt、无 token 密钥。
- `.gitignore` 忽略 `.env`、`auth.json`、`data/`、`*.db`。

---

## 10. 测试与核验门

每一轮「可以提交版本」必须同时满足：

| 门 | 命令（实现后） | 通过标准 |
|----|----------------|----------|
| 单元 | `go test ./...` | 夹具：Kimi 12 行求和、Codex 重复 snapshot 不双计、Claude tool_result 不计回合、OpenCode 不双计 |
| 格式化 | `go test ./internal/metric` | 1_000_000 → `1.00 M`；0 分母命中率 → `—` |
| 类型 | `vue-tsc` / vitest（有 web 之后） | KPI 组件吃同一 JSON shape |
| 实盘（开发机） | `scripts/verify-local.sh` | Kimi/OpenCode 与独立 python 对照 0 误差；Claude/Codex 有断言区间与质量旗标 |
| 安全抽查 | grep 扫描器打开的路径 | 无 `settings.json`、无 `auth.json`、无 credential 表 |

实盘脚本**不得**把家目录 jsonl 拷进 git。

---

## 11. 实现阶段（规格级路线，不是 task 清单）

批准本规格后，另写 `docs/superpowers/plans/2026-08-15-wheretoken.md`（TDD、按任务提交）。阶段如下，每阶段一次版本标签：

**v0（本提交）** 规格。

**v0.1 度量核**  
`metric` + `vendor`：公式、M 格式化、合计 / 按工具 / 按厂家聚合与守恒测试。纯函数。无 IO。

**v0.2 Kimi 适配器**  
夹具来自脱敏 `usage.record`。`scan --source kimi --json` 与夹具一致。本机实盘对照 330.04 M ± 0（允许用户在实现日用量增长，脚本改为「当场再算」而不是钉死 330）。

**v0.3 OpenCode**  
只读 sqlite 夹具（从空 schema + 插入伪造 tokens，不含真实账号表）。

**v0.4 Codex**  
重复 snapshot 夹具必须先红后绿。

**v0.5 Claude**  
requestId max、tool_result、degraded 旗标。

**v0.6 发现器 + 多源汇总**  
`wheretoken sources` / `scan`。

**v0.7 HTTP API + Vue KPI 页**  
合计六块 + 按工具表 + 按厂家表与 `scan --json` 同一 payload。

**v0.8 窑墙（原「日折线」改 P0）**  
`calendar` 进 JSON；Vue 以 53×7 墙为英雄区；峰值 + 当前/最长连烧；轴切换重绘。计划：[`docs/superpowers/plans/2026-08-15-wheretoken-calendar.md`](../plans/2026-08-15-wheretoken-calendar.md)。

**v0.9 下钻（模型 / 工作区 / 会话）+ 质量条**  
认为可日常自用后再打版本标签。

**v1.0** P1 发现器点亮本机其它根；Cursor 诚实空态；README 安装说明；再考虑是否反写其它仓库的 toolkit 表。

---

## 12. 风险

| 风险 | 缓解 |
|------|------|
| Claude JSONL 系统性偏低 | 质量旗标；不把 Claude 数当计费真值 |
| Codex 大文件 | 流式；单测用小夹具覆盖逻辑 |
| Cursor 2.3GB DB | v1 不全表扫 |
| 源格式升级 | 适配器版本探测；未知字段忽略；解析失败进 errors |
| 把密钥扫进缓存 | 路径黑名单 + code review 门 |
| 与 tokscale 功能攀比 | YAGNI：六列 + 下钻，不做社交 |

---

## 13. 规格自检

- 无 TBD/TODO 占位。第 0 轮三问已在 `opt.md` 关闭。第 3 轮周首/时区/分桶/连烧收口已在日历规格锁死。
- 公式在 §4 与 opt.md 0.5、README 一致。
- P0 四源均有本机证据（data-sources.md）。
- 范围是一个产品、一条扫描管道，不需要拆成多个仓库。
- 「用户回合」不会有两种解释：Claude 排除 tool_result，Kimi 用 `origin.kind==user`。
- 「额度」= 已消耗归属，不是套餐剩余。工具轴与厂家轴必须同时存在，且合计守恒。

---

## 14. 下一步

方案 B 已批准。v0.7 内核已落地。第 3 轮（窑墙）按 [`2026-08-15-wheretoken-calendar-design.md`](./2026-08-15-wheretoken-calendar-design.md) 与 [`docs/superpowers/plans/2026-08-15-wheretoken-calendar.md`](../plans/2026-08-15-wheretoken-calendar.md) 执行。
