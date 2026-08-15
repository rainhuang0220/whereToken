# whereToken

> 你的 token 都花在哪。
> 扫描本机 coding agent 目录，把用量摊开：先看合计，再按工具（Claude Code / Kimi / …）和厂家（Anthropic / Moonshot / …）拆开。单位是 M。

来自简单、实用、高效的协作工具箱。

### 同系列 toolkit

| 工具 | 状态 | 简介 |
|------|------|------|
| [PlainList](https://github.com/rainhuang0220/PlainList) | 🔨 进行中 | 计划清单 |
| [Flow](https://github.com/rainhuang0220/Flow) | 🔨 进行中 | 会议 |
| [Untitled](https://github.com/rainhuang0220/Untitled) | 🔨 进行中 | 网盘 |
| [docxEditor](https://github.com/rainhuang0220/docxeditor) | 🔨 进行中 | 文档编辑器 |
| <u>***whereToken***</u> | 🔨 进行中 | Token 用量追踪 |

---

## 这是什么

whereToken 是一个**本机优先**的 token 用量观测器。默认读你已经写在磁盘上的会话账本：

`~/.claude` · `~/.codex` · `~/.kimi-code` · `~/.local/share/opencode` · Cursor 的 `state.vscdb`。Cursor 的 **token 四列**在用户授权后走 Cursor 自己的账号用量接口（本机登录态，不上传 chats）。

**有什么算什么。** 适配器按目录探测；扫到的才进表，扫不到的不假装有数。

打开仪表盘，第一眼是一堵 **窑墙**：53 周 × 7 天，一天一块砖。没花过的是冷黏土（仍要看得见），烧过的从焦褐到白热。墙右边是峰值和连烧。切「合计 / 工具 / 厂家」，墙和数字一起换序列——日桶、峰值、连烧都在 Go 里算完，浏览器只渲染。

同一套六列会出现三次：**合计**、**按工具**、**按厂家**（Claude Code 里跑 MiniMax 时，工具记 Claude Code、厂家记 MiniMax）。下面还有模型 / 工作区 / 会话下钻。token 一律 **M = 百万**：

| 指标 | 含义 |
|------|------|
| 总 token（含缓存读取） | miss + cache read + cache create + output |
| 输入缓存命中率 | cache read / (cache read + miss + cache create) |
| 未命中 token | 未走缓存的输入（miss） |
| 输出 token | 模型写出（含 reasoning，若源提供则同时单列） |
| 请求次数 | 一次模型调用算一次 |
| 用户回合 | 真人开口的轮次，不含 tool_result 回灌 |

## 当前状态

可运行：`wheretoken scan --json` 给出合计 / 按工具 / 按厂家 / 窑墙日历 / 下钻；`wheretoken serve` 在 `127.0.0.1` 打开窑墙。右上角六个字切釉色（窑 / 苔 / 瓷 / 绛 / 青 / 霜），记在本机 `localStorage`。

Cursor：本机 `state.vscdb` 提供请求与用户回合；token 四列在用户授权后走 Cursor 账号 DashboardService（`GetFilteredUsageEvents` / `GetAggregatedUsageEvents`）。本机 `bubble.tokenCount` 仍几乎全是 0，不把上下文窗口快照加成用量。没有登录态时 `errors[]` 会写 `cursor: 未找到本机登录态`，token 列保持 degraded。只有 `~/.cursor`、没有 vscdb 时才显示「已发现，无用量」。

必读：

- [`docs/superpowers/specs/2026-08-15-wheretoken-design.md`](docs/superpowers/specs/2026-08-15-wheretoken-design.md) — 产品与架构规格
- [`docs/superpowers/specs/2026-08-15-wheretoken-calendar-design.md`](docs/superpowers/specs/2026-08-15-wheretoken-calendar-design.md) — 窑墙视觉与日历
- [`docs/data-sources.md`](docs/data-sources.md) — 本机实测的数据源清单与字段映射
- [`opt.md`](opt.md) — 过程决策，供复盘

实现计划：[`docs/superpowers/plans/2026-08-15-wheretoken.md`](docs/superpowers/plans/2026-08-15-wheretoken.md) · [`docs/superpowers/plans/2026-08-15-wheretoken-calendar.md`](docs/superpowers/plans/2026-08-15-wheretoken-calendar.md)。

## 第一次跑

需要 **Go 1.25+**，以及 **Node**（只用来编 `web/`）。

```bash
cd /Users/rainhuang/Desktop/whereToken
cd web && npm install && npm run build && cd ..
go run ./cmd/wheretoken serve
```

浏览器打开 [http://127.0.0.1:8787](http://127.0.0.1:8787)。第一次扫描大约 **10 秒**：要打 Cursor 账号用量接口，再读本机各家数据库。页面像没更新时硬刷新：**Cmd+Shift+R**。

## 再开一次

杀掉占着 8787 的进程，重新编前端，再 serve：

```bash
cd /Users/rainhuang/Desktop/whereToken
lsof -tiTCP:8787 | xargs kill
cd web && npm run build && cd ..
go run ./cmd/wheretoken serve
```

`serve` 每次请求读磁盘上的 `web/dist`。只改了 Vue/CSS 时编一次 dist 再硬刷新即可；Go 代码变了才需要重启进程。

## 开发命令

```bash
go test ./...
go run ./cmd/wheretoken scan --json
go run ./cmd/wheretoken sources
cd web && npm install && npm test && npm run build
go run ./cmd/wheretoken serve          # http://127.0.0.1:8787 ，被占用则 8788–8797
# serve 读的是 web/dist。改 Vue/CSS 先 npm run build，然后 Cmd+Shift+R。
# Go 代码变了才需要重启进程。不要拿一个旧的 serve 配新 API。

cd web && npm run dev                  # Vite :5173，把 /api 代理到 127.0.0.1:8787
# 另开终端：go run ./cmd/wheretoken serve

bash scripts/verify-local.sh           # Kimi / OpenCode 与本机磁盘对照，误差须为 0
```

`scan --json` 和 `GET /api/summary` 是同一份 `EncodeSummary`。窑墙格子来自 `calendar.window_*` + 稀疏 `days`；峰值 / 连烧来自 `calendar.*.stats`，前端不重算。

提交请用 `scripts/commit-no-ai.sh`，不要直接 `git commit`（避免 Cursor 注入 Co-authored-by）。

## 原则（从第一天就锁）

1. **只读本地，加用户授权的 Cursor 账号接口。** 不上传会话，不做排行榜，不读 `auth.json` 去打其它厂商。Cursor 可用本机已登录账号拉自己的用量；JWT 不进 git、不进日志。
2. **诚实。** Claude JSONL 的 `input_tokens` / `output_tokens` 有已知占位 bug；UI 必须标数据质量，而不是把错数印成真理。
3. **单位 M。** 内部用整数 token 累加，展示时 `/ 1e6`。禁止在累加路径上先除再加。
4. **提交不含 AI 署名。** 本仓库的 git 提交不写 Co-authored-by，不把编码助手列入贡献者。

## 许可

MIT。见 [`LICENSE`](LICENSE)。
