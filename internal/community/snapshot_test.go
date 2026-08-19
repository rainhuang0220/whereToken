package community

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/rainhuang0220/whereToken/internal/event"
	"github.com/rainhuang0220/whereToken/internal/metric"
	"github.com/rainhuang0220/whereToken/internal/price"
)

func TestDecodeUploadRejectsUnknownAndForbidden(t *testing.T) {
	id := "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee"
	base := map[string]any{
		"participant_id":     id,
		"period":             "2026-08-19",
		"utc_offset_minutes": 480,
		"tokens":             100,
		"client_version":     "0.5.0",
	}
	ok, err := json.Marshal(base)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeUpload(ok); err != nil {
		t.Fatalf("valid: %v", err)
	}
	for _, bad := range []string{
		"prompt", "session_id", "request_id", "jwt", "events",
		"path", "hostname", "email", "Prompt", "SESSION_ID",
	} {
		t.Run(bad, func(t *testing.T) {
			m := map[string]any{}
			for k, v := range base {
				m[k] = v
			}
			m[bad] = "nope"
			raw, _ := json.Marshal(m)
			if _, err := DecodeUpload(raw); err == nil {
				t.Fatalf("accepted forbidden %s", bad)
			}
		})
	}
	extra := map[string]any{}
	for k, v := range base {
		extra[k] = v
	}
	extra["unexpected"] = 1
	raw, _ := json.Marshal(extra)
	if _, err := DecodeUpload(raw); err == nil {
		t.Fatal("accepted unexpected field")
	}
}

func TestValidateUploadBounds(t *testing.T) {
	cost := 1.0
	u := Upload{
		ParticipantID:    "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee",
		Period:           "2026-08-19",
		UTCOffsetMinutes: 480,
		Tokens:           10,
		EstimatedCostUSD: &cost,
		CostStatus:       price.StatusComplete,
		ClientVersion:    "0.5.0",
	}
	if err := ValidateUpload(u); err != nil {
		t.Fatal(err)
	}
	bad := u
	bad.Tokens = -1
	if err := ValidateUpload(bad); err == nil {
		t.Fatal("neg tokens")
	}
	bad = u
	bad.Tokens = MaxTokens + 1
	if err := ValidateUpload(bad); err == nil {
		t.Fatal("huge tokens")
	}
	huge := MaxCostUSD + 1
	bad = u
	bad.EstimatedCostUSD = &huge
	if err := ValidateUpload(bad); err == nil {
		t.Fatal("huge cost")
	}
	bad = u
	bad.UTCOffsetMinutes = 2000
	if err := ValidateUpload(bad); err == nil {
		t.Fatal("offset")
	}
	zero := 0.0
	bad = u
	bad.CostStatus = price.StatusUnavailable
	bad.EstimatedCostUSD = &zero
	if err := ValidateUpload(bad); err == nil {
		t.Fatal("unavailable must not carry $0")
	}
	bad = u
	bad.ParticipantID = "not-a-uuid"
	if err := ValidateUpload(bad); err == nil {
		t.Fatal("uuid")
	}
	bad = u
	bad.Period = "today"
	if err := ValidateUpload(bad); err == nil {
		t.Fatal("period")
	}
}

func TestUploadJSONHasOnlyAllowedKeys(t *testing.T) {
	cost := 12.5
	u := Upload{
		ParticipantID:    "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee",
		Period:           "2026-08-19",
		UTCOffsetMinutes: 480,
		Tokens:           35700000,
		EstimatedCostUSD: &cost,
		CostStatus:       price.StatusPartial,
		ClientVersion:    "0.5.0",
	}
	raw, err := json.Marshal(u)
	if err != nil {
		t.Fatal(err)
	}
	keys, err := UploadKeys(raw)
	if err != nil {
		t.Fatal(err)
	}
	allowed := map[string]struct{}{}
	for _, k := range AllowedUploadKeys {
		allowed[k] = struct{}{}
	}
	for _, k := range keys {
		if _, ok := allowed[k]; !ok {
			t.Fatalf("unexpected key %q in %s", k, raw)
		}
	}
	s := string(raw)
	for _, bad := range []string{"prompt", "session", "request_id", "path", "jwt", "cookie", "events"} {
		if strings.Contains(s, bad) {
			t.Fatalf("payload mentions %s: %s", bad, s)
		}
	}
}

func TestBuildLocalAggNegativeUTCOffset(t *testing.T) {
	loc := time.FixedZone("PDT", -7*3600)
	now := time.Date(2026, 8, 19, 22, 0, 0, 0, loc) // 2026-08-20 05:00 UTC
	events := []event.UsageEvent{
		{Vendor: "anthropic", Model: "claude-opus-4.6", Miss: 1_000_000, Timestamp: time.Date(2026, 8, 20, 5, 0, 0, 0, time.UTC)},
		{Vendor: "anthropic", Model: "claude-opus-4.6", Miss: 9_000_000, Timestamp: time.Date(2026, 8, 20, 6, 0, 0, 0, loc)},
	}
	agg := BuildLocalAgg(events, now, loc)
	if agg.LocalDate != "2026-08-19" || agg.UTCOffsetMin != -420 {
		t.Fatalf("%+v", agg)
	}
	if agg.TodayTokens != 1_000_000 {
		t.Fatalf("PDT evening must keep the UTC-next-day event as local today: %d", agg.TodayTokens)
	}
}

func TestBuildLocalAggUsesLocalCalendarDay(t *testing.T) {
	loc := time.FixedZone("CST", 8*3600)
	now := time.Date(2026, 8, 19, 0, 30, 0, 0, loc) // 2026-08-18T16:30Z
	events := []event.UsageEvent{
		{Vendor: "anthropic", Model: "claude-opus-4.6", Miss: 1_000_000, Timestamp: time.Date(2026, 8, 18, 23, 0, 0, 0, loc)},
		{Vendor: "anthropic", Model: "claude-opus-4.6", Miss: 2_000_000, Timestamp: now},
		{Vendor: "anthropic", Model: "claude-opus-4.6", Miss: 9_000_000, Timestamp: time.Date(2026, 8, 18, 15, 30, 0, 0, time.UTC)},
	}
	agg := BuildLocalAgg(events, now, loc)
	if agg.LocalDate != "2026-08-19" {
		t.Fatalf("date=%s", agg.LocalDate)
	}
	if agg.UTCOffsetMin != 480 {
		t.Fatalf("offset=%d", agg.UTCOffsetMin)
	}
	// only the 00:30 CST event is today; 23:00 CST yesterday and 15:30 UTC (=23:30 CST 18th) stay out
	if agg.TodayTokens != 2_000_000 {
		t.Fatalf("tokens=%d", agg.TodayTokens)
	}
	if agg.TodayCostStatus != price.StatusComplete || agg.TodayCostUSD == nil {
		t.Fatalf("cost %+v", agg)
	}
}

func TestBuildLocalAggExcludesJustBeforeLocalMidnight(t *testing.T) {
	loc := time.FixedZone("CST", 8*3600)
	now := time.Date(2026, 8, 19, 10, 0, 0, 0, loc)
	events := []event.UsageEvent{
		{Vendor: "anthropic", Model: "claude-opus-4.6", Miss: 111, Timestamp: time.Date(2026, 8, 18, 23, 59, 59, 999999999, loc)},
		{Vendor: "anthropic", Model: "claude-opus-4.6", Miss: 7, Timestamp: time.Date(2026, 8, 19, 0, 0, 0, 0, loc)},
		{Vendor: "anthropic", Model: "claude-opus-4.6", Miss: 222, Timestamp: time.Date(2026, 8, 18, 15, 59, 59, 0, time.UTC)},
	}
	agg := BuildLocalAgg(events, now, loc)
	if agg.LocalDate != "2026-08-19" {
		t.Fatalf("date=%s", agg.LocalDate)
	}
	if agg.TodayTokens != 7 {
		t.Fatalf("just-before-midnight must stay on yesterday: tokens=%d", agg.TodayTokens)
	}
}

func TestBuildLocalAggDoesNotNeedCalendar(t *testing.T) {
	loc := time.FixedZone("CST", 8*3600)
	now := time.Date(2026, 8, 19, 12, 0, 0, 0, loc)
	today := event.UsageEvent{
		Source: "claude", Vendor: "anthropic", Model: "claude-opus-4.6",
		RequestID: "today", Miss: 1_000_000, Output: 2_000, Timestamp: now,
	}
	events := []event.UsageEvent{
		today,
		{
			Source: "claude", Vendor: "anthropic", Model: "claude-opus-4.6",
			RequestID: "yday", Miss: 9_000_000, Timestamp: time.Date(2026, 8, 18, 23, 0, 0, 0, loc),
		},
	}
	agg := BuildLocalAgg(events, now, loc)
	want := metric.CostSlice([]event.UsageEvent{today})
	if agg.TodayTokens == 0 || agg.TodayTokens != want.Total() {
		t.Fatalf("tokens=%d costslice=%d (BuildLocalAgg must not require Aggregate)", agg.TodayTokens, want.Total())
	}
	if agg.TodayCostStatus != want.CostStatus {
		t.Fatalf("status=%s want %s", agg.TodayCostStatus, want.CostStatus)
	}
	u, err := MakeUpload("aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee", "0.5.0", agg)
	if err != nil {
		t.Fatal(err)
	}
	if u.Tokens != agg.TodayTokens {
		t.Fatalf("upload tokens=%d", u.Tokens)
	}
}

func TestBuildLocalAggUnknownCostOmitsUSD(t *testing.T) {
	loc := time.UTC
	now := time.Date(2026, 8, 19, 12, 0, 0, 0, loc)
	events := []event.UsageEvent{
		{Vendor: "moonshot", Model: "k3", Miss: 1000, Timestamp: now},
	}
	agg := BuildLocalAgg(events, now, loc)
	if agg.TodayTokens != 1000 {
		t.Fatalf("tokens=%d", agg.TodayTokens)
	}
	if agg.TodayCostStatus != price.StatusUnavailable || agg.TodayCostUSD != nil {
		t.Fatalf("unknown cost must not become 0: %+v", agg)
	}
}

func TestMakeUploadNeverCarriesEvents(t *testing.T) {
	usd := 1.25
	agg := LocalAgg{LocalDate: "2026-08-19", UTCOffsetMin: 480, TodayTokens: 10, TodayCostUSD: &usd, TodayCostStatus: price.StatusComplete}
	u, err := MakeUpload("aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee", "0.5.0", agg)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(u)
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		t.Fatal(err)
	}
	if _, ok := obj["events"]; ok {
		t.Fatal("events leaked")
	}
	if obj["tokens"].(float64) != 10 {
		t.Fatalf("%v", obj)
	}
}

func TestMakeUploadOmitsDustCost(t *testing.T) {
	id := "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee"
	u, err := MakeUpload(id, "0.5.0", LocalAgg{
		LocalDate: "2026-08-19", UTCOffsetMin: 480, TodayTokens: 10,
		TodayCostUSD: ptrFloat(0.00004), TodayCostStatus: price.StatusComplete,
	})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(u)
	if err != nil {
		t.Fatal(err)
	}
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		t.Fatal(err)
	}
	if _, ok := obj["estimated_cost_usd"]; ok {
		t.Fatalf("dust complete must omit estimated_cost_usd: %s", raw)
	}
	if u.EstimatedCostUSD != nil {
		t.Fatalf("usd=%v", *u.EstimatedCostUSD)
	}
}

func TestUnavailableCostNeverBecomesZeroInUpload(t *testing.T) {
	id := "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee"
	tests := []struct {
		name   string
		agg    LocalAgg
		wantOK bool
	}{
		{
			name: "unavailable omits usd",
			agg: LocalAgg{
				LocalDate: "2026-08-19", UTCOffsetMin: 480, TodayTokens: 1000,
				TodayCostStatus: price.StatusUnavailable,
			},
			wantOK: true,
		},
		{
			name: "unavailable with explicit zero omitted",
			agg: LocalAgg{
				LocalDate: "2026-08-19", UTCOffsetMin: 480, TodayTokens: 1000,
				TodayCostUSD:    ptrFloat(0),
				TodayCostStatus: price.StatusUnavailable,
			},
			wantOK: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			u, err := MakeUpload(id, "0.5.0", tc.agg)
			if !tc.wantOK {
				if err == nil {
					t.Fatal("unavailable $0 must not upload")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			raw, err := json.Marshal(u)
			if err != nil {
				t.Fatal(err)
			}
			var obj map[string]any
			if err := json.Unmarshal(raw, &obj); err != nil {
				t.Fatal(err)
			}
			if _, ok := obj["estimated_cost_usd"]; ok {
				t.Fatalf("unavailable must omit estimated_cost_usd: %s", raw)
			}
			if obj["cost_status"] != price.StatusUnavailable {
				t.Fatalf("%s", raw)
			}
			got, err := DecodeUpload(raw)
			if err != nil {
				t.Fatal(err)
			}
			if got.EstimatedCostUSD != nil {
				t.Fatalf("decoded usd=%v", got.EstimatedCostUSD)
			}
		})
	}
}

func TestDecodeUploadRejectsZeroCost(t *testing.T) {
	raw := `{"participant_id":"aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee","period":"2026-08-19","tokens":10,"cost_status":"complete","estimated_cost_usd":0,"client_version":"0.5.0"}`
	if _, err := DecodeUpload([]byte(raw)); err == nil {
		t.Fatal("complete $0 must not decode")
	}
}

func ptrFloat(v float64) *float64 { return &v }
