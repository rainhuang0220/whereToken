<p align="center">
  <img src="docs/media/logo.png" width="96" alt="whereToken 窑崽">
</p>

<h1 align="center">whereToken</h1>

<p align="center">
  本机 coding agent 的 token，烧到哪去了。<br>
  一行命令，一张表。想看墙，就 <code>serve</code>。
</p>

<p align="center">
  <a href="./README.md">English</a> ·
  <a href="./README.zh-CN.md"><b>简体中文</b></a>
</p>

<p align="center">
  <a href="https://github.com/rainhuang0220/whereToken/releases"><img src="https://img.shields.io/github/v/release/rainhuang0220/whereToken?include_prereleases&style=flat-square&color=FFD700&label=alpha" alt="alpha"></a>
  <a href="https://github.com/rainhuang0220/whereToken/actions/workflows/ci.yml"><img src="https://img.shields.io/github/actions/workflow/status/rainhuang0220/whereToken/ci.yml?branch=main&style=flat-square&label=CI" alt="CI"></a>
  <a href="./LICENSE"><img src="https://img.shields.io/github/license/rainhuang0220/whereToken?style=flat-square" alt="MIT"></a>
  <a href="https://go.dev/"><img src="https://img.shields.io/github/go-mod/go-version/rainhuang0220/whereToken?style=flat-square" alt="Go"></a>
  <img src="https://img.shields.io/badge/platform-macOS%20%7C%20Linux%20%7C%20Windows-informational?style=flat-square" alt="macOS Linux Windows">
</p>

<p align="center">
  <img src="docs/media/cli-kpi.png" alt="wheretoken — 有账本以来的八格" width="720">
</p>

> **v0.3.0 Alpha。** 对着自己的账本跑。[不对就开 issue](https://github.com/rainhuang0220/whereToken/issues)，也欢迎 PR。

只读本机已经有的账本。不连云，不标美元，不做遥测。

**工具 ≠ 厂家** —— Claude Code 跑 MiniMax，仍是 Claude Code / MiniMax。Cursor 没登录写在注里，不当成假零。

<table>
  <tr>
    <td width="50%" valign="top"><img src="docs/media/cli-tools.png" alt="按工具排行"></td>
    <td width="50%" valign="top"><img src="docs/media/cli-vendors.png" alt="按厂家排行"></td>
  </tr>
  <tr>
    <td align="center"><sub>按工具</sub></td>
    <td align="center"><sub>按厂家</sub></td>
  </tr>
</table>

## 安装

```bash
brew tap rainhuang0220/wheretoken && brew install wheretoken
```

```bash
curl -fsSL https://raw.githubusercontent.com/rainhuang0220/whereToken/main/scripts/install.sh | bash
```

```powershell
irm https://raw.githubusercontent.com/rainhuang0220/whereToken/main/scripts/install.ps1 | iex
```

curl / irm 会印出路径（一般是 `~/.local/bin/wheretoken`）。跑那一行。当前终端找不到命令的话，新开一个。

```bash
go install github.com/rainhuang0220/whereToken/cmd/wheretoken@latest   # Go 1.25+
```

窑墙在 GitHub Release 和 `brew tap` 里。`go install` / `brew --HEAD` 只有短页 —— 克隆目录：`cd web && npm run build`，再 `WHERETOKEN_WEB=web/dist wheretoken serve`。

`npm/` 包装 **还没上 npm 源**。

## 用法

```bash
wheretoken                 # 有账本以来
wheretoken --today         # 本机时区的今天
wheretoken --cursor        # 还有 --claude --kimi --grok --codex --opencode --trae
wheretoken --vendor=xai
wheretoken --model=k3
wheretoken --json          # schema 1
wheretoken --offline       # 跳过 Cursor / Trae 云端
wheretoken serve           # http://127.0.0.1:8787
```

其余看 `wheretoken --help`。单位是 **M**（百万 token）。

## 窑墙

```bash
wheretoken serve           # http://127.0.0.1:8787
```

**刷新** 才重扫。浏览器重载不会。

**墨** 是报纸那套，作者最喜欢。

<p align="center">
  <img src="docs/media/dash-newspaper.jpg" alt="首页，报纸釉「墨」" width="900">
</p>

**窑** 是名字的来处。token 烧得快，像窑炉。吉祥物就是那只小窑炉。

<p align="center">
  <img src="docs/media/dash-kiln.png" alt="窑：吉祥物那只小窑炉" width="900">
</p>

## 账本

只扫磁盘上已经有的。字段对照：[`docs/data-sources.md`](docs/data-sources.md)。

| 来源 | 不登录也能读 token？ |
|------|----------------------|
| Claude Code | 能 |
| Kimi Code | 能 |
| Grok CLI | 能 |
| Codex | 能 |
| OpenCode | 能 |
| Cursor | 本地有请求 / 回合。**token 列要应用已登录。** |
| Trae / Trae CN / TRAE SOLO | **要登录。** 加密的 `storage.json` 只汇报，不解密。 |

还没有 Windsurf / Copilot / Cline / Lingma —— 等它们留下清楚的本机 token 账本。

`wheretoken scan --json` 和页内 **刷新** 是同一份，不是 schema 1，也不吃 `--today` / `--tool`。

<details>
<summary>旗标、环境变量、退出码</summary>

```bash
wheretoken --ascii
wheretoken --width 40
wheretoken --quiet
wheretoken sources
wheretoken completion zsh   # 还有 bash、fish、powershell
```

补全：[`completions/`](completions/)。手册：[`docs/wheretoken.1`](docs/wheretoken.1)。JSON：[schema 1](docs/cli-json.schema.json)。

| 环境变量 | |
|----------|--|
| `NO_COLOR` / `FORCE_COLOR` | 颜色（`NO_COLOR` 优先） |
| `WHERETOKEN_ASCII=1` / `NO_UTF8` | ASCII 框线 |
| `WHERETOKEN_OFFLINE=1` | 同 `--offline` |
| `WHERETOKEN_HOME` / `--home` | 测试用假家目录 |
| `WHERETOKEN_EXTRA_ROOTS` | 额外家目录（`:` / `;` / 逗号） |
| `WHERETOKEN_WEB` | `serve` 用的 `web/dist` |
| `CODEX_HOME` | Codex 也会读 |
| `COLUMNS` | 同 `--width` |

退出码：`0` 正常（没数据、登录残缺也算），`1` 失败，`2` 用法错。

</details>

## 隐私

只读本机。`serve` 绑 `127.0.0.1`。无遥测，不加载第三方字体，stdout 不印 JWT。别把秘密贴进 issue。[`SECURITY.md`](SECURITY.md)。

## 尚未

- GitHub 二进制 **未签名**（[`docs/macos-signing.md`](docs/macos-signing.md)）。
- **没有 npm 包。**
- Trae / Cursor 的 **token 列** 需要应用 **已登录**。Claude / Kimi / Grok / Codex / OpenCode 不用。

## 开发

```bash
go test ./...
make test
make ci
bash scripts/verify-cli.sh
cd web && npm install && npm test && npm run build
go run ./cmd/wheretoken serve
```

工具链是 `go.mod` 里的 **Go 1.25.13**。CI：Ubuntu / macOS / Windows（[`ci/github-workflows/`](ci/github-workflows/)）。

## 许可证

[MIT](LICENSE)。
