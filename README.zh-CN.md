<p align="center">
  <img src="docs/media/logo.png" width="96" alt="whereToken">
</p>

<h1 align="center">whereToken</h1>

<p align="center">
  <b>本机优先的 coding agent token 用量分析。</b>
</p>

<p align="center">
  汇总 Claude Code、Codex、Kimi Code、<br>
  Cursor、OpenCode、Grok CLI、Trae 的 token 用量。
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
  <img src="docs/media/dash-newspaper.jpg" alt="whereToken 仪表盘首页" width="900">
</p>

<p align="center">
  <sub>仪表盘首页 —— <b>墨</b>，黑白报纸风格。</sub>
</p>

用量来自本机已经存下的数据，在本机汇总。没有云同步，没有遥测。

## 功能

### 统一用量

把本机已有账本的 coding agent 加到一张表里。

### 应用与厂家

按 coding agent、模型厂家、模型切开。

### 历史用量

当日合计、连烧、缓存命中率。

### 本机仪表盘

浏览器里看用量，服务只跑在这台电脑上。

### 命令行

终端查询，或导出 JSON。

whereToken 只报 **token 数量**，不估美元。订阅和各家定价对不上本机账本。

## 安装

### Homebrew

```bash
brew tap rainhuang0220/wheretoken
brew install wheretoken
```

### macOS / Linux

```bash
curl -fsSL https://raw.githubusercontent.com/rainhuang0220/whereToken/main/scripts/install.sh | bash
```

### Windows

```powershell
irm https://raw.githubusercontent.com/rainhuang0220/whereToken/main/scripts/install.ps1 | iex
```

脚本会印出安装路径（一般是 `~/.local/bin/wheretoken`）。跑那一行。当前终端找不到命令，就新开一个。

```bash
go install github.com/rainhuang0220/whereToken/cmd/wheretoken@latest
```

仪表盘包含在 GitHub Release 和 `brew tap` 里。`go install` 和 `brew --HEAD` 只有短页。克隆目录：`cd web && npm run build`，再 `WHERETOKEN_WEB=web/dist wheretoken serve`。

`npm/` 包装 **还没上 npm 源**。

## 快速开始

```bash
wheretoken
```

<p align="center">
  <img src="docs/media/cli-kpi.png" alt="whereToken 命令行合计" width="720">
</p>

<table>
  <tr>
    <td width="50%" valign="top"><img src="docs/media/cli-tools.png" alt="按应用"></td>
    <td width="50%" valign="top"><img src="docs/media/cli-vendors.png" alt="按厂家"></td>
  </tr>
  <tr>
    <td align="center"><sub>按应用</sub></td>
    <td align="center"><sub>按厂家</sub></td>
  </tr>
</table>

```bash
wheretoken --today
wheretoken --cursor
wheretoken --vendor=anthropic
wheretoken serve
```

仪表盘在这台电脑的 [http://127.0.0.1:8787](http://127.0.0.1:8787)。**刷新** 才重扫。浏览器重载不会。

完整命令见 [`docs/wheretoken.1`](docs/wheretoken.1)。

## 仪表盘

whereToken 带一个本机网页，用来查看各 agent 的用量。界面目前以中文为主。

**窑** 是项目的吉祥物：一只小窑炉，对应 token 被烧掉。

<p align="center">
  <img src="docs/media/dash-kiln.png" alt="窑，whereToken 的吉祥物" width="900">
</p>

## 支持的 coding agent

whereToken 读的是各 agent 已经写在本机的用量。源数据没有时，它不会编一个数字。

| Agent | 用量数据 | 登录 |
|-------|----------|------|
| Claude Code | 完整 | 不需要 |
| Kimi Code | 完整 | 不需要 |
| Codex | 完整 | 不需要 |
| OpenCode | 完整 | 不需要 |
| Grok CLI | 完整 | 不需要 |
| Cursor | 部分 | token 列需要应用 **已登录** |
| Trae / Trae CN / TRAE SOLO | 部分 | 需要。加密的 `storage.json` 只汇报，不解密 |

各 agent 怎么读，见 [`docs/data-sources.md`](docs/data-sources.md)。

目前不支持：Windsurf、GitHub Copilot、Cline、Lingma。这些工具还没有 whereToken 能稳定读取的本机 token 数据源。

## 应用与厂家

whereToken 把**你使用的 coding agent**和**实际提供模型的厂家**分开记。

Claude Code 里用了一个 MiniMax 模型，记成：

- **应用：** Claude Code
- **厂家：** MiniMax

有的 agent 只有在应用已登录时才给出 token。whereToken 不会把读不到的数据当成零。

## 隐私与安全

whereToken 设计为在本机运行。

### 数据采集

whereToken 不会把用量、会话记录或凭证采集、上传到 whereToken 的服务器。没有遥测，没有云同步。

### 本机数据

它读取的是各 coding agent 已经存在这台电脑上的用量信息。

### 网络访问

仪表盘在本机提供，不依赖远程的 whereToken 服务，也不会暴露给局域网里的其他设备。地址是 [http://127.0.0.1:8787](http://127.0.0.1:8787)。

大多数 agent 只读本地文件。Cursor 和 Trae 可能用那些应用已经存在本机的登录态，访问**它们自己的主机**，补账本里没有的 token 列。

### 凭证

whereToken 不向你要 API key。需要认证时，使用对应应用已经管理好的本机数据或凭证。

### 报告问题

不要在 issue 或日志里附带 API key、session token、JWT、Cookie 或其他秘密。CLI 也不会把这些值打到 stdout。见 [`SECURITY.md`](SECURITY.md)。

## 限制

whereToken 目前是 **alpha**（v0.3.0）。本机已经能用；部分接入还在演进。

- Cursor、Trae 的 token 支持目前不完整
- macOS 的 GitHub 二进制 **尚未签名**（[`docs/macos-signing.md`](docs/macos-signing.md)）
- npm 发行 **还没上 npm 源**
- 仪表盘界面目前以中文为主

## 文档

- 命令行：[docs/wheretoken.1](docs/wheretoken.1)
- 数据源：[docs/data-sources.md](docs/data-sources.md)
- JSON schema：[docs/cli-json.schema.json](docs/cli-json.schema.json)
- 补全：[completions/](completions/)

## 开发

```bash
go test ./...
make test
make ci
bash scripts/verify-cli.sh
cd web && npm install && npm test && npm run build
go run ./cmd/wheretoken serve
```

工具链是 `go.mod` 里的 **Go 1.25.13**。CI 跑在 Ubuntu、macOS、Windows（[ci/github-workflows/](ci/github-workflows/)）。

## 许可证

[MIT](LICENSE)。
