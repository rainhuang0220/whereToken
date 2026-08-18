<p align="center">
  <img src="docs/media/logo.png" width="96" alt="whereToken">
</p>

<h1 align="center">whereToken</h1>

<p align="center">
  <b>本机优先的 coding agent token 用量分析。</b>
</p>

<p align="center">
  汇总 Claude Code、Kimi Code、Codex、Cursor、<br>
  OpenCode、Grok CLI、Trae 及其他已支持工具的 token 用量。
</p>

<p align="center">
  <a href="./README.md">English</a> ·
  <a href="./README.zh-CN.md"><b>简体中文</b></a>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/status-alpha-FFD700?style=flat-square" alt="Status: Alpha">
  <a href="https://github.com/rainhuang0220/whereToken/releases"><img src="https://img.shields.io/github/v/release/rainhuang0220/whereToken?include_prereleases&style=flat-square" alt="release"></a>
  <a href="https://github.com/rainhuang0220/whereToken/actions/workflows/ci.yml"><img src="https://img.shields.io/github/actions/workflow/status/rainhuang0220/whereToken/ci.yml?branch=main&style=flat-square&label=CI" alt="CI"></a>
  <a href="./LICENSE"><img src="https://img.shields.io/github/license/rainhuang0220/whereToken?style=flat-square" alt="MIT"></a>
  <a href="https://go.dev/"><img src="https://img.shields.io/github/go-mod/go-version/rainhuang0220/whereToken?style=flat-square" alt="Go"></a>
</p>

<p align="center">
  <img src="docs/media/dash-newspaper.jpg" alt="whereToken 仪表盘" width="900">
</p>

<p align="center">
  <sub><b>墨</b> 是黑白报纸风格的主题。</sub>
</p>

现在开发者往往会同时使用多个 coding agent。各工具把用量存在不同的地方，很难一眼看到 token 花在哪里。whereToken 发现这些已有数据，归一化之后，用命令行、本机仪表盘和 JSON 给出同一份结果。

它设计为在本机运行。欢迎反馈和缺陷报告。

## 功能

### 统一用量

在一个界面里查看本机已有数据的各 coding agent 用量。

### 应用、厂家与模型

区分发出请求的应用、提供模型的厂家，以及具体模型。

### 历史用量

当日合计、连烧、缓存命中率。

### 本机仪表盘

在浏览器中查看同一份数据，服务只跑在这台电脑上。

### 命令行与 JSON

在终端查询，或导出归一化后的 JSON 供脚本使用。

whereToken 报告 token 数量，不估算金额。

## 安装

### 推荐：Homebrew

```bash
brew tap rainhuang0220/wheretoken
brew install wheretoken
```

### 预编译二进制

macOS / Linux：

```bash
curl -fsSL https://raw.githubusercontent.com/rainhuang0220/whereToken/main/scripts/install.sh | bash
```

Windows：

```powershell
irm https://raw.githubusercontent.com/rainhuang0220/whereToken/main/scripts/install.ps1 | iex
```

脚本会印出安装路径（一般是 `~/.local/bin/wheretoken`）。跑那一行。当前终端找不到命令，就新开一个。

### 从源码构建

```bash
go install github.com/rainhuang0220/whereToken/cmd/wheretoken@latest
```

Release 二进制和 `brew tap` 包含仪表盘。`go install` 和 `brew --HEAD` 只编命令行。从克隆目录启动仪表盘时，先构建网页（`cd web && npm run build`），再设置 `WHERETOKEN_WEB` 为 `web/dist`。

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
wheretoken --since 7d
wheretoken --json
wheretoken serve
wheretoken doctor
wheretoken rebuild
wheretoken update
wheretoken uninstall
```

`wheretoken doctor` 说明哪些 agent 被发现、用量是否完整。`wheretoken rebuild` 删除本机扫描缓存并重新读取 agent 数据。完整命令见 `wheretoken --help`。

## 仪表盘

启动本机仪表盘：

```bash
wheretoken serve
```

仪表盘在本机运行，用于查看各 coding agent、厂家和模型的用量。页内刷新才会重新扫描；浏览器重载不会。

**窑** 是 whereToken 的窑炉吉祥物。

<p align="center">
  <img src="docs/media/dash-kiln.png" alt="窑，whereToken 的吉祥物" width="900">
</p>

## 支持的 coding agent

whereToken 读取各 coding agent 提供的用量信息。完整程度因工具而异，有时取决于应用是否已登录。

| Coding agent | 用量数据 | 认证 |
|--------------|----------|------|
| Claude Code | 完整 | 不需要 |
| Kimi Code | 完整 | 不需要 |
| Codex | 完整 | 不需要 |
| OpenCode | 完整 | 不需要 |
| Grok CLI | 完整 | 不需要 |
| MiniMax Agent | 完整 | 不需要 |
| OpenClaw | 完整 | 不需要 |
| Cursor | 部分 | token 列需要 |
| Trae / Trae CN / TRAE SOLO | 部分 | 需要 |

Cursor 和 Trae 的 token 列需要那些应用在本机 **已登录**。加密的 Trae 存储只汇报，不解密。

当某个 coding agent 没有提供可靠的用量信息时，whereToken 会标为不可用，而不是记成零。

各 agent 的读取方式见 [`docs/data-sources.md`](docs/data-sources.md)，归一化字段见 [`docs/token-accounting.md`](docs/token-accounting.md)。

### 目前不支持

Windsurf、GitHub Copilot、Cline、Lingma 目前不受支持，因为 whereToken 还没有这些工具可靠的用量数据源。

## 工作原理

```text
Coding agents
      ↓
本地文件；部分 agent 还会使用它们自己的用量接口
      ↓
按来源适配
      ↓
归一化后的用量
      ↓
命令行 / 仪表盘 / JSON
```

whereToken 发现各 coding agent 的用量信息，把来源各异的记录归一成同一套表示，再通过命令行、仪表盘和 JSON 输出。再次扫描时只会把本机文件索引当缓存；`wheretoken rebuild` 会删掉该索引并重新读取各 agent。

它区分你使用的 **coding agent** 和实际提供模型的 **厂家**。

通过 Claude Code、由 MiniMax 模型完成的请求记为：

- **应用：** Claude Code
- **厂家：** MiniMax

## 隐私与安全

### 本机优先

whereToken 设计为在本机运行，不依赖 whereToken 云服务。

### 数据采集

whereToken 不会把用量数据、会话记录或凭证上传到 whereToken 的服务器。没有遥测。

### 数据来源

用量信息来自各 coding agent 提供的数据。大多数来源是本机应用数据。

Cursor 和 Trae 可能使用那些应用已经保存的认证信息，访问它们自己的接口，以获取本地文件中没有的 token 列。

### 凭证

whereToken 不要求用户把 API key 粘贴进本程序。需要认证时，使用对应应用已经管理的本机数据或凭证。

### 安全策略

安全问题与安全策略见 [`SECURITY.md`](SECURITY.md)。不要在缺陷报告中附带 API key、会话令牌或其他秘密。

## 限制

whereToken 目前处于 **alpha**。

- Release 二进制目前 **未签名**（[`docs/macos-signing.md`](docs/macos-signing.md)）
- 目前未发布 npm 包
- 部分 agent 只提供不完整的用量信息
- 部分接入需要对应应用已登录
- 仪表盘界面目前以中文为主

## 文档

- 命令行参考：[`docs/wheretoken.1`](docs/wheretoken.1)
- 数据源：[`docs/data-sources.md`](docs/data-sources.md)
- Token 账本：[`docs/token-accounting.md`](docs/token-accounting.md)
- 估价：[`docs/cost.md`](docs/cost.md)
- 增加适配器：[`docs/adding-an-adapter.md`](docs/adding-an-adapter.md)
- JSON 输出格式：[`docs/cli-json.schema.json`](docs/cli-json.schema.json)
- 补全：[`completions/`](completions/)
- 安全策略：[`SECURITY.md`](SECURITY.md)
- 变更记录：[`CHANGELOG.md`](CHANGELOG.md)

完整命令行参考、环境变量、退出码和 JSON 输出格式见项目文档。

## 开发

```bash
go test ./...
make test
make ci
```

```bash
cd web && npm install && npm test && npm run build
go run ./cmd/wheretoken serve
```

## 许可证

[MIT](LICENSE)。
