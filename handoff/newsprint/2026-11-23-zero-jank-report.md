# Newsprint 渲染修复

## 基线与设计决定

Apple M4，Playwright Chromium，1440 × 900，仓库已发布的脱敏快照。复跑交接脚本得到冷启动 6,739 ms、All → 7d 3,695 ms。冷启动最长任务 2,006 ms，筛选最长任务 1,496 ms。原脚本固定写入基线 JSON，本次复跑覆盖了该文件。交接记录仍保有原始两项墙钟值 6,671 / 3,679 ms。

CDP trace 中，旧版两轮捕获分别耗时 235 / 178 ms。其中单个 DOM 克隆任务约 100–126 ms。材料阶段接近 2,000 / 1,470 ms。首个 1,904 ms WebGL 任务中有 1,425 ms 等待 GPU 命令完成。另有独立的 1,871 / 902 / 925 ms 合成任务调用 `GLES2::ReadPixels`。CDP 能看到纹理和顶点上传，以及 ANGLE 编译工作。trace 中最长的 ANGLE 编译工作约 22 ms，纹理上传 API 调用最长约 28 ms。编译、首次绘制和命令等待部分交叠，API 调用时长和读回阻塞分别记账。尺寸失配也可独立复现：`updateDocument()` 先把 1,635px 网格改成 1,286px，`queueCapture()` 随后才替换印刷内容。

主线程仅管理报告与交互，并分段读取浏览器布局。Worker 持有材质、网格、印刷纹理和 WebGL 上下文。它绘制到独立 OffscreenCanvas，完成后传回可转移的位图。主线程在同一 paint 前验证版本与尺寸并换帧。平放时用原生字体绘制静态文字，整页背后仍使用物理纸面。折起时使用同一版本的全页 UV 印刷纹理，让文字跟随纸面。平放位图随文档原生滚动，滚动无需等 Worker 调相机。折痕以独立状态保存，几何和剪裁阴影随折叠阶段更新。

基线证据位于 `evidence/before`。完整时间线为 `before.trace.json.gz`，可解压后导入 Chromium Performance。


## 最终性能实测

Apple M4，Playwright 1.55.0 Chromium，ANGLE SwiftShader，1440 × 900，DPR 1。三次冷启动各使用新的浏览器进程。连续十轮共 50 次正式切换，另有 18 次轮间复位和 3 次快速点击。Event Timing 的点击样本共 71 次。Playwright 的调用墙钟含定位、操作前检查和跨进程往返。

| 指标 | 基线 | 最终 |
| --- | ---: | ---: |
| 冷启动物理帧墙钟 | 6,739 ms | 2,630 / 2,329 / 2,329 ms |
| 首个有数据的原生帧 | — | 87.8 / 33.9 / 36.4 ms |
| 冷启动最长主线程任务 | 2,006 ms | 54 / 56 / 54 ms |
| All → 7d 物理帧墙钟 | 3,695 ms | 中位 366 ms，最大 460 ms |
| 切换点击 Event Timing | 基线 trace 的 7d 点击 1,052 ms | p95 48 ms，最大 104 ms |
| 点击时间戳至原生事件分发 | — | p95 2.2 ms，最大 6.1 ms |
| 点击至下一次原生帧回调 | — | p95 25.4 ms，最大 70.7 ms |
| Playwright 点击调用墙钟 | — | p95 194 ms，最大 222 ms |
| 全部 50 次物理帧切换墙钟 | — | 中位 365 ms，p95 419 ms |
| 筛选期间主线程 Long Tasks | 最长 1,496 ms | 1 个，50 ms |

最终冷启动的整页 DOM 捕获为 203–241 ms 墙钟，克隆、解析与绘制用可取消的小任务完成。首次纸材生成 594–617 ms、几何 1 ms、着色器编译与预热 34–40 ms、纹理上传 16–21 ms、平面绘制调用 7 ms、位图出帧 324–332 ms。这些 WebGL 和材料阶段均在 Worker 内。稳态捕获 p95 为 160 ms，复用纸材和着色器，上传 p95 11 ms，绘制调用 p95 1 ms，位图出帧 p95 108 ms。SwiftShader 的读回和传输仍会延后物理帧。

快速 7d → 30d → 7d 的三次鼠标输入墙钟为 232 ms，最终 7d 物理帧在序列开始后 600 ms 提交。1,713 个帧样本里的可见物理帧均匹配 DOM、网格、纹理尺寸和 revision。截图序列覆盖首屏和改变高度的切换。平放用原生文字以维持原有字形和焦点。纸翘起后，同版本的整页印刷纹理沿原始 UV 弯曲。折叠阴影贴图为 1,024 × 1,024，印刷纹理及输出仍按 DPR 和原有像素预算计算。

## 折痕和画面

以同样的折角拖动生成基线与候选截图。在折线中间 80% 的 81 个位置上，取垂直截面内平放图到回弹图的最大灰度下降，再取均值。候选折痕为旧版的 46.4%（1440 浅色）、49.2%（1920 浅色）、41.2%（1440 深色）。针对旧投影，在三个角落区域各比较 58,260 个折线外像素，灰度变化超过 3 个级别的像素数均为 0。沿两侧各 30px 的截面，最大差为 1 个灰度级。

折痕经过范围切换、指标切换、报表增高及缩短、同标签页刷新后仍在。离开 Newsprint 后清除，包括减少动态效果时引擎没有启动的情况。首屏控件在网络快照返回前可用，测试阻断了该响应并验证范围选择、键盘焦点和加载后的准确数值。

本地证据在 `evidence/verified`，包括 `measurements.json`、`summary.json`、两份 CDP trace、`filmstrip.png`、三段 WebM 和三组原尺寸折痕对照截图。基线运行、录像和截图在 `evidence/before`。

## 历史 CI 复现

基线提交的 Ubuntu 24.04 运行记录有三个失败。Firefox 物理用例等了 30 s。在同版 Firefox 的 Linux 探针中，无头模式得到 `pointer:fine=false`，Xvfb 桌面模式得到 `pointer:fine=true` 且可创建 Worker WebGL2。WebKit 的旧报表高度断言在 16 s 后仍得到 1,146px。新实现将静止 Canvas 裁在报表容器内，并覆盖增高和缩短两个方向。Chromium 的 Tooltip 超时另有可复现的计时器竞争，晚到的其他单元格离开事件能取消当前 Tooltip 的关闭。本分支加入了确定性回归。 [历史 Linux CI](https://github.com/rainhuang0220/whereToken/actions/runs/45181795201/job/137717174364)

Playwright WebKit 在 Linux 容器内能创建主线程 WebGL2，但两种运行模式都无法创建 Worker WebGL2。因此按原生 HTML 回退。在 macOS 的 WebKit 上跑物理用例。原有短折动作的最终目标角度接近 118° 快速锁定阈值，WebKit 的采样频率能触发锁定。测试用较短拖动测回弹，仍用 >150° 的动作测深折和重抓。

## 复现命令

```bash
node profile-e2e/tools/newsprint-render-trace.mjs handoff/newsprint/evidence/verified
WT_CAPTURE_SCAR_MATRIX=1 node profile-e2e/capture-review.mjs handoff/newsprint/evidence/verified/scars
python3 profile-e2e/tools/newsprint-evidence.py handoff/newsprint/evidence/verified \
  --scars-before handoff/newsprint/evidence/before/scars \
  --scars-after handoff/newsprint/evidence/verified/scars
npm --prefix profile-e2e test
WT_LINUX_DESKTOP=1 xvfb-run -a npm --prefix profile-e2e test
go test ./...
go vet ./...
python3 scripts/profile_pages_test.py
```

对照图的分析脚本依赖 Pillow。基线复现可用 `git archive 45dfff9 internal/profilewebembed/static` 提取静态文件，设置 `WT_PROFILE_STATIC` 指向提取的 `static`，然后给 trace 命令加 `--baseline`。所有浏览器用例使用仓库的已脱敏公开或合成快照。

## 验收与发布状态

| 项目 | 结果 |
| --- | --- |
| P0 原生交互、主线程预算和物理帧一致性 | 通过。点击 p95 48 ms，筛选最长任务 50 ms，尺寸与版本失配 0 |
| P1 整页折叠、残痕、键盘、Tooltip、深浅模式和移动端回退 | 通过。三组照片的折痕对比度均在 40%–60% 内 |
| macOS Chromium / Firefox / WebKit | 159 通过 |
| Ubuntu 24.04 arm64 Chromium / Firefox / WebKit | 151 通过。WebKit 的 8 个物理用例因 Worker WebGL2 不可用而跳过，原生用例通过 |
| 构建与静态资产 | Linux 重建结果逐字节一致。嵌入、公开站、演示站、本地 HTTP 的 HTML、CSS、控制器和 Worker SHA-256 一致 |
| Go / Pages | `go test ./...`、`go vet ./...`、Python Pages 的 7 个用例、本地镜像读取均通过 |

浏览器的完整日志在 `evidence/macos/verified-playwright.log` 与 `evidence/linux/verified-playwright.log`。Ubuntu 本地矩阵使用与 CI 同版本的 Playwright Noble 容器，GitHub 托管的 x64 runner 和生产 CDN 尚未执行候选验证。本地预览 [Newsprint](http://127.0.0.1:8786/?palette=newsprint) 已验证 All → 7d 的物理与数据版本。预览可用 `python3 -m http.server 8786 --bind 127.0.0.1 --directory public-profile` 重启。

候选仅保留在功能分支。回滚此提交会同时还原源码、三个静态资产副本和 CI 入口。两份 `profile.json` 均通过字节比对。
