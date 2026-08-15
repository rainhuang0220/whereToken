# whereToken 窑墙规格（视觉 + 日历）

Date: 2026-08-15
Status: 第 3 轮锁死。方案 B 与两轴已批准，本文件不重开。
Parent: [`2026-08-15-wheretoken-design.md`](./2026-08-15-wheretoken-design.md)
Decisions: [`opt.md`](../../../opt.md) 第 3 轮
Plan: [`docs/superpowers/plans/2026-08-15-wheretoken-calendar.md`](../plans/2026-08-15-wheretoken-calendar.md)

---

## 1. 这一轮要解决什么

上一版 Vue 是浅色新闻纸账本：双线报头、ZCOOL XiaoWei、Courier Prime、KPI 栅格、两张 ledger 表。它和工具箱里其它「本机实用页」同属一个家族，用户已经明确拒绝。

同时，日序列被计划推迟成 ECharts 折线。用户把它提成 P0：一堵 **GitHub 贡献图力学** 的小方格墙，强度 = 当天 token，并且必须跟已有两轴一起动。

成功标准：

1. 打开页面，第一眼是窑墙（53 周 × 7 天的方格），不是表、不是 KPI 卡片墙。
2. 切 **合计 / 某一个工具 / 某一个厂家**，同一堵墙重新上色，峰值和两段连烧立刻换成该序列的数。前端不做日桶、峰值、连烧的二次计算。
3. 峰值：该序列 `total` 最大的一天，日期 + `FormatM`。
4. 当前连烧 / 历史最长连烧：连续 `total > 0` 的本地日。规则见 §5.6。
5. `sum(calendar.all.days.total) == all.total`；按工具、按厂家的日桶各自守恒到对应 slice。
6. 视觉上不可能被认成 GitHub 绿墙、GitLab 蓝墙、tokscale Primer、或 PlainList / Flow / Untitled / docxeditor / 上一版 whereToken。

非目标：

- 3D isometric 贡献图（tokscale 做过，不跟）。
- ECharts calendar 组件（中国后台默认皮肤）。
- 美元、订阅剩余、社交提交。
- 把扫描错误的日子填成 0。

---

## 2. 从别人那里偷的手艺（不是皮肤）

只借力学和信息层次，不借调色盘、字体、组件库。

| 对象 | 偷什么 | 明确不偷 |
|------|--------|----------|
| GitHub contribution graph | 周为列、星期为行、`grid-auto-flow: column`、5 档强度、空 vs 有、hover 出当日值、月标签贴在整周列上、一三五标星期 | 周日周、primer 绿、圆角 2px 的「绿豆子」、commit 文案 |
| GitLab | 可周一为周首；空格与有值必须一眼可分 | 蓝梯度、`>30 commits → 最深` 这种绝对阈值 |
| WakaTime / wakafetch | 热力是「花掉的时间」不是「提交次数」；tooltip 带单位 | README 用的 GitHub 绿 SVG 皮肤 |
| tokscale TUI Stats | 把贡献墙当一等视图，而不是表下面的 chart；筛选源会重绘墙 | Primer 主题循环、3D 砖块、排行榜、成本 |
| Linear cycle | 密度高、数字是主角、页面不像后台 | 紫/靛 SaaS、卡片栅格、Inter |
| Stripe Sigma | 查询结果像仪器，不是仪表盘模板 | 浅灰卡片、通用图表库默认色 |

whereToken 自己的隐喻：**token 是烧掉的热量，日历是窑墙。** 一天一块砖。没烧过的砖是冷黏土；烧过的从焦褐到白热。这是消耗账，不是贡献秀。

---

## 3. 视觉身份（窑墙）

### 3.1 性格

工业、暗、密、烫。像看炉膛，不像看报表。中文工作台，但不是工具箱那套「素纸 + 衬线标题」。克制的动效：墙按列点亮，切轴时砖变色，数字不跳马戏团。

一句能记住的东西：**一堵会发光的消耗墙。**

### 3.2 为什么不是同系列

| 产品 | 它们的样子 | whereToken 这一轮 |
|------|-----------|-------------------|
| PlainList | 时间线清单 | 不是列表 |
| Flow | 会议 / 实时 | 不是媒体 |
| Untitled | 本机网盘、常见 Vue 工作台 | 不是文件管理壳 |
| docxeditor | 文档画布 | 不是纸 |
| whereToken 上一版 | 奶油纸、双线报头、账本表 | **整页替换**，不是改透明度 |

共同气质只保留：本机、中文、数字诚实。视觉不再「toolkit 默认皮肤」。

### 3.3 色

只做暗色窑。`color-scheme: dark`。不跟随系统切回浅色新闻纸。

```
--void:    #070504      炉膛黑
--clay:    #14100c      未烧砖（当日 total=0）
--future:  transparent  本周还未到的日子：只有极淡轮廓，不是「零用量」
--ember-1: #4a2714
--ember-2: #9a3a0d
--ember-3: #e85a10
--ember-4: #ffc44a      白热；峰值砖加 8px 铜晕
--bone:    #e8dcc8      正文
--ash:     #8a7a68      次要
--copper:  #c47a3a      轴标签、描边
--warn:    #ff6b3d      质量旗标
```

禁止：GitHub `#9be9a8…#216e39`、GitLab 蓝、紫渐变白底、Inter 紫按钮。

图例文案：`冷 → 白热`，不要 `Less / More`。

### 3.4 字

Google Fonts，开源可商用：

| 角色 | 字体 | 用法 |
|------|------|------|
| 大数 | **Big Shoulders Display** 800 | 峰值 M、两段连烧、合计 M |
| 中文 | **Chiron Hei HK** 400/600 | 标题、轴名、表头、tooltip |
| 日期 / 坐标 | **Martian Mono** | 月标签、星期、扫描时间 |

禁止：Inter、Roboto、Arial、system-ui 作为主栈、Space Grotesk、ZCOOL XiaoWei、Courier Prime、Noto Sans SC 作为主中文（Chiron 缺字时才 fallback Source Han Sans / Noto Sans SC）。

数字一律 `tabular-nums`。单位仍是 `M`，由后端 `FormatM` 提供，前端不得 `/ 1e6`。

### 3.5 密度与层次（一页）

从上到下，只有这一条脊柱：

1. **顶栏极瘦：** 左 `whereToken`，右本地时间 + 刷新。无 kicker「本机观测」，无双线报头。
2. **窑墙（英雄）：** 左墙右印。墙 = 53 列 × 7 行方格。右印三块铸造数字：峰值（日期 + M）、当前连烧（天）、最长连烧（天）。
3. **轴切换：** 一段阻尼条。先「合计」，再工具（Claude Code / Kimi / Codex / OpenCode，有数据才出现），再厂家（Anthropic / …）。当前项铜底。切轴 = 换 `calendar.all | calendar.by_source[id] | calendar.by_vendor[id]`，墙和三块数字一起变。
4. **合计六列：** 一行仪器数字，不是六张卡片。墙已经是主角，这里是复核。
5. **按工具 / 按厂家表：** 留下，但改成密排仪器表（无衬线账本章名、无浅色 zebra）。行可点，等于切到该轴（与阻尼条同步）。
6. **工具 × 厂家：** 默认折叠。
7. **质量 / 错误：** 黄/橙一句人话，不进格子。

不要：卡片 grid、侧边栏、ECharts 条形分享图（上一版 `ShareBars` 删除）。日历不是 footer widget。

### 3.6 动效（data-viz）

- 首次：周列从左到右点亮，列延迟 12ms，时长 280ms，`ease-out`。
- 切轴：砖 `background-color` 280ms；铸造数字替换，不滚动数字马戏团。
- hover：砖微抬 1px + tooltip（日期、当日 `total_m`、四列 M）。
- `prefers-reduced-motion: reduce`：全部瞬时。

纹理：一层 4% 不透明度的 grain（CSS repeating-conic 或 SVG noise），不是紫色 mesh。

---

## 4. 日历墙力学

### 4.1 格子

- **53 周列 × 7 日行。** 列是周，行是星期。CSS：`grid-auto-flow: column; grid-template-rows: repeat(7, var(--cell));`
- **周首：星期一。** 中国默认。锁死，不做设置项。`calendar.week_start = "monday"`。
- **窗口：** 含「今天」的这一周为最后一列。向前 52 个完整周。第一列的周一可能早于「今天减 364 天」。
- **今天之后** 的格子（本周未来）：`kind=future`，不是空烧。
- **窗口内、今天及以前、total=0：** `kind=empty`，冷黏土。这是「这天没花」，不是「系统没数据」。
- 月标签：该月第一次出现的那一列上方写 `1月`…`12月`（中文数字月）。
- 行标签：只标 `一` `三` `五`（对齐 GitHub 标 Sun/Wed/Fri 的密度，但周一为顶行）。

砖尺寸：桌面 `--cell: 12px`，间隙 3px；窄屏墙横向滚动，不把 53 列挤成不可点的 4px。

### 4.2 单元格的值

当天、当前过滤序列里，去重后事件的：

```
total = miss + cache_read + cache_create + output
```

同一本地日多条事件相加（先按现有 `request_id` 规则 merge，再按日加）。

Tooltip 单位全部 `*_m`（后端格式化）。不要出「1234567 tokens」。

### 4.3 过滤

同一堵墙，三个互斥模式：

| 模式 | 数据 | 例子 |
|------|------|------|
| 合计 | `calendar.all` | 所有工具所有厂家 |
| 按工具 | `calendar.by_source[id]` | 只 Claude Code（含它调 MiniMax 的那天） |
| 按厂家 | `calendar.by_vendor[id]` | 只 MiniMax（无论哪个工具） |

切过滤：强度分桶、峰值、两段连烧都用**该序列自己的分布**。禁止拿全局 max 去给 OpenCode 上色（否则 Kimi/Claude 的缓存读会把小源洗成全空）。

### 4.4 空、零、错误

| 状态 | 含义 | 画法 |
|------|------|------|
| future | 窗口里、日期 > 今天 | 空心轮廓 |
| empty | 窗口里、≤今天、该序列当天无事件或 total=0 | 冷黏土 `--clay` |
| lit | total>0 | ember 1–4 |
| 扫描错误 | 某文件失败 | **不**把那天画成 empty 来假装「确认零」。错误只进顶栏/页脚 `errors[]`。日历只反映成功解析的事件。 |

「无数据 / 第一次事件之前」：窗口里、第一次全库事件之前的日子仍画 empty（冷黏土）。不另开第三色——用户要的对比是 **没花 vs 花了多少**；扫描失败用文字，不用格子撒谎。

### 4.5 时区

事件 `Timestamp` 转 **扫描进程本地时区** 的日历日：`t.In(loc).Format("2006-01-02")`。生产 `loc = time.Local`。测试注入 `Asia/Shanghai`。

GitHub 用 UTC 日。我们不用 UTC。中国用户周一 08:00 的用量必须落在周一。

JSON 带 `calendar.timezone`（`time.Local.String()`，如 `Asia/Shanghai` 或 `Local`）。

---

## 5. 分桶、峰值、连烧（Go `internal/metric`，前端只渲染）

全部在过滤后的序列上算。前端禁止用日数组自己算 peak/streak/level。

### 5.1 日桶

```
Day {
  date          YYYY-MM-DD   // 本地
  miss, cache_read, cache_create, output, total  int64
  total_m, miss_m, cache_read_m, cache_create_m, output_m  string
  level         0..4         // 0 = 当天没花；1–4 有花。稀疏数组里通常只有 level>=1
}
```

`days` **稀疏**：只输出 `total > 0` 的日。窗口里缺席 = empty。这样 `sum(days.total)` 才能对上 slice.total（零日不占位）。

窗口的 `window_from` / `window_to` 仍输出，供前端铺 53×7。

### 5.2 强度算法（锁死：过滤序列非零日的四分位）

考虑过两种：

1. `max/4` 线性切 —— 否决。一天 50M 缓存读会把同序列其它天全按进 1 档；更糟的是若误用全局 max，OpenCode 整年都是空。
2. **非零日 raw total 的四分位** —— 采用。每个序列自己的非零日，大约四成颜色，小源切过去仍然有对比。四分位已经按日计数分配，不需要再 log；log 是给「按最大值切」用的。

算法（每个序列独立，含窗口外的历史非零日，避免峰值/分布被 53 周截断）：

```
nonzero = sort(total for day in series if total > 0)
if len == 0: 无砖点亮
q(p) = nonzero[max(0, ceil(n*p)-1)]     // nearest-rank
Q1, Q2, Q3 = q(0.25), q(0.50), q(0.75)

if total == 0           → 0
if Q1 == nonzero[n-1]   → 2     // 所有非零日相等：一律中档，避免看起来像没烧
if total <= Q1          → 1
if total <= Q2          → 2
if total <= Q3          → 3
else                    → 4
```

`level` 写进每个 Day。前端只读。

### 5.3 峰值

该序列全部历史日里 `total` 最大的一天。平手取 **日期更晚** 的那天。

```
peak_date, peak_total, peak_total_m
```

无非零日：`peak_date=""`，`peak_total=0`，`peak_total_m="0.00 M"`。

峰值日若在 53 周窗外，铸造数字仍显示该日；墙上不一定有对应砖（可接受）。墙内若有同一天，该砖加铜晕。

### 5.4 当前连烧（锁死）

连续 `total > 0` 的本地日，从锚点往回走。

- 锚点 = **今天** 若今天 `total > 0`，否则 **昨天**。
- 若昨天也是 0（且今天是 0），当前连烧 = 0。
- 今天有用量才把今天算进当前连烧。今天还没花、但昨天在烧 → 连烧计到昨天（人还没下班，不断条）。

`now` 在生产里是 `time.Now().In(loc)` 的日期。测试注入。

### 5.5 最长连烧

从该序列 **最早非零日** 到 **今天** 的闭区间里，最长的一段连续 `total > 0`。不限于 53 周窗口。中间空日打断。今天之后不存在。

无非零日 → 0。

### 5.6 与 GitHub 的差异（有意）

| | GitHub | whereToken |
|--|--------|------------|
| 周首 | 周日 | 周一 |
| 日界 | UTC | 本地 |
| 当前 streak | 贡献日历自己的规则 | §5.4（今天为 0 则收到昨天） |
| 颜色 | 绿 | 焦褐→白热 |
| 值 | commits | token `total`（M） |
| 分桶范围 | 过去一年 | 该序列全部已扫历史的非零日 |

---

## 6. API

`scan --json` 与 `GET /api/summary` 仍是同一 `EncodeSummary`。在现有 `all` / `by_source` / `by_vendor` / `by_source_vendor` / `errors` 上增加 `calendar`。前端不重算日桶。

```
calendar: {
  week_start: "monday",
  timezone: "Asia/Shanghai",
  window_from: "2025-08-18",
  window_to: "2026-08-15",
  all: Series,
  by_source: { [id]: Series },
  by_vendor: { [id]: Series }
}

Series: {
  days: Day[],          // 稀疏，按 date 升序
  stats: {
    peak_date: "2026-03-02",
    peak_total: 12400000,
    peak_total_m: "12.40 M",
    current_streak: 14,
    longest_streak: 41
  }
}
```

`by_source` / `by_vendor` 的 key 与现有 slice `id` 一致（`claude`、`minimax`…）。没有事件的源不出现。

守恒：

- `sum(calendar.all.days.total) == all.total`
- 对每个 source id：`sum(calendar.by_source[id].days.total) == by_source[id].total`
- 对每个 vendor id：同上
- `sum_id calendar.by_source[id].days.total == calendar.all.days.total`（与两轴不变量同一精神）

`Aggregate` 继续用 `time.Local` + `time.Now()` 填日历。纯函数 `BuildCalendar(events, loc, now)` 供测试；`Aggregate` 对 merge 后的事件调用它。扫描错误数组不传入 `BuildCalendar`。

---

## 7. 前端

- 自定义 CSS Grid 墙，**不用** ECharts calendar / heatmap。
- 删除 `ShareBars.vue`（ECharts 条）。`echarts` 依赖若无其它图则卸掉。
- 新组件：`KilnWall.vue`（格子+月/星期+图例）、`FoundryMarks.vue`（峰值/连烧）、`AxisDamper.vue`（合计/工具/厂家切换）。
- store 只存 payload + 当前 `axis: { kind: 'all'|'source'|'vendor', id: string }`。切轴是查表。
- tooltip：`2026-08-15 · 12.40 M` 加四列。
- 无障碍：墙 `role="img"` + 文字摘要「过去 53 周一共 N 天有用量，峰值 DATE M，当前连烧 D 天」；每砖 `title` 兜底。

---

## 8. 测试（必须先红后绿）

Go `internal/metric`，时区 `Asia/Shanghai`，`now` 钉死：

1. **同日本地合并：** 两条事件本地日相同 → 一个 Day，total 相加（在 request_id 不同的前提下）。
2. **空日断连烧：** 日 1、2 有，日 3 无，日 4 有 → 最长 = 2，不是 3。
3. **峰值取 max：** 三天 10 / 50 / 20 → peak 是 50 那天。
4. **厂家过滤：** Claude×Anthropic 与 Claude×MiniMax 并存 → `by_vendor["minimax"]` 只有 MiniMax 事件。
5. **守恒：** `sum(calendar.all.days.total) == Aggregate(...).All.Total()`。
6. **当前连烧收口：** `now` 当天为 0、昨天连续有值 → current_streak 计到昨天，不含今天。
7. **分桶按序列：** 合计里的大日不得把 MiniMax 序列的小日压成全 0 档；MiniMax 非零日在自己的四分位里至少有一格 level≥1（相等时为 2）。

HTTP：`/api/summary` 的 JSON 含 `calendar.week_start=="monday"` 且 `calendar.all.stats` 存在。

Vue：`columnsFrom` 仍只转发 `*_m`。可加一个纯函数测试：给定 Series + window，铺格子时缺席日为 empty、未来为 future（若把铺格抽成 ts）。

夹具仍脱敏，不提交实盘 jsonl。

---

## 9. 自检

- 无 TBD。周首、时区、空日、今天为 0 的连烧、分桶、API 形状均已锁。
- 与父规格不冲突：六列公式、两轴守恒、127.0.0.1、只读、单位 M、质量旗标全部继承。
- 范围仍是一个产品：度量核加字段 + 一页 Vue 重做。
- 「空」只有一种格子含义（没花）；扫描失败不进格子。
- 视觉与 tokscale/GitHub 的差异写在 §2 和 §5.6，实现时不许把绿主题「先做上再说」。
