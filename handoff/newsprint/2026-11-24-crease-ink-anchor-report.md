# Newsprint 第三轮修复验收

- 实现提交 `658d8f5bc44d2b6962a551e8a9437aead63a23a0`，分支 `codex/newsprint-crease-ink-20261124`。
- [本机候选预览](http://127.0.0.1:8787/whereToken/profile/?palette=newsprint&range=53w) · [本机 2263064 对照](http://127.0.0.1:8788/whereToken/profile/?palette=newsprint&range=53w)。
- `2263064` 保留为实验回滚点。任务书记录的生产发布点为 `b68b403`。候选只提交到本机分支。

在 1440 px 预览里，从左上角内约 12 px 向右下拖约 226 px。纸页锁定后，从折起的尖角向左上拖回，数字 `21.53B` 将穿过残痕。可继续切换 53w、All、7d、30d，并用顶部的浅色、深色按钮比较。

## 油墨和纸面的受光

`2263064` 平铺时用 GPU 绘制纸材，着色器将正面油墨置零。原生文字单独盖在纸面上。在 Chromium 中以同一 v1 残痕复现，浅色 DPR 1 的近景有 1,137 个纸面像素明暗改变，946 个实心字形样本全部保持原值。

评估了整页 GPU 印刷和原生内容受同一法线光照两条路径。整页 GPU 原型在平铺时启用现有油墨图集，由折痕网格直接给印刷着色。原型证明折痕可以穿过图集字形。它每次油墨变化需要提交一张整页光学位图，焦点、选区和原生状态变化还需要与图集显示权同步。该对照裁图保存在 [GPU 原型](evidence/round3/gpu-prototype/1440-light-dpr1-scar-ink.png)。

候选在平铺时保留原生文字和控件。Worker 在同一材质着色器内，用纤维法线、残痕网格法线、纸色和光源计算当前反射相对于平面参考的亮度增益。两张稀疏位图将小于 1 的增益和大于 1 的增益分别合成到已绘制的纸材与字形上，暗部按 (1-\alpha) 衰减，亮部通过 color dodge 实现等效的亮度放大。这样同一光照能经过纸纤维和实心笔画。初版白色透明叠加使浅色深墨亮度变化一度达到 83 个级别，比例增益将其降到 2。遮片保持指针穿透。真实拖选得到 `21.53B`，键盘和即时筛选继续走原生入口。弯曲时，原图集沿网格 UV 变形。

## 坐标和存储

折痕记录抓取时的左、右、上、下锚边及到边缘的 CSS px 偏移。角折使用真实角的两条边，边折沿抓取边并固定抓取时距相邻边的距离。尺寸变化时平移直线原点，保持法向、深度、强度和沿线纤维相位。直线与新矩形精确求交，重算端点和可见长度。完全位于纸外的痕迹保留在会话记录，纸张恢复到足够尺寸时重新可见。

旧 `sessionStorage` 数据读取时，从原来的角、法向、原点与保存尺寸推导锚点。新折痕写入同一个键，其中 `anchored.version = 2` 保存权威记录，外层保留旧程序可读的当前可见投影。同一标签页刷新保留折痕，切离 Newsprint 清除存储。

真实手势的 10 轮包含 50 次点击。基线坐标由保存的 v1 线段及各帧实际纸张尺寸按该版本公式重放，候选坐标来自 Worker。下表为折痕与左边缘交点，其中顶边交点在两版均为 `(362.54, 0)`。

| 筛选或布局 | 纸高 CSS px | 2263064 左端 y | 候选左端 y |
| --- | ---: | ---: | ---: |
| 53w | 1635 | 362.54 | 362.54 |
| All | 1635 | 362.54 | 362.54 |
| 7d | 1286 | 285.15 | 362.54 |
| 30d | 1286 | 285.15 | 362.54 |
| 返回 53w | 1635 | 362.54 | 362.54 |
| 展开 Technical details | 2232 | 494.91 | 362.54 |

十轮内最大端点误差为 **77.39 → 0.00 CSS px**。快速连续切换、滚动、1280 px / DPR 1.25、刷新恢复和切离清除均验证通过。坐标单测覆盖上、下、左、右，深折与回弹、暂时出界后回归，以及多痕迹达到九条上限时的淘汰一致性。[基线坐标](evidence/round3/control-fold/coordinates.json) · [候选坐标与逐帧状态](evidence/round3/candidate-fold/coordinates.json) · [基线红测](evidence/round3/control-coordinates-red.log)。

## 视觉证据与纤维相位

[同笔画对照](evidence/round3/ink-comparison.png) · [真实拖折、锁定、展开逐帧图](evidence/round3/gesture-filmstrip.png) · [展开后整页](evidence/round3/candidate-fold/gesture-released-full.png) · [局部原图](evidence/round3/candidate-fold/gesture-released-local.png)。

| Chromium 近景 | 改变的实心字形像素，基线 → 候选 | 候选字形最大正向亮度变化 | 纸面最大 RGB 变化，基线 → 候选 |
| --- | ---: | ---: | ---: |
| 1440 浅色 DPR 1 | 0 → 242 | 1.0 | 19 → 19 |
| 1440 深色 DPR 2 | 0 → 3609 | 16.3 | 3 → 3 |
| 1920 浅色 DPR 2 | 0 → 878 | 1.0 | 19 → 19 |
| 1920 深色 DPR 1 | 0 → 864 | 16.5 | 3 → 3 |

四组实验的折痕外近景均没有超过 4 个 RGB 级别的变化。残痕曲线、增益和深度系数沿用基线。平面材质的纤维相位从底边改为左上角，同一物理位置在纸高切换时保持采样相位。与旧版全页未折截图相比，这个相位变化使四组全页 RGB 平均绝对差在 0.25–1.92 个级别。设备 DPR 2 的 1920×1635 样本受原 1200 万像素上限约束，有效油墨比例约 1.955。

纸材缓存只存平面材质，按尺寸、缩放比例和纸色索引。整数 DPR 的缩短纸张从较高纸张顶边裁切，与高纸材采样逐字节相同。与直接渲染的短纸材相比，RGB 最大差为 1。不同缩放比例走独立渲染。折痕光照缓存另核对同样的尺寸和折痕版本。Firefox 保证裁切先写入实际尺寸的缓冲区，再交给位图画布，避免裁切结果携带源尺寸。[纤维抽样验证](evidence/round3/candidate-stock/stock-phase.json) · [候选像素数据](evidence/round3/candidate-visual/visual.json) · [基线像素数据](evidence/round3/control-visual/visual.json)。

## 同机性能对照

Apple M4，macOS，Playwright 1.55.0 Chromium，ANGLE SwiftShader，1440×900，DPR 1。每次运行包含 3 个冷启动及 50 次筛选，按旧基准记录 Playwright 点击开始到物理帧提交的墙钟时间。原生列使用 Chromium Event Timing。带折痕组从同一条 v1 物理残痕开始，顺序、快照、浏览器和采样方法一致。

| 独立配对运行 | 原生点击 p95，基线 → 候选 | 物理帧 p95，基线 → 候选 | 第 3–10 轮物理帧 p95，基线 → 候选 |
| --- | ---: | ---: | ---: |
| 无折痕，先基线 | 48 → 48 ms | 197 → 222 ms | 99 → 148 ms |
| 无折痕，先候选 | 56 → 48 ms | 197 → 197 ms | 103 → 125 ms |
| 带折痕，先基线 | 48 → 48 ms | 1031 → 548 ms | 198 → 99 ms |

无折痕候选的物理帧中位数为 96–97 ms，基线为 97–97.5 ms。两次候选均未复现上一轮报告中的 135 ms 全样本 p95，本机对照同样为 197 ms。第一对全样本 p95 增加 25 ms，交换运行顺序后相同。带折痕时，候选缩短了首次重算耗时并减少反复回切的纸材读回。候选的点击到下一次动画回调 p95 为 25.1–28.1 ms，三次运行的筛选期主线程长任务分别为 0、1、1 次，后两次为 52 ms 和 51 ms。

三个候选 trace 的 2,398 个采样帧中，物理帧版本和网格、油墨尺寸失配为 0。真实拖折后十轮回切时，热切换采样帧均保留光照。首次遇到 1286 px 高度的手势样本有约 **617 ms** 显示当前原生内容及备用纸材，直到精确尺寸的折痕提交。这是仍可观察的冷缓存边界，需一并纳入视觉验收。

50 次无折痕筛选的油墨命中 44 次，平面纸材复用 50 次，含 3 次顶边裁切。带折痕的候选保留同样的油墨与纸材命中，并复用光照 47 次。真实手势连切组复用光照 49/50 次。带折痕 trace 的缓存峰值为 1.86 MiB PNG 油墨、42.23 MiB 平面画布估算、5.71 MiB 光照画布估算、10.70 MiB 待编码输入。光照缓存限制为 6 项 / 16 MiB。换主题、比例或尺寸时按精确键选择资源。新折痕递增光照版本，离开 Newsprint 时释放缓存。字节数基于位图和 canvas 容量，浏览器进程总 RSS 未计入。

[全部配对指标](evidence/round3/comparison.json) · [基线冷启动 trace](evidence/round3/control-perf/after-cold.trace.json.gz) · [候选冷启动 trace](evidence/round3/candidate-perf/after-cold.trace.json.gz) · [基线筛选 trace](evidence/round3/control-perf/after-filters.trace.json.gz) · [候选筛选 trace](evidence/round3/candidate-perf/after-filters.trace.json.gz) · [基线带折痕 trace](evidence/round3/control-scar-perf/after-filters.trace.json.gz) · [候选带折痕 trace](evidence/round3/candidate-scar-perf/after-filters.trace.json.gz) · [交换顺序的基线 trace](evidence/round3/control-reverse-perf/after-filters.trace.json.gz) · [交换顺序的候选 trace](evidence/round3/candidate-reverse-perf/after-filters.trace.json.gz)。

## 回归验证

- Playwright Chromium、Firefox、WebKit 共 162 项通过，覆盖原生筛选、键盘、回弹与深折、重抓、滚动到底角、Worker 失败与降级。另做 Chromium 真实手势、像素、DPR 和纤维相位验证。[浏览器日志](evidence/round3/playwright-final.log)
- Node 6 项、Go 全量测试、Go vet、Python Pages 测试 7 项通过。嵌入资源与两份本地生成包同步，两个 `profile.json` 逐字节沿用基线。[Node](evidence/round3/node-test.log) · [Go](evidence/round3/go-test.log) · [Vet](evidence/round3/go-vet.log) · [Pages](evidence/round3/pages.log)

### 本机复现

```sh
npm --prefix profile-paper run build
node --test profile-paper/test/*.test.js
npm --prefix profile-e2e test
go test ./...
go vet ./...
python3 -m unittest -v scripts/profile_pages_test.py

WT_PREVIEW_PORT=8787 node profile-e2e/tools/newsprint-crease-evidence.mjs --serve
node profile-e2e/tools/newsprint-crease-evidence.mjs handoff/newsprint/evidence/round3/candidate-visual
node profile-e2e/tools/newsprint-crease-evidence.mjs handoff/newsprint/evidence/round3/candidate-fold --fold
node profile-e2e/tools/newsprint-crease-evidence.mjs handoff/newsprint/evidence/round3/candidate-stock --stock
node profile-e2e/tools/newsprint-render-trace.mjs handoff/newsprint/evidence/round3/candidate-perf
WT_TRACE_SCAR=1 node profile-e2e/tools/newsprint-render-trace.mjs handoff/newsprint/evidence/round3/candidate-scar-perf
```

对照可以将 `git archive 2263064 internal/profilewebembed/static` 解包到临时目录，再将 `WT_PROFILE_STATIC` 指向解包后的 `static`。视觉基线使用 `WT_OBSERVE_ONLY=1` 保存已知失败的截图和像素计数。原始 PNG、录像、trace 和日志保留在本机交接目录。关键图、坐标与性能摘要随报告提交。
