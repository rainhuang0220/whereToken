<p align="center">
  <img src="docs/media/logo.png" width="96" alt="whereToken">
</p>

<h1 align="center">whereToken</h1>

<p align="center">
  本机 coding agent 的 token，花到哪去了。
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
  <img src="docs/media/dash-newspaper.jpg" alt="whereToken 仪表盘" width="900">
</p>

whereToken 是一个 **local-first** 的命令行（可选网页）用量工具。它读的是已经躺在这台电脑上的账本。

**Claude Code · Kimi Code · Codex · Cursor · OpenCode · Grok CLI · Trae**

用量数据留在本机。没有云同步，没有遥测。

<p align="center">
  <img src="docs/media/cli-kpi.png" alt="whereToken 命令行" width="720">
</p>

## 功能

- 把本机已有账本的 agent 加总
- 按**工具**、**厂家**、**模型**切开
- 今天、连烧、缓存命中率
- 给脚本的 JSON
- 可选的本机仪表盘
- 只报 token，不估美元 —— 订阅和各家定价对不上本机账本

## 安装

```bash
brew tap rainhuang0220/wheretoken
brew install wheretoken
```

```bash
curl -fsSL https://raw.githubusercontent.com/rainhuang0220/whereToken/main/scripts/install.sh | bash
```

```powershell
irm https://raw.githubusercontent.com/rainhuang0220/whereToken/main/scripts/install.ps1 | iex
```

curl / irm 会印出装好的路径（一般是 `~/.local/bin/wheretoken`）。跑那一行。当前终端找不到命令，就新开一个。

```bash
go install github.com/rainhuang0220/whereToken/cmd/wheretoken@latest
```

仪表盘在 GitHub Release 和 `brew tap` 里。`go install` / `brew --HEAD` 只有短页 —— 克隆目录：`cd web && npm run build`，再 `WHERETOKEN_WEB=web/dist wheretoken serve`。

`npm/` 包装 **还没上 npm 源**。

## 快速开始

```bash
wheretoken
```

<table>
  <tr>
    <td width="50%" valign="top"><img src="docs/media/cli-tools.png" alt="按工具"></td>
    <td width="50%" valign="top"><img src="docs/media/cli-vendors.png" alt="按厂家"></td>
  </tr>
  <tr>
    <td align="center"><sub>按工具</sub></td>
    <td align="center"><sub>按厂家</sub></td>
  </tr>
</table>

```bash
wheretoken --today
wheretoken --cursor          # 还有 --claude --kimi --grok --codex --opencode --trae
wheretoken --vendor=xai
wheretoken --model=k3
wheretoken --json
wheretoken --offline         # 跳过 Cursor / Trae 云端
wheretoken serve
```

然后在这台电脑打开 [http://127.0.0.1:8787](http://127.0.0.1:8787)。**刷新** 才重扫。浏览器重载不会。

其余看 `wheretoken --help`。单位是 **M**（百万 token）。

## 仪表盘

上面那张首页是 **墨**：黑白，像报纸。作者最喜欢。

**窑** 是名字的来处。token 烧得快，像窑炉。吉祥物就是那只小窑炉。

<p align="center">
  <img src="docs/media/dash-kiln.png" alt="窑：吉祥物那只小窑炉" width="900">
</p>

## 支持的工具

只扫磁盘上已经有账本的。字段对照：[`docs/data-sources.md`](docs/data-sources.md)。

| 工具 | token 数据 | 要登录吗 |
|------|------------|----------|
| Claude Code | 有 | 不用 |
| Kimi Code | 有 | 不用 |
| Grok CLI | 有 | 不用 |
| Codex | 有 | 不用 |
| OpenCode | 有 | 不用 |
| Cursor | 部分 —— 本地气泡仍计请求 / 回合 | **要。** token 列需要应用 **已登录** |
| Trae / Trae CN / TRAE SOLO | 部分 | **要。** 加密的 `storage.json` 只汇报，不解密 |

这一版还没有 Windsurf、Copilot、Cline、Lingma 之类。

## 工具和厂家

whereToken 把**你敲命令的那个应用**和**背后的模型厂家**分开算。

Claude Code 里跑了一个 MiniMax 模型，记成：

- **工具：** Claude Code
- **厂家：** MiniMax

这样既能看见你在哪问的，也能看见谁真正接的单。

## 隐私

whereToken 是 local-first。

它读的是这台电脑上已经有的用量文件。不会把这些文件上传，不会同步到 whereToken 的服务器，也不会做遥测。

它不向你要 API key，也不会把你的凭证发给 whereToken。

大多数来源只读本地文件。Cursor 和 Trae 可能用那些应用已经存在本机的登录态，补账本里没有的 token 列。那次请求打的是**它们自己的主机**，不是 whereToken。

可选的仪表盘也跑在这台电脑上，不会暴露给局域网里的其他设备。

## 安全

CLI 从不打印 JWT、access token、API key、Cookie。不要把秘密贴进 issue。其余见 [`SECURITY.md`](SECURITY.md)。

## 当前限制

**v0.3.0 是 Alpha。** 本机已经能用。有些接入还在变。界面目前是中文。

- macOS 的 GitHub 二进制 **还没签名**（[`docs/macos-signing.md`](docs/macos-signing.md)）
- **没有 npm 包**
- Cursor / Trae 的 token 列需要那些应用在这台机器上 **已登录**。Claude / Kimi / Grok / Codex / OpenCode 不用

<details>
<summary>旗标、环境变量、退出码</summary>

```bash
wheretoken --ascii
wheretoken --width 40
wheretoken --quiet
wheretoken sources
wheretoken scan --json       # 和页内「刷新」同一份；不是 schema 1；不吃 --today / --tool
wheretoken completion zsh    # 还有 bash、fish、powershell
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
