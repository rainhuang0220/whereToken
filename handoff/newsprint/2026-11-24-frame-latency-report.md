# Newsprint 第二轮延迟实验

基线为已部署提交 `b68b403`。实验在当前目录的 `codex/newsprint-frame-latency-20261124` 分支进行。原生报表脚本、CSS、网格折叠求解器、阴影和纹理输出分辨率沿用基线。

上一轮 CDP Trace 中，冷启动纸材生成约 617 ms，Worker 位图读回约 332 ms。稳态 DOM 解析中位数 84.4 ms，Worker `GLES2::ReadPixels` 中位数 84.3 ms。本轮先用原基准脚本复跑 3 次冷启动和 50 次筛选作为同机对照。

## 保留的优化

- Worker 初始化时预计算扫描纸材的纤维场、法线和粗糙度，按色值生成反照率。两种主题共用法线和粗糙度，初始化可与 DOM 捕获并行。像素单测对照原白纸材质流程。
- 报表内容、捕获样式、视口和内部滚动位置一致时，命中无损 PNG 油墨缓存，跳过整页 DOM 捕获。首次出现的状态仍解析变化区域。热力面板的 markup、捕获环境、位置和滚动状态相同时，复用该区域的布局和字形解析结果，最终由原画笔按原堆叠顺序绘制。
- 静止纸面和油墨独立复用。Worker 在确认当前网格和油墨已准备完毕后，可提交同尺寸静止纸面。对无折痕、同纸色、同宽度、整数 DPR 的短报表，从较高纸面的底部取对应像素。原 UV 的竖直相位以底边为准，裁切保持原采样间距。折痕、较大页面、不同色值和分数缩放走正常 WebGL 渲染。
- PNG 缓存最多 12 项或 8 MiB。待编码输入最多 2 项或 48 MiB。只有已编码且仍在缓存的 ID 才可用于下一次提交。主线程静止纸面缓存上限为 6 项或 48 MiB，淘汰时释放 Canvas；切离主题、Worker 失效时清除缓存。活跃折叠继续使用版本化 ImageBitmap，原有逐帧确认与回收路径保留。

## 放弃的直接呈现

独立探针用 1440 × 1635 WebGL2 表面交替绘制红、绿、蓝，每种浏览器各测试 12 帧。与 ImageBitmap 对照相比，双 placeholder 缩短了 Worker 回复时间。Chromium 却在两个缓冲各自首次呈现时读到白色，已完成的 GL 命令没有保证新的 placeholder 像素已经合成。这个方案留作探针，不接入候选。 [HTML OffscreenCanvas 更新时序](https://html.spec.whatwg.org/multipage/canvas.html#the-offscreencanvas-interface)

| 探针 | ImageBitmap 回复中位数 | 双 placeholder 回复中位数 | 双 placeholder 错帧 |
| --- | ---: | ---: | ---: |
| Chromium | 21.9 ms | 0.1 ms | 2 / 12 |
| Firefox | 16 ms | 2 ms | 0 / 12 |
| WebKit | 4 ms | 约 0 ms | 0 / 12 |

探针使用纯色帧，数值只描述呈现机制，整页应用延迟以相同基准脚本的结果为准。

## 配对结果

Apple M4，Playwright 1.55.0 Chromium，ANGLE SwiftShader，1440 × 900，DPR 1。对照与候选连续运行，沿用上一轮 Playwright 点击到物理 `snapshotReady` 的墙钟口径。每组 3 个新浏览器冷启动和 50 次正式筛选，正式样本包含首次出现的报表状态。

| 指标 | b68b403 | 候选 |
| --- | ---: | ---: |
| 冷启动物理帧墙钟 | 1,261 / 1,327 / 1,223 ms | 986 / 1,064 / 1,115 ms |
| 50 次筛选物理帧墙钟 p95 | 249 ms | **135 ms** |
| 50 次筛选物理帧墙钟中位数 / 最大值 | 232 / 283 ms | 82 / 165 ms |
| 第 3–10 轮物理帧墙钟 p95 | 249 ms | 99 ms |
| 点击 Event Timing p95 / 最大值 | 32 / 64 ms | 32 / 64 ms |
| 筛选期主线程 Long Tasks | 0 | 0 |
| 冷启动纸材计算 | 308–359 ms | 99–129 ms 预计算，另有 29–34 ms 着色 |
| 冷启动 Worker 位图读回 | 170–181 ms | 176–180 ms |

性能随运行环境波动。本轮较早的对照 p95 为 414 ms，最终候选另一次运行 p95 为 133 ms。以连续配对的 249 → 135 ms 作为收益比较。冷启动仍需生成第一个完整位图，读回阶段没有缩短。

配对候选在 50 次中复用 44 次油墨、50 次纸面，其中 3 次用底边裁切。652 个帧样本中版本、网格与纹理尺寸失配为 0。缓存观测峰值为 1.86 MiB PNG、42.23 MiB 静止画布估算和 10.70 MiB 待编码输入。具体估算口径为宽 × 高 × 4，浏览器进程总 RSS 未计入。DPR 2 的单张画布占用较高，缓存可保留的不同尺寸更少。

## 画面与回归

9 组完整静止纸面像素对照覆盖浅色、深色、DPR 1 和 DPR 2。全尺寸直接绘制像素相同，裁切输出的 RGB 最大差为 1 个级别。实际缓存油墨上传前的像素复查中，Chromium 和 WebKit 逐字节相同，Firefox 的 alpha 相同，预乘色差最大 1.94 个级别。1440 浅色、1920 浅色和 1440 深色的折痕对照强度与 `b68b403` 相同，折线外区域没有超过 3 个级别的变化。

macOS Chromium、Firefox、WebKit 共 162 项通过。Ubuntu 24.04 arm64 共 153 项通过，Linux WebKit 无 Worker WebGL2，9 个物理用例按能力跳过。Ubuntu 重新构建的资源逐字节一致。Node 单测 2 项、`go test ./...`、`go vet ./...` 和 Python Pages 7 项通过。GitHub 托管 runner 与生产 CDN 未执行候选验证。

证据位于 `evidence/round2`。其中 `paired-control` 与 `paired-candidate` 各含冷启动、筛选 Trace、原始测量和录像。逐帧图为 `filmstrip.png`，视觉矩阵在 `verified/stock`、`verified/scars` 与 `verified/ink-*`。直接呈现探针保存在 `presentation-probe`。汇总在 `comparison.json`。

## 复现

```bash
npm --prefix profile-paper run build
node --test profile-paper/test/*.test.js
node profile-e2e/tools/newsprint-render-trace.mjs handoff/newsprint/evidence/round2/paired-candidate
node profile-e2e/tools/newsprint-presentation-probe.mjs handoff/newsprint/evidence/round2/presentation-probe
WT_CAPTURE_STOCK_MATRIX=1 node profile-e2e/capture-review.mjs handoff/newsprint/evidence/round2/verified/stock
WT_CAPTURE_INK_PARITY=1 WT_CAPTURE_ENGINE=firefox node profile-e2e/capture-review.mjs handoff/newsprint/evidence/round2/verified/ink-firefox
npm --prefix profile-e2e test
WT_LINUX_DESKTOP=1 xvfb-run -a npm --prefix profile-e2e test
```

要对照 `b68b403`，将 `git archive b68b403 internal/profilewebembed/static` 解包到临时目录，再用 `WT_PROFILE_STATIC` 指向提取出的 `static`。实验分支未推送，生产部署未改动。
