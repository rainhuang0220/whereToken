# whereToken

本机优先的 token 用量观测器。扫描已经写在磁盘上的 coding agent 账本，把用量摊开：先看合计，再按**工具**和**厂家**拆开。单位是 **M**（百万 token）。

打开页面，第一眼是一堵 **窑墙**：过去一年，一天一块砖。没花过的是冷黏土，烧过的从淡到热。墙右边是峰值和连烧。切「合计 / 某一个工具 / 某一个厂家」，墙和数字一起换序列。

工具 ≠ 厂家。例如在 Claude Code 里跑 MiniMax 时，工具记 Claude Code，厂家记 MiniMax。

whereToken 是同一套本机工具里的第五件，排在 [PlainList](https://github.com/rainhuang0220/PlainList)、[Flow](https://github.com/rainhuang0220/Flow)、[Untitled](https://github.com/rainhuang0220/Untitled)、[docxeditor](https://github.com/rainhuang0220/docxeditor) 之后。

## 你会看到什么

- **合计**：未命中、缓存读、缓存写、输出、总 token、命中率、请求、用户回合
- **按工具 / 按厂家** 两张表，以及折叠的「工具 × 厂家」
- **窑墙**：53 周 × 7 天。指针旁两行：`8月15日` 和当天的 `12.40 M`（或空砖 `0.00 M`）
- 右上角 **刷新** 重新扫描本机账本并拉需要登录的账号用量；**主题** 打开釉厅（窑 / 苔 / 瓷 / 绛 / 昼 / 墨 / 漫 / 端）。点一块看整页预览，**应用** 才带回首页

`scan --json` 与 **刷新**（`POST /api/scan`）是同一份煅烧结果。`GET /api/summary` 只返回**上次煅烧**，所以浏览器重载（F5）不会重读磁盘上的登录态。日桶、峰值、连烧在 Go 里算完，浏览器只渲染。

## 第一次跑

需要 **Go 1.25+**，以及 **Node**（只用来编 `web/`）。

```bash
cd web && npm install && npm run build && cd ..
go run ./cmd/wheretoken serve
```

浏览器打开 [http://127.0.0.1:8787](http://127.0.0.1:8787)。第一次会煅烧本机账本，可能要几秒到十几秒：要读各家目录；Cursor / Trae 在本机已登录时还会拉各自的账号用量。窑墙在煅烧时仍留着上次的砖，上面有进度。

**刷新** = 重新扫描本机账本（不是口头上的「再扫」）。浏览器重载只显示上次煅烧。Cursor / Trae 的 token 列需要对应 IDE 已登录，然后点 **刷新**，不要指望 F5 去重读 JWT。

## 再开一次

杀掉占着 8787 的进程，重新编前端，再 serve：

```bash
lsof -tiTCP:8787 | xargs kill
cd web && npm run build && cd ..
go run ./cmd/wheretoken serve
```

`serve` 每次请求读磁盘上的 `web/dist`。只改了 Vue/CSS 时编一次 dist 再硬刷新即可；Go 代码变了才需要重启进程。

## 开发

```bash
go test ./...
go run ./cmd/wheretoken scan --json
go run ./cmd/wheretoken sources
cd web && npm install && npm test && npm run build
go run ./cmd/wheretoken serve          # http://127.0.0.1:8787 ，被占用则 8788–8797

cd web && npm run dev                  # Vite :5173，把 /api 代理到 127.0.0.1:8787
# 另开终端：go run ./cmd/wheretoken serve

bash scripts/verify-local.sh           # 有本机账本时，与独立脚本对照
```

扫哪些目录、字段怎么映射：[`docs/data-sources.md`](docs/data-sources.md)。

路径一律按当前用户的家目录解析（`os.UserHomeDir()`、XDG、`~/Library/Application Support`、`%APPDATA%`），不写死某台机器的绝对路径。缺的目录会静默跳过。非默认安装可以把那个用户的家目录加进 `WHERETOKEN_EXTRA_ROOTS`（Unix 用 `:`，Windows 用 `;`，也可用逗号）。Codex 还认 `CODEX_HOME`。

## 数据源

默认只扫**已经装过、磁盘上有账本**的工具。macOS 与 Linux 是一等支持；Windows 按 `%APPDATA%` 写了同样的探测，但没有在真实 Windows 机器上跑过。

不要把 token 贴进 whereToken 的界面、issue 或配置。需要登录态的源会去读**该 App 自己已经写下的本机会话**，再只打它自己的主机。

| 源 | 本机路径（约定） | 需要已登录？ | 读什么 |
|----|------------------|--------------|--------|
| Claude Code | `~/.claude/projects/**/*.jsonl`，Linux 另探 `~/.config/claude/projects` | 否 | JSONL 的 `message.usage`；真用户回合（排除 `tool_result`） |
| Kimi Code | `~/.kimi-code/`，并探 `~/.kimi/` | 否 | `wire.jsonl` 的 `usage.record` / `turn.prompt` |
| Codex | `${CODEX_HOME:-~/.codex}/sessions/**/rollout-*.jsonl` | 否 | 累计 `token_count` 的 delta |
| OpenCode | `$XDG_DATA_HOME/opencode` 或 `~/.local/share/opencode` 的 `opencode.db` | 否 | message 级 tokens |
| Cursor | macOS `~/Library/Application Support/Cursor/.../state.vscdb`；Linux `~/.config/Cursor/...`；Windows `%APPDATA%\Cursor\...`；回退 `~/.cursor/` | **token 列需要**本机已登录；请求/回合仍走本地 bubble | 本地 bubble 计请求与回合；token 用 Cursor DashboardService（本机 `cursorAuth/accessToken`，永不打印） |
| Trae / Trae CN / TRAE SOLO | macOS `~/Library/Application Support/{Trae,Trae CN,Trae-CN,TRAE SOLO,TRAE SOLO CN}/User/globalStorage/state.vscdb`；Linux `~/.config/Trae*`；Windows `%APPDATA%\Trae*` | **token 列需要**本机已登录 | 本地 `state.vscdb` 收会话 id；JWT 只从本机 `~/.trae-cn/trae-jwt-token` 或明文 `storage.json` 读（CN 版 `storage.json` 里的登录态常是加密的，不解密）；token 打 Trae 自己的 `get_session_usage`。厂家按模型（DeepSeek / Doubao / GLM…），不是「Trae」。`~/.trae` 技能目录不算账本 |

对话库 `ModularData/ai-agent/database.db` 在 Trae CN 上是 SQLCipher，本轮不解密、不读正文。

### 还没适配

本机或常见安装里能见到、但**这一轮没有做成适配器**（没有清楚、可测的本地 token 账本，或不想做半成品）：Windsurf、GitHub Copilot、Cline、通义灵码 / Lingma、CodeBuddy、Comate、Kiro、Qoder、扣子 Coze。

「科泽」无法唯一对应一个 coding agent：公开资料里常被用来翻译 ByteDance 的 **Coze（扣子）**（无代码 Agent 搭建，不是 IDE 账本）；语音也可能是 **Kiro** 或 **Qoder**。这台机器上没有 Coze / Kiro / Qoder 的 Application Support 或家目录账本，所以没有猜一个适配器。

## 隐私

只读本机目录，HTTP 绑在 `127.0.0.1`。不上传会话，不做遥测。页面和日志都不打印密钥。

Cursor 与 Trae 的 token 四列在你本机已登录时，用**它们自己的**账号用量接口；JWT 不进 git、不进日志、不要粘贴到 whereToken。没有登录态时 token 列会标明质量，不会把空数假装成「没用过」。

不要把 API key、access token 或会话内容贴进 issue。

## 许可

MIT。见 [`LICENSE`](LICENSE)。
