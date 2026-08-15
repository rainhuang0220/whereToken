# 釉厅 4×2 + 展开预览

Date: 2026-08-16
Status: 实现中（任务书已锁死布局、删釉、动效；本文件记下两枚新釉与 FLIP 回路）
Companion: [`2026-08-15-wheretoken-calendar-design.md`](./2026-08-15-wheretoken-calendar-design.md)

---

## 1. 问题

`/themes` 把 8 块釉铺成色卡托盘。首页是观察台；釉厅应像选一件仪器皮肤，而不是换 swatch。

成功标准：

1. 正好 **8** 块釉，桌面 **一行 4、共两行**，卡片比色卡大，多出来的面积给 1–3 句中文，不是空白。
2. 点卡片：其余淡出，**同一张卡** FLIP 长成整页；整页是「这釉下的首页」而不是色条。
3. **应用** 才写 `wheretoken.theme` 并回 `/`。只展开不持久化。
4. 回网格走反向：预览收掉，卡回到格子，另外七张淡入。
5. 窑墙本身仍是平 2D（无阴影、无 3D、无 glow）。釉可以改圆角、字体、砖形。
6. `prefers-reduced-motion: reduce`：跳过 Flip/淡入，瞬时换布局，不抛错。

---

## 2. 八釉

保留六枚，重写说明书。删 **青墨 `qingmo`**、**霜/霜碳 `frost`**。`localStorage` 若是这两枚（或未知 id）→ `kiln`。

| id | 印 | 说明书（作者口吻，1–3 短句） |
|----|----|------------------------------|
| kiln | 窑 | token 烧得快，像窑。焦黄到炭黑：速度、热、动。 |
| moss | 苔 | 清新，又复古一点的现代。青苔贴在石头上，绿不必吵。 |
| porcelain | 瓷 | 青花瓷。白地、钴料，砖从素坯走到近墨。 |
| jiang | 绛 | 粉和黑。夜里的现代，口红那种亮，不是糖果。 |
| day | 昼 | 蓝白黑。产品界面的现代，不是霓虹。 |
| ink | 墨 | 黑白。印成报纸也长这样。作者最喜欢的配色——没别的好说。 |
| cartoon | 漫 | 圆砖、粗字。观察台也可以玩一下，合计还是要能读。 |
| ledger | 端 | 终端账本。等宽、直角、磷光。墙仍是平的。 |

`qingmo` / `frost` 不得出现在 `THEME_IDS`、`themes[]`、boot `allowed`、CSS、测试期望、README。

---

## 3. 两枚新釉（系统，不是换色）

不是玻璃拟态。每套必须改 **字体 + 圆角 + 砖几何 + 控件外形**，首页套上该釉后肉眼不只是换 hue。

色 token 仍走 `REQUIRED_TOKENS`（void…scheme）。另加 `CHROME_TOKENS`，写进 `[data-theme]`：`--brick-radius` `--lever-radius` `--wall-radius` `--cell` `--gap` `--font-display` `--font-ui` `--font-mono`。窑墙 CSS 读这些变量，不再写死 `border-radius: 2px`。

对比：`bone` / `ash` / `ember-4` / `warn` 对 `void` ≥ 4.5；`clay` 异于 `void` 与 `mortar`，对两者 ≥ 1.15。日历空→热仍靠 ember-1…4。

### 3.1 cartoon / 漫

- **想法：** 圆角砖 + 厚字。周日漫画的纸，不是儿童 app。
- **场：** 暖纸浅底 `#fff1d6`，字近黑。热阶焦糖→砖红，标题用深砖红（浅底上 ember-4 必须深）。
- **形：** `--brick-radius: 7px`，lever 胶囊，墙圆角 14px，格子略疏。
- **字：** 标题 `Bagel Fat One`（拉丁）+ `M PLUS Rounded 1c`（CJK 圆体）+ Chiron 垫字；UI 圆体；数字仍 Martian Mono，合计能读。

### 3.2 ledger / 端

- **想法：** CRT/终端账本。墨已经是报纸，这套是仪器，不是铅字 2.0，也不是青/霜换皮。
- **场：** 橄榄近黑 + 磷光黄绿，不是窑的焦橙，也不是 Matrix 纯绿。
- **形：** 砖/墙/lever 直角 0。格子略密。
- **字：** 标题 `Share Tech Mono`，UI/等宽 `IBM Plex Mono`。全页等宽，像一份终端。
- **禁：** 砖上 scanline、box-shadow、glow。平的。

两枚 id/印的取舍只记本机 `opt.md`，不进 git。

---

## 4. 釉厅交互

### 4.1 网格

- 路由：`/themes`。桌面 `.glaze-shelf { grid-template-columns: repeat(4, minmax(0, 1fr)) }`。窄屏可 2 列（触控），测试钉 4 列结构。
- 每卡：印 + 名 + 1–3 句。可留一排很小的空→热砖作署名，不占正文。
- 首页仍只有一个 **主题** lever。
- hover：描边/轻微 scale。禁止 `translateY`（全局 CSS 仍禁砖抬起）。`prefers-reduced-motion` 关掉过渡。

### 4.2 展开（主 UX）

路由：`/themes/:id`（同一 `Themes.vue`，同一张 DOM 卡，`data-flip-id`）。非法 id → `replace('/themes')`。

1. `Flip.getState` 点中的卡。
2. 另外七张 stagger 淡出+scale（约 200–250ms）。
3. 点中的卡 `position: fixed; inset: 0`，`Flip.from`（`absolute` + `scale`，约 320–400ms，`power2.inOut`）。应是**同一张卡在长**，不是路由抹成新页。
4. 展开后：
   - 上带：印/名 + 同一段说明书（可略长，仍短）。
   - 余下视口：首页缩样。至少：display 字号的假合计 `M`、用该釉 clay/ember 的缩窑墙（约 24×7）、首页 chrome 影子（刷新 / 主题，合计 / 工具 / 厂家）。缩样根节点带 `data-theme="id"` 或继承已预览的变量，圆角/字体/砖形必须对。
   - **应用**、回网格（`/themes` 或浏览器后退）。
5. 展开时 `applyTheme(id, { persist: false })`。用卡铺满视口盖住壳，避免整页闪色。
6. 回网格：先收缩样，Flip 卡回格子，七张淡入。壳的 `data-theme` 在卡仍盖住时恢复进入厅时的釉。
7. **应用：** `applyTheme(id)`（写 storage）+ `push('/')`。
8. 未应用离开厅：`onBeforeUnmount` 恢复进入时的釉（现行为）。
9. `will-change` 只在 tween 期间。动的是 transform/opacity。GSAP `context` + `onUnmounted` `revert`。
10. 中途再点/后退：杀掉当前 timeline，不要叠两段 Flip。

深链 `/themes/kiln`：直接展开，不做从网格飞入。从此后退仍走反向 Flip（若有网格 DOM）。

### 4.3 缩样

假数字即可，不打 `/api/summary`。用真实 `.rail` `.wall` `.brick` `.damper` `.lever` 类，让 chrome token 生效。窑墙规则不变：无月标、无星期槽。

---

## 5. 动效技术

- 依赖：`gsap`（含免费 Flip 插件）。
- Vue 3：`onMounted` 后才 tween；选择器包在 `gsap.context(scope)`；卸载 `ctx.revert()`。
- 时间轴：others fade `0`，Flip `<` 同时起步；缩样内容 `-=0.1` 淡入。总长 < 400ms 量级。
- 减动：`matchMedia('(prefers-reduced-motion: reduce)')` 为真则 `Flip`/`fade` 全跳过，只切 class。

---

## 6. 测试

- 8 个 id；无 qingmo/frost；未知与这两枚 → kiln。
- 新釉 REQUIRED + CHROME + 对比度。
- 釉厅 8 卡、CSS 4 列。
- 展开含说明书 + `.glaze-mock`；应用才 persist 并离开。
- 减动路径不抛错。
- 窑墙仍无 box-shadow / translateY / mix-blend / 3D glow；砖圆角走 `var(--brick-radius)`。

核验：`cd web && npm test && npm run build`。Go 与 `scripts/verify-local.sh` 若未改后端可跳过，改了再跑。
