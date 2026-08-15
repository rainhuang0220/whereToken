# whereToken

本机优先的 token 用量观测器。扫描已经写在磁盘上的 coding agent 账本，把用量摊开：先看合计，再按**工具**和**厂家**拆开。单位是 **M**（百万 token）。

打开页面，第一眼是一堵 **窑墙**：过去一年，一天一块砖。没花过的是冷黏土，烧过的从淡到热。墙右边是峰值和连烧。切「合计 / 某一个工具 / 某一个厂家」，墙和数字一起换序列。

工具 ≠ 厂家。例如在 Claude Code 里跑 MiniMax 时，工具记 Claude Code，厂家记 MiniMax。

whereToken 是同一套本机工具里的第五件，排在 [PlainList](https://github.com/rainhuang0220/PlainList)、[Flow](https://github.com/rainhuang0220/Flow)、[Untitled](https://github.com/rainhuang0220/Untitled)、[docxeditor](https://github.com/rainhuang0220/docxeditor) 之后。

## 你会看到什么

- **合计**：未命中、缓存读、缓存写、输出、总 token、命中率、请求、用户回合
- **按工具 / 按厂家** 两张表，以及折叠的「工具 × 厂家」
- **窑墙**：53 周 × 7 天。指针旁两行：`8月15日` 和当天的 `12.40 M`（或空砖 `0.00 M`）
- 右上角 **刷新** 重新扫描；**主题** 打开釉色页（窑 / 苔 / 瓷 / 绛 / 青墨 / 霜碳 / 昼 / 墨），选好后 **应用** 回首页

`scan --json` 与 `GET /api/summary` 是同一份结果。日桶、峰值、连烧在 Go 里算完，浏览器只渲染。

## 第一次跑

需要 **Go 1.25+**，以及 **Node**（只用来编 `web/`）。

```bash
cd web && npm install && npm run build && cd ..
go run ./cmd/wheretoken serve
```

浏览器打开 [http://127.0.0.1:8787](http://127.0.0.1:8787)。第一次扫描可能要几秒到十几秒：要读本机各家账本，Cursor 在已登录时还会拉账号用量。页面像没更新时硬刷新（Cmd+Shift+R / Ctrl+Shift+R）。

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

## 隐私

只读本机目录，HTTP 绑在 `127.0.0.1`。不上传会话，不打其它厂商的云，不做遥测。页面和日志都不打印密钥。

Cursor 的 token 四列在你本机已登录时，用 Cursor 自己的账号用量接口；JWT 不进 git、不进日志。没有登录态时 token 列会标明质量，不会把空数假装成「没用过」。

不要把 API key、access token 或会话内容贴进 issue。

## 许可

MIT。见 [`LICENSE`](LICENSE)。
