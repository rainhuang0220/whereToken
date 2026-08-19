package community

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/rainhuang0220/whereToken/internal/event"
	"github.com/rainhuang0220/whereToken/internal/metric"
	"github.com/rainhuang0220/whereToken/internal/price"
)

const (
	MaxTokens     int64   = 1_000_000_000_000_000 // 1e15
	MaxCostUSD    float64 = 1_000_000_000
	MaxVersionLen         = 32
	MaxOffsetMin          = 14 * 60
)

var (
	periodRe  = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	uuidRe    = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	versionRe = regexp.MustCompile(`^[0-9A-Za-z._+-]{1,32}$`)
)

// AllowedUploadKeys is the only JSON object the rank service accepts.
var AllowedUploadKeys = []string{
	"participant_id",
	"period",
	"utc_offset_minutes",
	"tokens",
	"estimated_cost_usd",
	"cost_status",
	"client_version",
}

// ForbiddenUploadKeys are rejected even if they would otherwise be unknown.
// The decoder already rejects unknown fields; this list documents the
// privacy boundary and is used by tests.
var ForbiddenUploadKeys = []string{
	"prompt", "response", "content", "transcript",
	"session", "session_id", "request_id", "request",
	"path", "filename", "repository", "workspace", "project",
	"jwt", "cookie", "authorization", "api_key", "credential",
	"events", "usage_events", "turns", "sqlite", "index",
	"email", "username", "hostname", "ip", "github",
}

// Upload is the only payload that crosses the network. It is built from
// already-aggregated local totals. It must never carry a usage event.
type Upload struct {
	ParticipantID    string   `json:"participant_id"`
	Period           string   `json:"period"`
	UTCOffsetMinutes int      `json:"utc_offset_minutes"`
	Tokens           int64    `json:"tokens"`
	EstimatedCostUSD *float64 `json:"estimated_cost_usd,omitempty"`
	CostStatus       string   `json:"cost_status,omitempty"`
	ClientVersion    string   `json:"client_version"`
}

// LocalAgg is the local-side aggregation boundary: canonical accounting
// already happened. Only these numbers may be uploaded.
type LocalAgg struct {
	LocalDate       string
	UTCOffsetMin    int
	TodayTokens     int64
	TodayCostUSD    *float64
	TodayCostStatus string
}

func (a LocalAgg) Equal(b LocalAgg) bool {
	if a.LocalDate != b.LocalDate || a.TodayTokens != b.TodayTokens || a.TodayCostStatus != b.TodayCostStatus {
		return false
	}
	if (a.TodayCostUSD == nil) != (b.TodayCostUSD == nil) {
		return false
	}
	if a.TodayCostUSD != nil && b.TodayCostUSD != nil && *a.TodayCostUSD != *b.TodayCostUSD {
		return false
	}
	return true
}

func DecodeUpload(raw []byte) (Upload, error) {
	if err := rejectForbidden(raw); err != nil {
		return Upload{}, err
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var u Upload
	if err := dec.Decode(&u); err != nil {
		return Upload{}, fmt.Errorf("invalid community upload: %w", err)
	}
	if err := ValidateUpload(u); err != nil {
		return Upload{}, err
	}
	return u, nil
}

func rejectForbidden(raw []byte) error {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return fmt.Errorf("invalid community upload: %w", err)
	}
	for k := range obj {
		lk := strings.ToLower(strings.TrimSpace(k))
		for _, bad := range ForbiddenUploadKeys {
			if lk == bad {
				return fmt.Errorf("community upload must not include %q", k)
			}
		}
	}
	return nil
}

func ValidateUpload(u Upload) error {
	if !uuidRe.MatchString(u.ParticipantID) {
		return fmt.Errorf("invalid participant_id")
	}
	if !periodRe.MatchString(u.Period) {
		return fmt.Errorf("invalid period")
	}
	if _, err := time.Parse("2006-01-02", u.Period); err != nil {
		return fmt.Errorf("invalid period")
	}
	if u.UTCOffsetMinutes < -MaxOffsetMin || u.UTCOffsetMinutes > MaxOffsetMin {
		return fmt.Errorf("invalid utc_offset_minutes")
	}
	if u.Tokens < 0 || u.Tokens > MaxTokens {
		return fmt.Errorf("tokens out of range")
	}
	switch u.CostStatus {
	case "", price.StatusComplete, price.StatusPartial, price.StatusUnavailable:
	default:
		return fmt.Errorf("invalid cost_status")
	}
	if u.EstimatedCostUSD != nil {
		if *u.EstimatedCostUSD < 0 || *u.EstimatedCostUSD > MaxCostUSD {
			return fmt.Errorf("estimated_cost_usd out of range")
		}
	}
	if u.CostStatus == price.StatusUnavailable && u.EstimatedCostUSD != nil {
		return fmt.Errorf("unavailable cost must omit estimated_cost_usd")
	}
	if !versionRe.MatchString(u.ClientVersion) {
		return fmt.Errorf("invalid client_version")
	}
	return nil
}

func (u Upload) MarshalAllowed() ([]byte, error) {
	return json.Marshal(u)
}

func UploadKeys(raw []byte) ([]string, error) {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(obj))
	for k := range obj {
		keys = append(keys, k)
	}
	return keys, nil
}

// BuildLocalAgg prices and totals today's local-calendar events. It is the
// last step that may see UsageEvent values. The returned struct has no
// events, paths, or request ids.
func BuildLocalAgg(events []event.UsageEvent, now time.Time, loc *time.Location) LocalAgg {
	if loc == nil {
		loc = time.Local
	}
	now = now.In(loc)
	day := now.Format("2006-01-02")
	_, offset := now.Zone()
	var today []event.UsageEvent
	for _, e := range events {
		if e.Timestamp.IsZero() {
			continue
		}
		if e.Timestamp.In(loc).Format("2006-01-02") == day {
			today = append(today, e)
		}
	}
	sum := metric.CostSlice(today)
	agg := LocalAgg{
		LocalDate:       day,
		UTCOffsetMin:    offset / 60,
		TodayTokens:     sum.Total(),
		TodayCostStatus: sum.CostStatus,
	}
	if agg.TodayCostStatus == "" {
		agg.TodayCostStatus = price.Status(sum.PricedTokens, sum.UnpricedTokens)
	}
	if (agg.TodayCostStatus == price.StatusComplete || agg.TodayCostStatus == price.StatusPartial) && sum.CostMicro > 0 {
		usd := float64(sum.CostMicro) / 1e6
		if usd > 0 {
			agg.TodayCostUSD = &usd
		}
	}
	return agg
}

func MakeUpload(participantID, version string, agg LocalAgg) (Upload, error) {
	u := Upload{
		ParticipantID:    participantID,
		Period:           agg.LocalDate,
		UTCOffsetMinutes: agg.UTCOffsetMin,
		Tokens:           agg.TodayTokens,
		EstimatedCostUSD: agg.TodayCostUSD,
		CostStatus:       agg.TodayCostStatus,
		ClientVersion:    version,
	}
	if err := ValidateUpload(u); err != nil {
		return Upload{}, err
	}
	return u, nil
}
