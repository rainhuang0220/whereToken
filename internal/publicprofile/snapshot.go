// Package publicprofile builds a privacy-allowlisted public usage snapshot.
// Renderers, JSON writers, and the live page consume Snapshot only —
// never scan.Result, events, turns, or raw errors.
package publicprofile

import (
	"time"

	"github.com/rainhuang0220/whereToken/internal/event"
)

const (
	SchemaName             = "wheretoken.public-profile"
	SchemaVersion          = 1
	TokenAccountingVersion = "1"
	WallWeeks              = 53
	WallDays               = WallWeeks * 7
	GzipBudget             = 500 * 1024
)

const (
	StatusAvailable   = "available"
	StatusPartial     = "partial"
	StatusUnavailable = "unavailable"
)

const (
	CellUnknown = "unknown"
	CellEmpty   = "empty"
	CellActive  = "active"
	CellFuture  = "future"
)

const (
	PeriodAll   = "all"
	PeriodToday = "today"
	Period7d    = "7d"
	Period30d   = "30d"
	Period53w   = "53w"
)

const (
	AgentOtherID     = "other"
	AgentOtherLabel  = "Other"
	VendorOtherID    = "other"
	VendorOtherLabel = "Other"
	ModelOtherID     = "other"
	ModelOtherLabel  = "Other"
	SeriesAllID      = "all"
)

const (
	NoticePartialUsage = "partial_usage"
	NoticeUnpricedCost = "unpriced_cost"
)

const emDash = "—"

// Snapshot is the allowlisted public profile contract.
type Snapshot struct {
	Schema        string     `json:"schema"`
	SchemaVersion int        `json:"schema_version"`
	GeneratedAt   string     `json:"generated_at"`
	AsOfDate      string     `json:"as_of_date"`
	Producer      Producer   `json:"producer"`
	Owner         *Owner     `json:"owner,omitempty"`
	Provenance    Provenance `json:"provenance"`
	DataStatus    string     `json:"data_status"`
	Privacy       Privacy    `json:"privacy"`
	Periods       Periods    `json:"periods"`
	Activity      Activity   `json:"activity"`
	Notices       []string   `json:"notices"`
	Links         Links      `json:"links"`
}

type Producer struct {
	Name                   string `json:"name"`
	Version                string `json:"version"`
	TokenAccountingVersion string `json:"token_accounting_version"`
}

type Owner struct {
	DisplayName string `json:"display_name,omitempty"`
	GitHubLogin string `json:"github_login,omitempty"`
	AvatarURL   string `json:"avatar_url,omitempty"`
	ProfileURL  string `json:"profile_url,omitempty"`
}

type Provenance struct {
	Kind        string `json:"kind"`
	RefreshMode string `json:"refresh_mode"`
	LiveSync    bool   `json:"live_sync"`
}

type Privacy struct {
	RawEvents       bool     `json:"raw_events"`
	ModelsIncluded  bool     `json:"models_included"`
	CostIncluded    bool     `json:"cost_included"`
	DailyPrecision  bool     `json:"daily_precision"`
	OmittedSections []string `json:"omitted_sections"`
}

type Periods struct {
	All   Period `json:"all"`
	Today Period `json:"today"`
	D7    Period `json:"7d"`
	D30   Period `json:"30d"`
	W53   Period `json:"53w"`
}

func (p Periods) ByID(id string) (Period, bool) {
	switch id {
	case PeriodAll:
		return p.All, true
	case PeriodToday:
		return p.Today, true
	case Period7d:
		return p.D7, true
	case Period30d:
		return p.D30, true
	case Period53w:
		return p.W53, true
	default:
		return Period{}, false
	}
}

type Period struct {
	Range         DateRange   `json:"range"`
	Totals        Totals      `json:"totals"`
	HitRate       HitRate     `json:"hit_rate"`
	Requests      Count       `json:"requests"`
	UserTurns     Count       `json:"user_turns"`
	ActiveDays    Count       `json:"active_days"`
	CurrentStreak Count       `json:"current_streak"`
	LongestStreak Count       `json:"longest_streak"`
	Peak          Peak        `json:"peak"`
	Portrait      Portrait    `json:"portrait"`
	ByAgent       []Breakdown `json:"by_agent"`
	ByVendor      []Breakdown `json:"by_vendor"`
	ByModel       []Breakdown `json:"by_model,omitempty"`
	Cost          *Cost       `json:"cost,omitempty"`
}

type DateRange struct {
	From  *string `json:"from"`
	To    string  `json:"to"`
	Label string  `json:"label"`
}

type Totals struct {
	Miss        Component `json:"miss"`
	CacheRead   Component `json:"cache_read"`
	CacheCreate Component `json:"cache_create"`
	Output      Component `json:"output"`
	Total       Component `json:"total"`
}

type Component struct {
	Value   *int64 `json:"value"`
	Display string `json:"display"`
	Status  string `json:"status"`
}

type HitRate struct {
	Value   *float64 `json:"value"`
	Display string   `json:"display"`
}

type Count struct {
	Value   *int64 `json:"value"`
	Display string `json:"display"`
	Status  string `json:"status"`
}

type Peak struct {
	Date  string    `json:"date"`
	Total Component `json:"total"`
	Scope string    `json:"scope"`
}

type Portrait struct {
	State   string   `json:"state"`
	Primary string   `json:"primary"`
	Tags    []string `json:"tags"`
}

type Breakdown struct {
	ID       string  `json:"id"`
	Label    string  `json:"label"`
	Totals   Totals  `json:"totals"`
	Share    string  `json:"share"`
	HitRate  HitRate `json:"hit_rate"`
	Requests Count   `json:"requests"`
	Quality  string  `json:"quality"`
}

type Cost struct {
	Status         string `json:"status"`
	USDMicro       int64  `json:"usd_micro"`
	Display        string `json:"display"`
	PricedTokens   int64  `json:"priced_tokens"`
	UnpricedTokens int64  `json:"unpriced_tokens"`
	PriceCardID    string `json:"price_card_id"`
	VerifiedAt     string `json:"verified_at"`
}

type Activity struct {
	WeekStart string   `json:"week_start"`
	From      string   `json:"from"`
	To        string   `json:"to"`
	Dates     []string `json:"dates"`
	Series    []Series `json:"series"`
}

type Series struct {
	Dimension string   `json:"dimension"`
	ID        string   `json:"id"`
	Label     string   `json:"label"`
	Values    []int64  `json:"values"`
	Levels    []int    `json:"levels"`
	States    []string `json:"states"`
}

type Links struct {
	Project         string `json:"project"`
	TokenAccounting string `json:"token_accounting"`
	LivePage        string `json:"live_page"`
}

type Input struct {
	Events        []event.UsageEvent
	Turns         []event.TurnEvent
	Now           time.Time
	Loc           *time.Location
	Version       string
	IncludeModels bool
	IncludeCost   bool
	Owner         *Owner
	PortraitSeed  string
}
