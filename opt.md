# whereToken 决策日志

供复盘。只记**已经拍板或明确留给用户**的决定，不记流水账。
格式：日期 · 主题 · 选项 · 选择 · 理由 · 后果。

---

# 第 8 轮：昼 / 墨两套新釉（2026-08-15）

触发：用户要再加两套釉——纯白黑字现代化蓝，以及无彩黑白；日历仍是平的 2D 砖。

## 8.1 昼 day

- **选项：** 昼 / 海 / 霁。
- **选择：** id `day`，印记 **昼**。
- **不选：** 海（偏青绿，容易滑向 neon cyan）；霁（晴空蓝，和瓷的青花釉阶抢戏）。
- **场：** `--void #ffffff` 纯白，`--bone` 近黑。强调色是 Vercel/Linear 附近的产品蓝 `#1d4ed8` / `#2f6fed`，不是婴儿蓝 UI kit，也不是窑的焦橙。
- **砖：** 空砖浅灰蓝 `#d4dce8`（比纸底深，白叠白能看见）→ 淡蓝 → 天蓝 → 强蓝 → 深亮蓝。浅色釉的 `ember-4` 走深色，才能当标题。

## 8.2 墨 ink

- **选项：** 墨 / 玄；页面白底黑字或黑底白字。
- **选择：** id `ink`，印记 **墨**。白纸 + 黑字。砖 **白灰 → 中灰 → 近黑**。
- **不选：** 玄（夜空黑底会把最热一档的黑砖吃掉，和「日历从白灰到黑」相反）；黑底白字同理。
- **无彩：** 全部 token 的 R=G=B。空砖 `--clay` 深于 `--void`，墙缝 `--mortar` 另档，避免白叠白。`warn` 也是深灰，不偷渡红。
- **和青墨：** 切换印记仍是「青」与「墨」两个字；青墨是松烟上的青釉，墨是宣纸上的灰阶。

## 8.3 切换条

- **选择：** 右上角八个字 窑苔瓷绛青霜昼墨。按钮略收（1.38rem / gap 0.1rem），八枚仍是一排，不改成设置页。
- **仍：** `localStorage['wheretoken.theme']`。`index.html` 的 boot `allowed` 必须与 `THEME_IDS` 同步，否则新釉首屏会闪回窑。

---

# 第 7 轮：釉色包 + 去掉「消耗」水印（2026-08-15）

触发：用户要可切换主题（说「插件」），窑墙必须跟着重漆；巨大「消耗」水印必须删；README 要能复制粘贴的第一次跑 / 再开一次。

## 7.1 水印

- **选择：** 删掉 `.watermark`「消耗」。窑的隐喻留在釉色和字体，不靠巨型汉字撑场。
- **理由：** 用户原话太 tacky，mandatory delete。

## 7.2 主题是一份 manifest，不是散落的 hex

- **选择：** `web/src/themes/` 釉色包。`manifest.ts` 是唯一调色盘；CSS 只写 `var(--void)` / `var(--ember-*)` / `color-mix(...)`。Vite 把 `themeStylesheet()` 打进 `virtual:wheretoken-themes.css`。`document.documentElement[data-theme]` + `localStorage['wheretoken.theme']`。
- **不选：** 每个 Vue 文件里写死 hex；半切换（标题换了砖还是窑橙）。
- **切换 UI：** 右上角印记切釉，不是设置页。第 7 轮六个字 窑苔瓷绛青霜；第 8 轮起八个字 窑苔瓷绛青霜昼墨。

## 7.3 六套釉

| id | 名 | 为什么在 whereToken 成立 |
|----|----|--------------------------|
| kiln | **窑** | 默认。炉膛黑 + 焦褐到白热。现有产品皮肤，抽出变量，不改味道。 |
| moss | **苔** | 淡绿纸底 + 草绿加厚。空砖仍是可辨的冷苔，不是 Bootstrap 绿按钮。浅色场上「烧得越狠砖越深」，和 GitHub 绿秀无关，是苔藓长厚。 |
| porcelain | **瓷** | 高岭土暖白为主，青釉蓝和墨作发丝线。不是 Twitter 白底蓝链。砖从素坯走到青花再到近黑。 |
| jiang | **绛** | 粉黑：木炭底上的热玫瑰红，不是 Barbie 浅粉。账本里的「烧」换成绛色窑火。 |
| qingmo | **青墨** | 松烟纸 + 青釉。token 是一笔墨越写越亮的青。比窑更冷、比霜更有铜绿。 |
| frost | **霜碳** | 碳灰墙，只有最热一档结霜白蓝。冷账本：大部分是炭，峰值才起霜。 |

- **对比：** Vitest 钉 `bone` / `ash` / `ember-4` / `warn` 对 `void` ≥ 4.5；空砖 `clay` 必须异于 `mortar` 和页面底。浅色釉的 `ember-4` 走深色（苔藓底、墨）才能当标题色；深色釉的 `ember-4` 走高光。

## 7.4 serve 与 dist

- **选择：** `http.FileServer` 每次请求读 `web/dist`。改前端编 dist 后硬刷新即可；不必为 CSS 杀 8787。README 仍给出 `lsof -tiTCP:8787 | xargs kill` 的完整再开一次命令，因为用户要复制即用。

---

# 第 6 轮：Cursor 账号用量 API（2026-08-15 23:00）

触发：用户明确推翻「永远不用 Cursor 登录 / cursor.com CSV」。本机 `tokenCount` 全 0 不能当账。人正在这台 Mac 的 Cursor 里登录。

## 6.1 新隐私边界

- **旧：** 不上 Cursor 云、不读登录 Cookie、不拉 CSV。
- **新（用户原话）：** 可以用 **Cursor 已经登录的同一个账号**，走 **Cursor 自己的 API/界面**，拉 **这个用户** 的用量。
- **仍禁止：** 上传 chats；打其它厂商云；把 token / JWT 写进 git、夹具、日志、Authorization 打印。

## 6.2 接口与登录态

- **选择：** Cursor 桌面端已在调用的 `https://api2.cursor.sh` `aiserver.v1.DashboardService`（Connect JSON）。优先 `GetFilteredUsageEvents`（有 timestamp + cache 四列），空则回退 `GetAggregatedUsageEvents`（按模型合计，无逐日时间戳）。
- **登录态：** 只读 `ItemTable` 键 `cursorAuth/accessToken` 作 Bearer；401 才读 `cursorAuth/refreshToken` 内存刷新。测试 `httptest` + 假 JWT `test-token`。
- **不选：** 刮 HTML；把 tokscale CSV 当主路径（仅当账号 UI 自己用导出时才考虑，本轮不需要）。

## 6.3 映射与守恒

- token 四列 + 窑墙日期来自账号 API；requests / user_turns 仍来自本机 bubble。API 行 `SkipRequest`，避免请求数双计。有 API token 时丢掉 bubble `tokenCount`（本机本就是 0）。
- 质量：API 有 token → `authoritative`。无登录态 → `errors[]` 含 `cursor: 未找到本机登录态`，token 列 degraded。
- **后果：** 窑墙在 Cursor 轴上会按 API 时间戳点亮（仅 aggregations 时诚实记成扫描当日）。

---

# 第 5 轮：Cursor 必须是真源（2026-08-15）

触发：用户在 Cursor 里用了 whereToken，却看到 Cursor「已发现，无用量」，觉得账本把一年的 Cursor 使用抹掉了。

## 5.1 为什么会空

- **当时的选择：** P1 记下 `ai-code-tracking.db` 无 token 列、`state.vscdb` 2.3GB 不敢扫；后来用 `quality=absent` 表示「检测到 ~/.cursor」。
- **为什么产品错：** 人正在 Cursor 里聊天，本地账本是 `Application Support/Cursor/.../state.vscdb`，不是 `~/.cursor/ai-tracking`。缺席行等于说「你没在用 Cursor」。
- **不选：** 用登录 Cookie 拉 cursor.com CSV（tokentop/tokscale 的路）；用 `text.length/4` 或把 `promptTokenBreakdown.totalUsedTokens` 加总（那是窗口快照，会虚增数百万）。

## 5.2 本机字段（2026-08-15）

- `cursorDiskKV` 前缀 `composerData:%`（328）+ `bubbleId:%`（~5.5 万）。`usageData` 全是空对象。`tokenCount.inputTokens/outputTokens` **存在且全为 0**。`usageUuid` 全无。
- 真信号：用户泡 `type=1`、助手/工具泡 `type=2`、`capabilityType`（30=thinking）、用户泡 `modelInfo.modelName`、`composerHeaders.isSubagent` + 工作区路径。
- **选择：** 请求 = type 2 且非 thinking；回合 = type 1 且非 subagent；token 列照字段填（本机 0）+ `quality=degraded`。厂家走已有 `vendor.Lookup`。
- **后果：** 六列都在；窑墙按 token 点亮，所以 Cursor 单独切轴时墙仍是冷的（0 token，不是没扫到）。合计守恒不变。

---

# 第 0 轮：规格（2026-08-15）

触发：用户要求先写详细规划文档，再实现；同系列对照 PlainList / Flow / Untitled / docxeditor；过程决定写入本文件；重要处询问用户；其余按最优解执行；GitHub 建仓；每轮核验后提交一版；提交不含 AI coauthor。

## 0.1 本轮产出是文档，不是代码

- **选择：** v0 只提交规格、数据源实测和本日志。
- **理由：** 用户原话「一份好的文档是工作的开始」。指标定义和 Claude/Codex 解析陷阱比 UI 更先决定对错。
- **后果：** 仓库暂时不能 `go run` / `npm run dev`。下一版必须是可核验的扫描内核，而不是空壳页面。

## 0.2 产品形态（待用户确认后锁死实现计划）

三种候选：

| 方案 | 形态 | 优点 | 缺点 |
|------|------|------|------|
| A | TypeScript CLI（ccusage 路线） | 生态熟、出表快 | 与工具箱视觉产品不一致；大 JSONL / 2.3GB Cursor DB 吃力 |
| B | **Go 扫描内核 + 本地 HTTP + Vue 仪表盘** | 与 Untitled 同构；单二进制；并发扫盘；浏览器即 UI | 要自己维护适配器 |
| C | Tauri 桌面壳（docxeditor 路线） | 原生窗口、权限模型清晰 | v1 把核验重心从「数对不对」挪到打包，过早 |

- **选择：B（用户 2026-08-15 确认）。**
- **明确不做：** Tokscale 式 TUI 排行榜、社交 submit、云同步。
- **状态：** 已锁死。

## 0.3 和竞品的关系

对照过 [ccusage](https://github.com/ccusage/ccusage)（CLI 报表）和 [tokscale](https://github.com/junhoyeo/tokscale)（Rust TUI + 多源 + 排行榜）。

- **选择：** 借鉴它们的**路径与解析陷阱**，不复用它们的产品目标。
- **whereToken 差异：** 工具箱里的视觉仪表盘；六列指标（含命中率）一等公民；单位 M；数据质量旗标；无账号无上云。
- **不 fork：** 许可证、架构、社交功能都不对齐；解析逻辑按本机实测重写，用夹具锁行为。

## 0.4 技术栈

| 层 | 选择 | 不选 | 理由 |
|----|------|------|------|
| 扫描 / API | Go 1.25 + chi + SQLite WAL（`modernc.org/sqlite`） | Node 扫盘、Python、内嵌 ccusage | 与 Untitled 一致；纯 Go SQLite 无 CGO；大文件流式 |
| UI | Vue 3 + Vite + TypeScript + Pinia + ECharts | React（docxeditor）、纯 CLI | 工具箱四件里三件是 Vue |
| 测试 | Go `testing` + testdata 夹具；Vitest 只测格式化/聚合展示 | 先写页面再补测试 | 解析对错必须用黄金文件钉死 |
| 打包 v1 | `wheretoken serve` 打开本机端口 | 先做 DMG/Tauri | 先核验数字 |
| 桌面 v2 | 可选 Tauri 或走 PlainList 的 electron 打包经验 | v1 不做 | 延后 |

## 0.5 指标公式（锁死）

内部 `int64` token。展示 `value / 1_000_000`，后缀 `M`，默认 2 位小数，小于 `0.01 M` 时升到 4 位。

```
total      = miss + cache_read + cache_create + output
hit_rate   = cache_read / (cache_read + miss + cache_create)   // 分母为 0 则显示 —
miss       = 未命中输入（各源字段见 data-sources.md）
output     = 输出（含 reasoning；若源单列 reasoning 则另给一列但不从 output 再加一次）
requests   = 去重后的模型调用次数
user_turns = 真人回合，排除 tool_result / 工具回灌
```

**不做 v1：** 美元估价、订阅额度条、按定价表算钱。用户要的是 token 去向，不是账单仿真。可在 P2 加「估价」开关，默认关。

## 0.6 数据源优先级

本机 2026-08-15 实测后排序：

| 优先级 | 源 | 本机是否有可解析用量 | 备注 |
|--------|----|----------------------|------|
| P0 | Claude Code `~/.claude/projects/**/*.jsonl` | 有。原始合计约 360.11 M | 必须 requestId 去重；input/output 有占位 bug，要质量旗标 |
| P0 | Kimi Code `~/.kimi-code/sessions/**/wire.jsonl` 的 `usage.record` | 有。约 330.04 M，命中率 98.81% | 字段干净，作为第一个黄金夹具 |
| P0 | OpenCode `~/.local/share/opencode/opencode.db` | 有。session 合计与 message.tokens 一致，约 1.74 M | 只读；禁止读 `credential` / `account` |
| P0 | Codex `~/.codex/sessions/**/rollout-*.jsonl` 的 `token_count` | 有 29 个 rollout | 用累计 `total_token_usage` 的增量，禁止对重复 snapshot 求和 |
| P0 | Cursor `state.vscdb` 键前缀 + 本机登录态调 Cursor DashboardService | 请求/回合在本地；token 走账号 API | 第 6 轮用户授权。禁止把 JWT 写入 git；禁止上下文快照当用量 |
| P1 | 发现器：家目录 `.*` + macOS Application Support 里能识别的其它 agent | 目录存在但用量字段未核完 | MiniMax / Copilot / Trae / OpenClaw 等 |
| P2 | Gemini / Qwen / Amp / Factory / Goose / Hermes / pi | 本机无对应家目录 | 适配器接口先留好，有目录再点亮 |

**不扫：** `auth.json`、Keychain、浏览器 Cookie、其它厂商账单 API。Cursor 账号用量是第 6 轮用户授权的例外。

## 0.7 Claude「用户回合」定义（锁死）

本机 `type=user` 共 3394 行，其中 **3289 是 `tool_result`**，真人口头回合约 **105**。

- **选择：** 用户回合 = `type==user` 且 content 不是 tool_result。
- **理由：** 若把 tool 回灌算成回合，数字会大一个数量级，和用户直觉相反。

## 0.8 隐私与安全（锁死）

- 扫描只读。任何适配器不得 `UPDATE`/`DELETE` 源文件。
- SQLite 源用 `mode=ro`。
- 仪表盘默认只展示聚合与会话元数据（源、模型、时间、工作区路径、token）。默认不展示 prompt 正文。
- 绑定 `127.0.0.1`，不默认对局域网开放。
- Cursor 的 2.3GB `state.vscdb` 只允许**有键前缀的查询**，禁止全表 dump 进内存。`ItemTable` 只按键名读登录态，永不打印值。
- 第 6 轮起：可用本机 Cursor 登录态打 `api2.cursor.sh` DashboardService，只拉当前用户用量。

## 0.9 仓库与提交习惯

- **选择：** 公开 GitHub 仓库 `rainhuang0220/whereToken`（与 PlainList/Flow/docxeditor 一致）。若用户要改私有，下一轮改 visibility。
- 每一轮「核验通过」才提交并 push。v0 的核验 = 规格自检（无占位、公式自洽、数据源与本机一致）。
- `git commit` **禁止** `Co-authored-by:`，禁止把 Cursor/Claude/Codex 等写进 contributors。Cursor 会在 `git commit -m` 之后注入 trailer；本仓库用 `git commit-tree` 写提交来绕开。
- 不改 git config；用已有 `user.name=rainhuang0220`。

## 0.10 许可证

- **暂缓。** 实现第一笔代码前再选。倾向 MIT（Untitled/tokscale 同款），但未问用户。
- **状态：** 可在实现开始时确认。

## 0.11 同系列 README 反写

- **本轮不做。** 不改 PlainList/Flow/Untitled/docxeditor 的 toolkit 表。
- 等 whereToken 有可运行 v1 再提 PR 把第五行加进去，避免四个仓库出现「规划中但点进去是空的」。

## 0.12 技能与外部资料

本轮读过并遵循：`using-superpowers`、`brainstorming`、`writing-plans`（实现计划要等规格批准后才写完整 task list）。
参考过 ccusage / tokscale / tokenuse 的路径表和 Codex 去重 issue，以及本机目录实测。
未安装新 skill：规划阶段不需要 frontend-design；UI 开工时再读。

---

# 待用户决策（第 0 轮）— 已关闭

1. 方案 B：用户 2026-08-15 确认「可以按照 B」。
2. P0 范围：用户强调必须能区分 Claude Code / Kimi / 各厂家；未否决 Cursor 放 P1。维持 P0 四源。
3. 仓库 public：已建且已推送，用户未要求改私有。

---

# 第 1 轮：两轴拆分（2026-08-15）

触发：用户确认 B，并补充「既要总的，又要区分哪些是 Claude Code、哪些是 Kimi、分别是哪些厂家」。

## 1.1 「额度」怎么理解

- **选择：** 额度 = **已经花掉的 token 归属**，不是套餐还剩多少。
- **理由：** 原需求是追踪 token 花在哪；剩余配额要打厂商 API，与「只读本地、不上云」冲突。
- **后果：** UI 不出现「本月还剩」。若用户其实要订阅余额，下一轮再开，不塞进 v1。

## 1.2 两轴，不是一轴

- **工具 source：** Claude Code / Kimi Code / Codex / OpenCode。回答「哪个客户端烧掉的」。
- **厂家 vendor：** Anthropic / Moonshot / OpenAI / MiniMax / Google / unknown。回答「哪家模型烧掉的」。
- **交叉：** 本机 Claude Code 日志里已有 `MiniMax-M3`。若把工具等同厂家，MiniMax 用量会错记到 Anthropic。
- **不变量：** `sum(by_source) == all == sum(by_vendor)`。

## 1.3 用户回合只挂工具轴

厂家轴 v1 保证 token 四列 + 请求次数。回合按工具计。避免把一次 Kimi 开口拆到多个厂家。

## 1.4 实现计划

- 规格批准，开始写 `docs/superpowers/plans/2026-08-15-wheretoken.md`。
- 许可证：第一笔代码用 MIT（此前倾向，用户未否决）。
## 1.5 相对规格的两处收缩

- HTTP 用 Go 标准库，不引入 chi，直到路由变复杂。
- v1 先每次全量扫描，不写 `~/.wheretoken/cache.db`。本机四源体量下足够；扫盘超过 10s 再加缓存。

---

# 第 4 轮：空白窑墙 + 下钻可上线（2026-08-15）

触发：用户截图窑墙是黑洞（只有星期标签和图例），峰值 0.00 M、连烧 0 天；同时 KPI 738.89 M 和工具/厂家表正常。独立 `scan --json` 当天有 12 个点亮日、峰值 244.95 M。

## 4.1 根因

- **选择：** 不是 CSS 把 371 块砖画成与炉膛同色那么简单。运行中的 `wheretoken serve`（:8787）是**旧进程**：`GET /api/summary` 的 JSON **没有 `calendar` 键**。当前源码的 `scan --json` 有 calendar。Vue 在 `!payload.calendar` 时走 `emptySeries` 且 `cells=[]`，于是墙高度为 0、铸造数字全是零；KPI 走的是一直都在的 `all` / `by_source`。
- **不选：** 只怪黏土色太暗。12 个白热砖如果 DOM 里有，不可能完全看不见。截图「一块砖都没有」+ 峰值 `0.00 M` 对得上 `emptySeries` + `cells=[]`。
- **后果：** 前端即使 API 缺 calendar 也必须铺 53×7 冷黏土，避免再出现空洞。空砖加深黏土色、内描边、砂浆底，不再从 opacity 0 开场。`serve` 必须重启才能拿到带 calendar 的 JSON；`web/dist` 仍 gitignore，改 UI 要 `npm run build`。

## 4.2 下钻

- **选择：** 后端预聚合 `drill.all` / `drill.by_source[id]` / `drill.by_vendor[id]`（模型、工作区、会话），前端切轴只换表，不把 token 再加一遍。
- **会话：** 有 `session_id` 用它，否则退回 `request_id`。不展示 prompt。
- **Cursor：** `state.vscdb` 键前缀解析为真源。无 vscdb 时才 `quality=absent`。

## 4.3 相对规格

- 父规格 §7 把下钻标成「仍非本轮」；本轮提前做完，作为 v0.9 一并上线。
- 窑墙空砖对比度高于日历规格里的 `--clay: #14100c`（现 `--clay: #3a2c22`），否则 53 周稀疏点亮时墙仍像黑洞。

---

# 第 2 轮：实现 Task 4–12（2026-08-15）

触发：规格与度量核已落地，按计划从适配器接口做到 `scan --json` + localhost Vue。

## 2.1 Codex 超长 JSONL

- **选项：** 维持计划「Scanner 至少 10 MiB」 / 按本机最长行加大缓冲区 / `ReadBytes` 无上限逐行。
- **选择：** Codex 用 `bufio.Reader.ReadBytes('\n')`。
- **理由：** 本机 `rollout-*.jsonl` 最长一行约 24.6 MB，10 MiB Scanner 会把该文件记进 `errors` 并少计用量。
- **后果：** 单行仍会进内存；禁止 `ReadFile` 整文件。Claude / Kimi 本机最长行低于 10 MiB，仍用 Scanner。

## 2.2 仪表盘气质

- **选择：** 新闻纸账本（ZCOOL XiaoWei + Courier Prime），浅色默认、跟随系统深色；KPI + 按工具表 + 按厂家表吃同一 `/api/summary`。
- **不选：** 极光渐变、在前端 `/ 1e6` 重算合计。
- **后果：** `web/src/format.ts` 的 `columnsFrom` 只转发后端已格式化的 `*_m` / `hit_rate_text`。
- **第 3 轮推翻：** 用户认为新闻纸账本与工具箱同族、太泛。视觉按日历规格整页替换，本条只作为「做过什么」留档，不再约束实现。

---

# 第 3 轮：窑墙日历（2026-08-15）

触发：用户嫌前端和每个项目长一样；要求上网/GitHub 偷手艺做独特 UI；并加 GitHub 贡献图力学的日格墙，强度 = 日 token，跟合计/工具/厂家三轴走；先写更完整规格再改产品。

## 3.1 视觉研究方向

看过（借力学，不借皮肤）：

- GitHub contribution graph：周列、星期行、5 档、空 vs 有、tooltip、`grid-auto-flow: column`。周首周日、UTC、primer 绿 — 不采用。
- GitLab：可周一为周首；蓝绝对阈值 — 只借周一，不借蓝。
- WakaTime / wakafetch：热力表示花掉的时间。
- tokscale TUI Stats + 其 2D/3D web：墙是一等视图、可按源过滤。不借 Primer、3D、排行榜。
- Linear cycle / Stripe Sigma：密度与数字优先，不是 admin 卡片。
- CSS Grid 重绘贡献图的若干 gist/文章：确认列流向，不贴它们的绿 CSS。
- 本仓库上一版 Vue：奶油纸 + ZCOOL XiaoWei + Courier Prime + KPI 栅格 + ledger 表 — **整页替换**。

未装 Syncfusion/Mapbox 热力图 skill：那是 React/地图组件，跟 Vue 窑墙无关。frontend-design + data-visualization 已读。

## 3.2 隐喻

- **选择：** 窑墙（token = 烧掉的热，一天一块砖）。
- **不选：** GitHub 绿贡献秀、新闻纸账本、SaaS analytics、tokscale 3D 砖。
- **理由：** 产品问的是「花在哪」，不是「我贡献了多少」。暗色炉膛和工具箱浅色实用页立刻分开。

## 3.3 周首、时区、今天为 0 的连烧

- **周首：星期一。** 中国默认。不做设置。
- **日界：扫描进程本地时区**（测试钉 `Asia/Shanghai`）。不用 GitHub 的 UTC。
- **当前连烧：** 今天有用量才含今天；否则从昨天往回数连续 `total>0`。今天昨天都是 0 → 0。
- **最长连烧：** 该序列最早非零日到今天，空日打断，不限 53 周窗。

## 3.4 分桶

- **选择：** 每个过滤序列自己的非零日 raw total 四分位（nearest-rank），4 档热度 + 空档。
- **不选：** 全局 max/4（会洗掉 OpenCode）；log(max)/4（四分位已按日分配对比）。
- **相等特例：** 所有非零日 total 相同 → 一律 level 2，避免墙看起来没烧过。

## 3.5 API 形状

- **选择：** `calendar: { week_start, timezone, window_from, window_to, all, by_source, by_vendor }`，每条 Series 含稀疏 `days[]`（只 total>0）和预计算 `stats`（peak + 两段 streak）以及每日本地 `level`。
- **不选：** 只给一张宽表让前端 groupby；不选 ECharts 在浏览器里桶。
- **后果：** 切 tab 是查表。守恒：`sum(calendar.all.days.total) == all.total`。

## 3.6 前端技术

- **选择：** 手写 CSS Grid 墙。卸掉分享条 ECharts。字体 Big Shoulders Display / Chiron Hei HK / Martian Mono。只暗色。
- **不选：** `vue3-activity-calendar`、ECharts calendar、跟随系统浅色。

## 3.7 规格与计划

- 日历完整规格：`docs/superpowers/specs/2026-08-15-wheretoken-calendar-design.md`
- 父规格 §7 / §11 改为墙是 P0
- 实现计划：`docs/superpowers/plans/2026-08-15-wheretoken-calendar.md`


## 2.3 SQLite 驱动

- **选择：** `modernc.org/sqlite`（计划锁定）。`go get` 把 `go 1.25` 写成 `go 1.25.0`。
- **不选：** CGO `mattn/go-sqlite3`。
- **后果：** OpenCode 只 `SELECT session_id, data FROM message`；生产 SQL 不含 account / credential 字样。
