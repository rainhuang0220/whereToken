package report

import (
	"strings"
	"testing"
	"time"

	"github.com/rainhuang0220/whereToken/internal/event"
	"github.com/rainhuang0220/whereToken/internal/publicprofile"
)

func TestCursorHistoryNoteUsesFilteredTokensAndUTCRecordDays(t *testing.T) {
	loc := shanghai()
	now := time.Date(2026, 11, 9, 12, 0, 0, 0, loc)
	events := []event.UsageEvent{
		{Source: "cursor", Vendor: "unknown", RequestID: "retained:first:history-unallocated", Timestamp: time.Date(2025, 10, 1, 23, 30, 0, 0, time.UTC), Miss: 1_000_000, CacheRead: 100_000, CacheCreate: 10_000, Output: 1_000, Reasoning: 99_999, Quality: event.QualityAuthoritative, Derivation: event.DeriveRetained, SkipRequest: true},
		{Source: "cursor", Vendor: "unknown", RequestID: "retained:last:history-unallocated", Timestamp: time.Date(2026, 11, 8, 23, 30, 0, 0, time.UTC), Miss: 123_567, Quality: event.QualityAuthoritative, Derivation: event.DeriveRetained, SkipRequest: true},
		{Source: "cursor", Vendor: "unknown", RequestID: "ordinary-unknown", Timestamp: now, Miss: 7, Quality: event.QualityAuthoritative},
		{Source: "cursor", Vendor: "openai", Model: "gpt-5", RequestID: "assigned:history-unallocated", Timestamp: now, Miss: 50, Quality: event.QualityAuthoritative},
		{Source: "claude", Vendor: "anthropic", Model: "claude-opus-4.6", RequestID: "other:history-unallocated", Timestamp: now, Miss: 100, Quality: event.QualityAuthoritative},
	}
	// A duplicate normalized row must not inflate either the report or note.
	events = append(events, events[0])
	tests := []struct {
		name   string
		filter Filter
		want   string
		total  int64
	}{
		{"default", Filter{}, "Cursor · 历史未分配 1,234,567 Token；UTC 记录日 2025-10-01 … 2026-11-08（仅有记录日，非完整覆盖）", 1_234_724},
		{"cursor", Filter{Tool: "cursor"}, "Cursor · 历史未分配 1,234,567 Token；UTC 记录日 2025-10-01 … 2026-11-08（仅有记录日，非完整覆盖）", 1_234_624},
		{"today in local timezone", Filter{Today: true}, "Cursor · 历史未分配 123,567 Token；UTC 记录日 2026-11-08 … 2026-11-08（仅有记录日，非完整覆盖）", 123_724},
		{"explicit date range", Filter{From: time.Date(2025, 10, 1, 0, 0, 0, 0, time.UTC), To: time.Date(2025, 10, 2, 0, 0, 0, 0, time.UTC)}, "Cursor · 历史未分配 1,111,000 Token；UTC 记录日 2025-10-01 … 2025-10-01（仅有记录日，非完整覆盖）", 1_111_000},
		{"unknown vendor", Filter{Vendor: "unknown"}, "Cursor · 历史未分配 1,234,567 Token；UTC 记录日 2025-10-01 … 2026-11-08（仅有记录日，非完整覆盖）", 1_234_574},
		{"known vendor excludes unknown history", Filter{Vendor: "openai"}, "", 50},
		{"known model excludes unknown history", Filter{Model: "gpt-5"}, "", 50},
		{"other tool with marker", Filter{Tool: "claude"}, "", 100},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			snap, err := Build(events, nil, nil, tc.filter, now, loc)
			if err != nil {
				t.Fatal(err)
			}
			if snap.Total != tc.total {
				t.Fatalf("total=%d want=%d", snap.Total, tc.total)
			}
			joined := strings.Join(snap.Notes, "\n")
			if tc.want != "" {
				if strings.Contains(joined, "token 列是近 53 周账号用量") || !strings.Contains(joined, "Token 列含历史保留账号用量") {
					t.Fatalf("historical coverage note misstates the 53-week window: %v", snap.Notes)
				}
			}
			var history []string
			for _, note := range snap.Notes {
				if strings.Contains(note, "历史未分配") {
					history = append(history, note)
				}
			}
			if tc.want == "" {
				if len(history) != 0 {
					t.Fatalf("history outside current filters leaked: %v", history)
				}
			} else if len(history) != 1 || history[0] != tc.want {
				t.Fatalf("history notes=%v want=%q", history, tc.want)
			}
		})
	}

	// Public Profile consumes these same retained events. A report-only
	// explanation must not drop their totals or invent model attribution.
	profile, err := publicprofile.Build(publicprofile.Input{Events: events, Now: now, Loc: loc, IncludeModels: true})
	if err != nil {
		t.Fatal(err)
	}
	if got := publicprofile.TotalTokens(profile); got != 1_234_724 {
		t.Fatalf("profile total=%d want=1234724", got)
	}
	if events[0].Model != "" || events[0].Vendor != "unknown" {
		t.Fatalf("report rewrote historical attribution: %+v", events[0])
	}
}

func TestOrdinaryUnknownVendorDoesNotClaimUnallocatedHistory(t *testing.T) {
	now := time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)
	snap, err := Build([]event.UsageEvent{{Source: "cursor", Vendor: "unknown", RequestID: "account-usage", Timestamp: now, Miss: 10, Quality: event.QualityAuthoritative}}, nil, nil, Filter{}, now, time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(snap.Notes, "\n")
	if strings.Contains(joined, "历史未分配") || !strings.Contains(joined, "未知厂家") || !strings.Contains(joined, "token 列是近 53 周账号用量") {
		t.Fatalf("ordinary unknown usage notes=%v", snap.Notes)
	}
}
