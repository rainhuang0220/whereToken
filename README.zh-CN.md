<p align="center">
  <img src="docs/media/logo.png" width="108" alt="whereToken 窑崽">
</p>

<h1 align="center">whereToken</h1>

<p align="center">
  <b>本机 coding agent 的 token，烧到哪去了。</b><br>
  一行命令，一张字符表。想看墙，就 <code>serve</code>。<br>
  不连云、不标美元价、不做遥测。
</p>

<p align="center">
  <a href="./README.md">English</a> · <b>简体中文</b>
</p>

<p align="center">
  <a href="#安装">安装</a> ·
  <a href="#你会看到什么">你会看到什么</a> ·
  <a href="#窑墙">窑墙</a> ·
  <a href="#读哪些账本">账本</a> ·
  <a href="#隐私">隐私</a>
</p>

<p align="center">
  <a href="https://github.com/rainhuang0220/whereToken/releases"><img src="https://img.shields.io/github/v/release/rainhuang0220/whereToken?include_prereleases&style=for-the-badge&color=FFD700&label=alpha" alt="alpha 版本"></a>
  <a href="https://github.com/rainhuang0220/whereToken/actions/workflows/ci.yml"><img src="https://img.shields.io/github/actions/workflow/status/rainhuang0220/whereToken/ci.yml?branch=main&style=for-the-badge&label=CI" alt="CI"></a>
  <a href="./LICENSE"><img src="https://img.shields.io/github/license/rainhuang0220/whereToken?style=for-the-badge" alt="MIT"></a>
  <a href="https://go.dev/"><img src="https://img.shields.io/github/go-mod/go-version/rainhuang0220/whereToken?style=for-the-badge&color=00ADD8" alt="Go"></a>
  <img src="https://img.shields.io/badge/macOS%20%7C%20Linux%20%7C%20Windows-只读本机-24292f?style=for-the-badge" alt="macOS Linux Windows，只读本机">
</p>

<p align="center">
  <img src="docs/media/cli.png" alt="whereToken 终端：金色窑崽、八格 KPI、工具和厂家排行" width="820">
</p>

<p align="center"><sub>2026-08-18 实机，<code>--offline</code>。你的数字不会长这样。</sub></p>

> **v0.3.0 是 Alpha。** 装上，对着自己的账本跑；不对或缺了就 [开 issue](https://github.com/rainhuang0220/whereToken/issues)，也欢迎 PR。点子和坏掉的账本都有用。

## 为什么要有它

Claude Code、Cursor、Kimi、Grok、Codex、OpenCode、Trae，常常同时在烧。各自有各自的一角，没有谁肯把整垛加起来。

whereToken 只读已经躺在这台机器上的账本，打出一张表：**有账本以来**，单位是 **百万 token（M）**。命中率、连烧、请求、用户回合，再按**工具**和**厂家**排。这两件事不是一回事 —— Claude Code 跑 MiniMax，工具仍是 Claude Code，厂家是 MiniMax。

不写美元价。不往外打电话。

## 你会看到什么

- **只读本机** — 家目录里的账本。HTTP 只绑 `127.0.0.1`。
- **字符表** — 金色窑崽、八格 KPI、近 7 日火花、排行。
- **窑墙观察台** — 53 周砖墙、下钻、八种釉。
- **注是真的** — Cursor 没登录会写在注里，不当成「这个工具你没用过」的 `0.00 M`。
- **给脚本** — `--json` 是 [schema 1](docs/cli-json.schema.json)。
- **嘴严** — JWT、Cookie、API key 不会出现在 stdout。

```bash
wheretoken                 # 有账本以来
wheretoken --today         # 本机时区的今天
wheretoken --cursor        # 还有 --claude --kimi --grok --codex --opencode --trae
wheretoken --vendor=xai
wheretoken --model=k3
wheretoken serve           # 窑墙 → http://127.0.0.1:8787
```

<p align="center">
  <img src="docs/media/cli-today.png" alt="wheretoken --today：一天、一个工具、一个模型" width="720">
</p>

<p align="center"><sub><code>wheretoken --today</code> — 只看今天，并列出总表里不展开的模型。</sub></p>

## 安装

macOS / Linux：

```bash
brew tap rainhuang0220/wheretoken
brew install wheretoken
```

```bash
curl -fsSL https://raw.githubusercontent.com/rainhuang0220/whereToken/main/scripts/install.sh | bash
```

Windows（PowerShell）：

```powershell
irm https://raw.githubusercontent.com/rainhuang0220/whereToken/main/scripts/install.ps1 | iex
```

curl / irm 脚本会印出装好的路径（一般是 `~/.local/bin/wheretoken`）。跑它印的那一行。当前这个终端还没有 `~/.local/bin`，新开一个才会直接打 `wheretoken`。包会按 `checksums.txt` 做 SHA-256 校验。

已经有 **Go 1.25+**：

```bash
go install github.com/rainhuang0220/whereToken/cmd/wheretoken@latest
```

克隆下来之后，Homebrew 仍可从源码编（需要 Go）：

```bash
brew install --HEAD ./Formula/wheretoken.rb
```

窑墙嵌在 **GitHub Release** 二进制和 `brew tap rainhuang0220/wheretoken` 里。`go install` 和 `brew --HEAD` 只有一段短 HTML —— 克隆目录里先 `cd web && npm run build`，再 `WHERETOKEN_WEB=web/dist wheretoken serve`。

`npm/` 包装 **还没上 npm 源**。

## 窑墙

```bash
wheretoken serve
```

打开 [http://127.0.0.1:8787](http://127.0.0.1:8787)。页内 **刷新** 才重新扫描本机（以及已登录的 Cursor / Trae）。浏览器重载只会显示上次结果。

<p align="center">
  <img src="docs/media/dashboard.png" alt="whereToken 窑墙：53 周砖、KPI、工具和厂家表" width="900">
</p>

八种釉。点 **主题** 换一块。观察台不加载 Google Fonts，也不加载别的第三方资源。

<p align="center">
  <img src="docs/media/themes.png" alt="八种釉：窑、苔、瓷、绛、昼、墨、漫、端" width="900">
</p>

## 读哪些账本

默认只扫磁盘上已经有账本的工具。macOS / Linux 是一等公民；Windows 走同样的 `%APPDATA%`。

| 来源 | 不登录也能读 token？ |
|------|----------------------|
| Claude Code | 能 — `~/.claude/projects/**/*.jsonl` |
| Kimi Code | 能 — `~/.kimi-code/` |
| Grok CLI | 能 — `~/.grok/sessions/**/updates.jsonl` |
| Codex | 能 — `${CODEX_HOME:-~/.codex}/sessions/**/rollout-*.jsonl` |
| OpenCode | 能 — `$XDG_DATA_HOME/opencode` 或 `~/.local/share/opencode` |
| Cursor | 本地气泡有请求 / 回合。**token 列需要应用已登录。** |
| Trae / Trae CN / TRAE SOLO | **要登录。** 加密的 `storage.json` 只汇报，不解密。 |

字段对照：[`docs/data-sources.md`](docs/data-sources.md)。这一版还没有 Windsurf、Copilot、Cline、Lingma 之类 —— 要等它们有一份清楚的本机 token 账本。

`wheretoken scan --json` 和窑墙 **刷新** 是同一份观察台 payload。它 **不是** schema 1，也不吃 `--today` / `--tool`。

<details>
<summary>更多旗标、环境变量、退出码</summary>

```bash
wheretoken --today --cursor
wheretoken --tool=claude
wheretoken --json              # schema 1 — docs/cli-json.schema.json
wheretoken --ascii             # 老的 Windows 控制台
wheretoken --width 40          # 排行先丢掉回合/请求，再把名字收成 C...
wheretoken --quiet             # stderr 不印「正在读 …」
wheretoken --offline           # 只用本机账本，跳过 Cursor/Trae 云端
wheretoken sources
wheretoken completion zsh      # 还有 bash、fish、powershell
```

仓库里的补全：[`completions/`](completions/)。手册页：[`docs/wheretoken.1`](docs/wheretoken.1)。

```bash
wheretoken completion zsh > ~/.zfunc/_wheretoken   # 然后把 ~/.zfunc 加进 fpath
```

`--json`（schema 1）有 `period`、`total`、`total_m`、`hit_rate`、`requests`、`user_turns`、`tools`、`vendors`、`notes`。`--today` 多 `models`，没有 `last_7d` / 连烧。`--model` 会带 `hide_turns: true`，因为回合是按工具计的。

| 环境变量 | 作用 |
|----------|------|
| `NO_COLOR` | 不要 ANSI（和 `--no-color` 一样） |
| `FORCE_COLOR` | 即使 stdout 不是 TTY 也上色（`NO_COLOR` 优先） |
| `WHERETOKEN_ASCII=1` / `NO_UTF8` | ASCII 框线 |
| `WHERETOKEN_HOME` / `--home` | 测试用假家目录 |
| `WHERETOKEN_OFFLINE=1` | 同 `--offline` |
| `WHERETOKEN_EXTRA_ROOTS` | 额外家目录（Unix `:`，Windows `;`，逗号也行） |
| `WHERETOKEN_WEB` | `serve` 用的 `web/dist` 目录 |
| `CODEX_HOME` | Codex 也会读 |
| `COLUMNS` | 限制排行宽度（同 `--width`） |

退出码：`0` 正常（包括没数据、登录残缺），`1` 运行失败，`2` 用法错误（不认识的命令 / 工具 / 厂家 / 模型）。

</details>

## 隐私

只读本机账本。`serve` 只绑 `127.0.0.1`，外来的 `Host` / `Origin` / `Referer` 会拒。无遥测。窑墙不加载第三方字体。CLI 从不打印 JWT、access token、API key、Cookie。不要把秘密贴进 issue。见 [`SECURITY.md`](SECURITY.md)。

## 尚未

短名单，不是路线图：

- GitHub Release 的 macOS 二进制 **还没签名**，要等 Apple 签名密钥配上（[`docs/macos-signing.md`](docs/macos-signing.md)）。
- **没有 npm 包。**
- Trae / Cursor 的 **token 列** 需要那些应用在这台机器上 **已登录**。本机的 Claude / Kimi / Grok / Codex / OpenCode 账本不用。

## 开发

```bash
go test ./...
make test                          # go + web + npm 包装
make ci                            # fmt-check + vet + test + race + 夹具 CLI
bash scripts/verify-cli.sh         # 对着 testdata 打表，不读 $HOME
bash scripts/verify-local.sh       # 可选：扫这台机器自己的账本
cd web && npm install && npm test && npm run build
go run ./cmd/wheretoken serve
```

`go test` / `go install` 用 `go.mod` 里的 **Go 1.25.13** 工具链（本机 1.25.0+ 会自己下）。克隆目录里，只有 `go.mod` 是本模块时，`go run` 才会读 `./web/dist`。

CI 跑在 GitHub Actions（Ubuntu、macOS、Windows）。同一份 YAML 也放在 [`ci/github-workflows/`](ci/github-workflows/)。

## 许可证

[MIT](LICENSE)。Copyright (c) 2026 rainhuang0220.
