# Newsprint 跨筛选组缓存修复

[本机候选](http://127.0.0.1:8790/whereToken/profile/?palette=newsprint&range=53w) · [第三轮对照](http://127.0.0.1:8791/whereToken/profile/?palette=newsprint&range=53w) · [根因报告](2026-11-24-cross-range-cache-rootcause.md)

候选分支为 `codex/newsprint-cross-range-cache-20261124`，实现提交为 `48ddcc3f0462816877b2ea864dd4d7153a78566f`，以 `d7fe505` 为对照。候选保留在本机分支，由用户验收。任务书记录的生产版本为 `b68b403`，上轮实验回滚点为 `2263064`。

## 实现

- 相同纸宽、纸色、材质版本和实际像素间距共享从左上角起算的图集。较短报表只改变 CSS 裁剪窗口。纸面增长时，Worker 仅绘制新增底部行，并将它们追加为等比例条带。宽度、DPR 或纸色改变时重新校验像素间距及材质键，无法精确复用时重新渲染。
- 光照按折痕的文档坐标、裁切线段、采样间距、纸色和版本索引。尺寸改变时，上角可沿用同一光照区域，已移动的下角或被重新裁切的边折生成新区域。相交区域合并，分离区域独立缓存。局部网格以原物理凹痕求解器采样，阴影与高光由一次 GPU 读取生成。
- 平铺提交校验当前油墨栅格和尺寸。折叠用 GPU 纹理在角落按下时上传，绘制折页前再次确保当前纹理已就绪。稳定态仍由原生文字、控件、选区和物理光照共同显示。Worker 继续负责整页几何、光照与活跃折叠。
- 常驻纸材上限仍为 48 MiB，光照为 16 MiB。图集按宽度与总行数计费，当前可见但超出光照缓存的局部位图只保留到下一次切换。若遇到多次连续增高导致超过八条条带，下一次增长重建该组图集。分数缩放只有在像素间距一致时才共享。

## 像素与锚点

DPR 2 的 Today → All 从 2880×2406 增长到 2880×3270，只读回 864 行，共 2,488,320 像素，占完整图集的 26.4%。追加接缝附近与独立整页直出的 RGB 最大差为 1。整数 DPR 裁剪直接沿用图集顶部像素，DPR 2 的 1920 px 样本因达到原 1200 万像素上限，在像素间距改变时生成独立图集。[增长采样](evidence/round4/growth/stock-growth.json) · [纤维相位采样](evidence/round4/stock-phase/stock-phase.json)

浅色与深色、DPR 1/2、1440/1920 px 的近景仍可看到折痕穿过数字笔画。真实拖动、锁定、展开、滚动及十轮回切验证中，锚点误差在采样精度内为 0 CSS px。Firefox 的正文可能带分数高度，裁剪窗口以实际排版边界为限，版本和背缓冲尺寸继续按整数逻辑纸高核对。[局部截图矩阵](evidence/round4/ink-matrix.png) · [手势坐标](evidence/round4/gesture/coordinates.json)

## 同机性能

Apple M4、Chromium Playwright、ANGLE SwiftShader、1440×900、DPR 2。每组 55 次真实页面筛选操作，按用户指定的 53w、All、7d、30d、Today 跨组路径运行。排除首次暖缓存的一轮后，包括 44 次热切换和 32 次热跨纸高切换。用同一页 `click()` 调用到精确版本提交统计物理耗时。原生下一帧以浏览器动画回调时间为准，合成点击不产生可信指针 Event Timing。

| 场景 | 热跨纸高物理 p95 | 热跨纸高最大值 | 热切换整页纸材读回 |
| --- | ---: | ---: | ---: |
| 第三轮，无折痕 | 1010.3 ms | 1029.3 ms | 16/44 |
| 候选，无折痕 | 82.2 ms | 96.2 ms | 0/44 |
| 第三轮，上角折痕 | 1023.6 ms | 1030.8 ms | 16/44 |
| 候选，上角折痕 | 68.1 ms | 82.3 ms | 0/44 |
| 候选，下角折痕 | 65.6 ms | 69.3 ms | 0/44 |
| 候选，上下角双折痕 | 69.2 ms | 82.1 ms | 0/44 |

候选的原生下一帧 p95 为 14.6–14.7 ms，基线为 14.5 ms。四组候选热切换均在第一帧保留有效纸底，热跨组全部低于 150 ms，纸材和局部光照读回均为零。另从 Today 冷启动验证两段图集增长，热跨组 p95 为 81.7 ms。候选五组共采样 1674 帧，提交版本、网格与当前逻辑尺寸的失配为 0。

另用真实鼠标点击逐对验证 Today ↔ All、Today ↔ 30d 和 7d ↔ 53w。带上角折痕的基线与候选各有 36 次点击。Chromium 点击 Event Timing p95 均为 **56 ms**，热跨高度的原生下一帧 p95 均为 **15 ms**。物理帧 p95 为 **776.2 → 82.9 ms**，热切换重读整页纸材为 **12/24 → 0/24**。[鼠标比较数据](evidence/round4/pointer-comparison.json) · [候选鼠标 trace](evidence/round4/pointer-candidate/after-filters.trace.json.gz)

首次进入 7d / Today 的上角折痕场景，第三轮分别为 716 / 691 ms，候选为 148 / 131 ms，光照读回为零。下角首次移位时，局部光照读取耗时 48 / 49 ms，包括新油墨捕获在内的整次提交为 218 / 203 ms。双折痕场景的局部读取为 22 / 24 ms，同一批次复用上角光照。

两版纸材峰值均为 35.93 MiB。基线上角光照占 13.69 MiB，候选上角占 4.56 MiB，下角占 13.68 MiB，双折痕占 8.51 MiB。候选 PNG 油墨缓存峰值为 2.68 MiB，待编码输入峰值为 35.93 MiB。这些数字是缓存可见的位图容量。超预算但仍在展示的局部片和浏览器进程总 RSS 独立于缓存上限。热态 trace 中观察到的基线超 150 ms 样本全部关联纸材读回，已在原始数据中逐次保留。

[逐组数字与异常样本](evidence/round4/comparison.json) · [基线 trace](evidence/round4/control-top/after-filters.trace.json.gz) · [候选带折痕 trace](evidence/round4/final-top/after-filters.trace.json.gz) · [双折痕 trace](evidence/round4/final-both/after-filters.trace.json.gz) · [双折痕整页截图](evidence/round4/final-both/rest-1.png) · [DPR 2 追加后的整页截图](evidence/round4/growth/dpr2-grown-stock.png)

## 测试结果

- Chromium、Firefox、WebKit 完整 Playwright 回归 162 项通过。DPR 2 五组轨迹的结构校验、150 ms 门槛、增长接缝像素和光照矩阵已分别核对。[浏览器日志](evidence/round4/playwright-final.log)
- Node 12 项、Go 全量测试、Go vet、Python Pages 7 项通过。两份生成包与嵌入资源同步，Profile 快照保持相同字节。[Node](evidence/round4/node.log) · [Go](evidence/round4/go-test.log) · [Vet](evidence/round4/go-vet.log) · [Pages](evidence/round4/pages.log)

## 本机复现

```sh
npm --prefix profile-paper run build
node --test profile-paper/test/*.test.js
npm --prefix profile-e2e test
go test ./...
go vet ./...
python3 -m unittest -v scripts/profile_pages_test.py
WT_PREVIEW_PORT=8790 node profile-e2e/tools/newsprint-crease-evidence.mjs --serve
WT_TRACE_RANGES=1 WT_TRACE_ASSERT_LATENCY=1 node profile-e2e/tools/newsprint-render-trace.mjs handoff/newsprint/evidence/round4/final-flat
WT_TRACE_RANGES=1 WT_TRACE_SCAR=1 WT_TRACE_ASSERT_LATENCY=1 node profile-e2e/tools/newsprint-render-trace.mjs handoff/newsprint/evidence/round4/final-top
WT_TRACE_RANGES=1 WT_TRACE_SCAR=both WT_TRACE_ASSERT_LATENCY=1 node profile-e2e/tools/newsprint-render-trace.mjs handoff/newsprint/evidence/round4/final-both
WT_TRACE_RANGES=1 WT_TRACE_START=Today WT_TRACE_ROUNDS=2 node profile-e2e/tools/newsprint-render-trace.mjs handoff/newsprint/evidence/round4/final-growth
WT_TRACE_RANGES=1 WT_TRACE_POINTER=1 WT_TRACE_SCAR=1 WT_TRACE_ROUNDS=3 WT_TRACE_ASSERT_LATENCY=1 node profile-e2e/tools/newsprint-render-trace.mjs handoff/newsprint/evidence/round4/pointer-candidate
node profile-e2e/tools/newsprint-crease-evidence.mjs handoff/newsprint/evidence/round4/growth --stock-growth
node profile-e2e/tools/newsprint-crease-evidence.mjs handoff/newsprint/evidence/round4/visual
```

对照使用 `git archive d7fe505 internal/profilewebembed/static` 解包到临时目录，将 `WT_PROFILE_STATIC` 指向该 `static`。设 `WT_TRACE_OBSERVE=1` 可保留基线失败数据及 trace。红测对照在暖缓存后首个 11 次循环仍触发 4 次纸材读回。[红测日志](evidence/round4/red.log)
