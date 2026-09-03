package profile

// Phrase bank: short Chinese usage-behavior descriptors, grouped by trait
// key. Style rules (enforced by phrases_test.go):
//   - describe usage behavior only — never personality or intelligence
//   - no insults or derogatory wording
//   - no social-rank claims (超过 N% 用户, 排名, 榜)
//   - globally unique, non-empty, 18–24 per group
//
// A phrase only ever renders for its own trait: a high-intensity user can
// draw any intensity_high phrasing but never an intensity_light one.
var pools = map[string][]string{
	// Intensity: tokens per active day.
	"intensity_high": {
		"深度使用", "高强度调用", "重度使用", "全天候调用", "持续高负载",
		"高频会话", "满载运转", "大体量消耗", "重负载工作流", "高频调用节奏",
		"持续烧 token", "深度集成流程", "高频迭代节奏", "长时连续使用", "高吞吐使用",
		"主力生产工具", "深度日常使用", "密集调用", "高频请求节奏", "重度工作流",
	},
	"intensity_light": {
		"轻量使用", "偶尔调用", "点到为止", "低频体验", "零星使用",
		"轻度尝试", "浅度使用", "低强度调用", "偶尔上手", "小步试用",
		"低频调用", "轻触式使用", "少量调用", "间歇使用", "轻量试水",
		"低耗使用", "偶发调用", "少量体验", "轻度探索", "低节奏使用",
	},

	// Cost: API-equivalent USD per active day, only when priced.
	"cost_high": {
		"高算力投入", "高成本投入", "高预算使用", "旗舰级消耗", "高规格调用",
		"高单价模型为主", "成本高位运行", "高费用节奏", "重投入使用", "高成本工作流",
		"高额 token 消耗", "高端模型高频用", "成本密集型", "高消耗模式", "高投入节奏",
		"旗舰模型常驻", "高资费使用", "重成本投入", "算力预算充足", "高消耗工作流",
	},
	"cost_efficient": {
		"精打细算", "低成本运行", "经济适用", "低消耗使用", "成本友好",
		"轻成本运行", "高性价比路径", "低预算节奏", "节省型调用", "低成本工作流",
		"花费克制", "低资费模式", "成本可控", "节俭式使用", "小成本使用",
		"低消耗节奏", "经济型调用", "花费低位", "成本轻量", "低投入调用",
	},

	// Diversity: distinct labeled models / known vendors.
	"model_diversity": {
		"多模型探索", "模型游牧", "多模型并用", "模型万花筒", "多模型切换",
		"模型杂食", "跨模型工作流", "多模型轮换", "模型集邮", "多线探索",
		"模型自助餐", "广泛试用模型", "多模型并行", "模型多点开花", "多模型实验",
		"模型探索型", "多模型混搭", "博采众模型", "多模型实践", "多模型广撒网",
	},
	"vendor_diversity": {
		"多厂家覆盖", "跨平台使用", "多供应商并行", "厂家游牧", "多平台工作流",
		"跨厂家调用", "多生态接入", "厂家多点布局", "多平台探索", "跨生态使用",
		"多厂家轮换", "平台多元化", "多厂家混搭", "跨供应商工作流", "多平台并用",
		"厂家广覆盖", "多渠道调用", "多平台切换", "跨厂家探索", "多平台实践",
	},

	// Concentration: top labeled model's share of the window.
	"concentration": {
		"单模型主力", "专一模型路线", "单点深耕", "主力模型突出", "单一模型为主",
		"模型高度集中", "单模型深度绑定", "专注单一模型", "单模型重仓", "模型集中式",
		"单模型为主线", "深度单模型使用", "单模型策略", "单一模型工作流", "模型单点投入",
		"单模型深耕", "主力模型集中", "单模型贯穿", "单一模型路线", "模型一枝独秀",
	},

	// Cache reuse: cache_read share of input-side tokens.
	"cache_high": {
		"上下文复用充分", "高缓存复用", "缓存命中拉满", "长上下文复用", "缓存吃得很透",
		"上下文接续充分", "缓存利用率高", "重复上下文复用", "缓存复用型", "上下文延续性强",
		"缓存读取为主", "高复用工作流", "缓存复用充分", "上下文缓存高效", "缓存复用深入",
		"长会话复用", "缓存红利吃满", "上下文沉淀充分", "高缓存命中", "复用驱动使用",
	},

	// Rhythm: active days over window span / peak day over mean day.
	"steady": {
		"稳定使用", "节奏平稳", "日均稳定", "持续稳定输出", "规律使用",
		"稳定节奏", "常态化使用", "平稳运行", "固定节奏使用", "稳定输出节奏",
		"使用节奏均匀", "常态稳定", "均衡使用节奏", "稳定在线", "节奏可预期",
		"连续性使用", "稳定消耗节奏", "均匀分布使用", "规律调用", "平稳输出",
	},
	"bursty": {
		"阶段爆发式", "脉冲式使用", "峰值突出", "突发式调用", "集中爆发",
		"波浪式使用", "峰谷分明", "冲刺式使用", "间歇性爆发", "单点高峰",
		"爆发式消耗", "峰值驱动", "集中式突击", "高低峰交替", "阶段性冲刺",
		"突增式使用", "峰值型工作流", "短时高爆发", "潮汐式调用", "冲刺节奏明显",
	},
}

// salts give every trait pool its own deterministic offset into the phrase
// picker. Arbitrary fixed constants (fractional digits of pi); changing one
// re-phrases every user of that trait, so treat them as frozen.
var salts = map[string]uint64{
	"cost_high":        0x243f6a8885a308d3,
	"intensity_high":   0x13198a2e03707344,
	"model_diversity":  0xa4093822299f31d0,
	"vendor_diversity": 0x082efa98ec4e6c89,
	"concentration":    0x452821e638d01377,
	"cache_high":       0xbe5466cf34e90c6c,
	"steady":           0xc0ac29b7c97c50dd,
	"bursty":           0x3f84d5b5b5470917,
	"cost_efficient":   0x9216d5d98979fb1b,
	"intensity_light":  0xd1310ba698dfb5ac,
}
