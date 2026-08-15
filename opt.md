# whereToken 决策日志

供复盘。只记**已经拍板或明确留给用户**的决定，不记流水账。
格式：日期 · 主题 · 选项 · 选择 · 理由 · 后果。

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

- **推荐并暂定：B。** 本机工具、要读家目录、要快、要能进工具箱。
- **明确不做：** Tokscale 式 TUI 排行榜、社交 submit、云同步。
- **状态：** **需要用户点头。** 若否决 B，实现计划整份作废。

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
| P1 | Cursor | `state.vscdb` 2.3 GB；`ai-tracking` 无 token 列 | 本地 token 不完整；**禁止**用 Cursor 登录态去拉云端 CSV |
| P1 | 发现器：家目录 `.*` + macOS Application Support 里能识别的其它 agent | 目录存在但用量字段未核完 | MiniMax / Copilot / Trae / OpenClaw 等 |
| P2 | Gemini / Qwen / Amp / Factory / Goose / Hermes / pi | 本机无对应家目录 | 适配器接口先留好，有目录再点亮 |

**不扫：** `auth.json`、Keychain、浏览器 Cookie、厂商账单 API。

## 0.7 Claude「用户回合」定义（锁死）

本机 `type=user` 共 3394 行，其中 **3289 是 `tool_result`**，真人口头回合约 **105**。

- **选择：** 用户回合 = `type==user` 且 content 不是 tool_result。
- **理由：** 若把 tool 回灌算成回合，数字会大一个数量级，和用户直觉相反。

## 0.8 隐私与安全（锁死）

- 扫描只读。任何适配器不得 `UPDATE`/`DELETE` 源文件。
- SQLite 源用 `mode=ro`。
- 仪表盘默认只展示聚合与会话元数据（源、模型、时间、工作区路径、token）。默认不展示 prompt 正文。
- 绑定 `127.0.0.1`，不默认对局域网开放。
- Cursor 的 2.3GB `state.vscdb` 只允许**有键前缀的查询**，禁止全表 dump 进内存。

## 0.9 仓库与提交习惯

- **选择：** 公开 GitHub 仓库 `rainhuang0220/whereToken`（与 PlainList/Flow/docxeditor 一致）。若用户要改私有，下一轮改 visibility。
- 每一轮「核验通过」才提交并 push。v0 的核验 = 规格自检（无占位、公式自洽、数据源与本机一致）。
- `git commit` **禁止** `Co-authored-by:`，禁止把 Cursor/Claude/Codex 等写进 contributors。
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

# 待用户决策（本轮只问这些）

1. **产品形态是否采用方案 B**（Go + 本机 HTTP + Vue，无账号、不上云、不做排行榜）？
2. 若 B 成立：**P0 是否同意「Claude / Kimi / OpenCode / Codex 先闭环，Cursor 放 P1」**？
3. 仓库维持 **public** 是否可以？

其余条目按上表执行，除非用户推翻。
