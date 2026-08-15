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

whereToken 是一个**本机优先**的 token 用量观测器。它不去云端拉账单，而是去读你已经写在磁盘上的会话账本：

`~/.claude` · `~/.codex` · `~/.kimi-code` · `~/.local/share/opencode` · `~/.cursor` · 以及家目录里其它能识别的 agent 根。

**有什么算什么。** 适配器按目录探测；扫到的才进表，扫不到的不假装有数。

同一套六列会出现三次：**合计**、**按工具**、**按厂家**（Claude Code 里跑 MiniMax 时，工具记 Claude Code、厂家记 MiniMax）。token 一律 **M = 百万**：

| 指标 | 含义 |
|------|------|
| 总 token（含缓存读取） | miss + cache read + cache create + output |
| 输入缓存命中率 | cache read / (cache read + miss + cache create) |
| 未命中 token | 未走缓存的输入（miss） |
| 输出 token | 模型写出（含 reasoning，若源提供则同时单列） |
| 请求次数 | 一次模型调用算一次 |
| 用户回合 | 真人开口的轮次，不含 tool_result 回灌 |

## 当前状态

可运行：`wheretoken scan --json` 给出合计 / 按工具 / 按厂家；`wheretoken serve` 在 `127.0.0.1` 提供同一份 JSON 和 Vue 页。

必读：

- [`docs/superpowers/specs/2026-08-15-wheretoken-design.md`](docs/superpowers/specs/2026-08-15-wheretoken-design.md) — 产品与架构规格
- [`docs/superpowers/specs/2026-08-15-wheretoken-calendar-design.md`](docs/superpowers/specs/2026-08-15-wheretoken-calendar-design.md) — 窑墙视觉与日历
- [`docs/data-sources.md`](docs/data-sources.md) — 本机实测的数据源清单与字段映射
- [`opt.md`](opt.md) — 过程决策，供复盘

实现计划：[`docs/superpowers/plans/2026-08-15-wheretoken.md`](docs/superpowers/plans/2026-08-15-wheretoken.md)。

## 开发命令

```bash
go test ./...
go run ./cmd/wheretoken scan --json
go run ./cmd/wheretoken sources
go run ./cmd/wheretoken serve          # http://127.0.0.1:8787 ，被占用则 8788–8797

cd web && npm install && npm test && npm run build && npm run dev
# 另开终端：go run ./cmd/wheretoken serve
# Vite 把 /api 代理到 127.0.0.1:8787

bash scripts/verify-local.sh           # Kimi / OpenCode 与本机磁盘对照，误差须为 0
```

提交请用 `scripts/commit-no-ai.sh`，不要直接 `git commit`（避免 Cursor 注入 Co-authored-by）。

## 原则（从第一天就锁）

1. **只读本地。** 不上传会话，不做排行榜，不读 `auth.json` 去调厂商账单 API（Cursor 云端 CSV 那条路明确不做，见规格）。
2. **诚实。** Claude JSONL 的 `input_tokens` / `output_tokens` 有已知占位 bug；UI 必须标数据质量，而不是把错数印成真理。
3. **单位 M。** 内部用整数 token 累加，展示时 `/ 1e6`。禁止在累加路径上先除再加。
4. **提交不含 AI 署名。** 本仓库的 git 提交不写 Co-authored-by，不把编码助手列入贡献者。

## 许可

MIT。见 [`LICENSE`](LICENSE)。
