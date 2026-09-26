package publicprofile

import (
	"strings"
	"testing"
	"time"
	_ "time/tzdata"

	"github.com/rainhuang0220/whereToken/internal/event"
)

func intPtr(n int64) *int64 { return &n }

func gateSnap(id, status, date string, total *int64, agents ...Breakdown) Snapshot {
	return Snapshot{
		SnapshotID: id,
		DataStatus: status,
		AsOfDate:   date,
		Periods: Periods{All: Period{
			Totals:  Totals{Total: Component{Value: total, Status: status}},
			ByAgent: agents,
		}},
	}
}

func measured(id string, n int64) Breakdown {
	return Breakdown{
		ID:       id,
		Totals:   Totals{Total: Component{Value: intPtr(n), Status: StatusAvailable}},
		Coverage: Coverage{Tokens: StatusAvailable},
	}
}

func TestDecideRefreshTable(t *testing.T) {
	updated := time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)
	later := updated.Add(2 * time.Hour)
	remote := gateSnap("sha256:prev", StatusAvailable, "2026-09-25", intPtr(1_000_000))
	cases := []struct {
		name    string
		in      RefreshInput
		publish bool
		code    string
		label   string
	}{
		{
			name: "first available small total",
			in: RefreshInput{
				Local: gateSnap("", StatusAvailable, "2026-09-25", intPtr(1)),
				Now:   later,
			},
			publish: true,
			code:    CodePublished,
		},
		{
			name: "first unavailable",
			in: RefreshInput{
				Local: gateSnap("", StatusUnavailable, "2026-09-25", nil),
				Now:   later,
			},
			publish: false,
			label:   "用量不可用",
		},
		{
			name: "nil local total is not zero",
			in: RefreshInput{
				Local:           gateSnap("sha256:next", StatusAvailable, "2026-09-25", nil),
				Remote:          &remote,
				RemoteUpdatedAt: updated,
				Now:             later,
			},
			publish: false,
			label:   "用量不可用，保留上次",
		},
		{
			name: "nil remote total is not a zero baseline",
			in: RefreshInput{
				Local: gateSnap("sha256:next", StatusAvailable, "2026-09-25", intPtr(50_000)),
				Remote: func() *Snapshot {
					s := gateSnap("sha256:prev", StatusAvailable, "2026-09-25", nil)
					return &s
				}(),
				RemoteUpdatedAt: updated,
				Now:             later,
			},
			publish: false,
			code:    CodeSkippedDelta,
		},
		{
			name: "delta 9999 after an hour",
			in: RefreshInput{
				Local:           gateSnap("sha256:next", StatusAvailable, "2026-09-25", intPtr(1_000_000+9_999)),
				Remote:          &remote,
				RemoteUpdatedAt: updated,
				Now:             updated.Add(time.Hour),
			},
			publish: false,
			code:    CodeSkippedDelta,
		},
		{
			name: "delta 10000 before an hour",
			in: RefreshInput{
				Local:           gateSnap("sha256:next", StatusAvailable, "2026-09-25", intPtr(1_000_000+10_000)),
				Remote:          &remote,
				RemoteUpdatedAt: updated,
				Now:             updated.Add(time.Hour - time.Nanosecond),
			},
			publish: false,
			code:    CodeSkippedCooldown,
		},
		{
			name: "delta 10000 at exactly one hour",
			in: RefreshInput{
				Local:           gateSnap("sha256:next", StatusAvailable, "2026-09-25", intPtr(1_000_000+10_000)),
				Remote:          &remote,
				RemoteUpdatedAt: updated,
				Now:             updated.Add(time.Hour),
			},
			publish: true,
			code:    CodePublished,
		},
		{
			name: "date changed and total dropped",
			in: RefreshInput{
				Local: gateSnap("sha256:next", StatusAvailable, "2026-09-26", intPtr(1_000_000-1), measured("claude", 1_000_000-1)),
				Remote: func() *Snapshot {
					s := gateSnap("sha256:prev", StatusAvailable, "2026-09-25", intPtr(1_000_000), measured("claude", 1_000_000))
					return &s
				}(),
				RemoteUpdatedAt: updated,
				Now:             later,
			},
			publish: false,
			code:    CodeSkippedDelta,
		},
		{
			name: "date changed and total held",
			in: RefreshInput{
				Local:           gateSnap("sha256:next", StatusAvailable, "2026-09-26", intPtr(1_000_000)),
				Remote:          &remote,
				RemoteUpdatedAt: updated,
				Now:             later,
			},
			publish: true,
			code:    CodeDateRollover,
		},
		{
			name: "same day drop does not publish",
			in: RefreshInput{
				Local:           gateSnap("sha256:next", StatusAvailable, "2026-09-25", intPtr(1_000_000-1)),
				Remote:          &remote,
				RemoteUpdatedAt: updated,
				Now:             later,
			},
			publish: false,
			code:    CodeSkippedDelta,
		},
		{
			name: "weaker coverage does not publish on a new date",
			in: RefreshInput{
				Local:           gateSnap("sha256:next", StatusPartial, "2026-09-26", intPtr(2_000_000)),
				Remote:          &remote,
				RemoteUpdatedAt: updated,
				Now:             later,
			},
			publish: false,
			label:   "覆盖变弱，保留上次",
		},
		{
			name: "offline does not clobber",
			in: RefreshInput{
				Local:   gateSnap("sha256:next", StatusAvailable, "2026-09-26", intPtr(9_000_000)),
				Remote:  &remote,
				Now:     later,
				Offline: true,
			},
			publish: false,
			label:   "离线不覆盖已发布快照",
		},
		{
			name: "offline with no snapshot",
			in: RefreshInput{
				Local:   gateSnap("", StatusAvailable, "2026-09-25", intPtr(10)),
				Now:     later,
				Offline: true,
			},
			publish: false,
			label:   "离线不发布",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := DecideRefresh(tc.in)
			if got.Publish != tc.publish {
				t.Fatalf("publish=%v want %v (%s %s)", got.Publish, tc.publish, got.Phase, got.Label)
			}
			if tc.code != "" && got.Code != tc.code {
				t.Fatalf("code=%s want %s", got.Code, tc.code)
			}
			if tc.label != "" && got.Label != tc.label {
				t.Fatalf("label=%q want %q", got.Label, tc.label)
			}
			if got.Label != "" && (strings.Contains(got.Label, "0.00 M") || strings.Contains(got.Label, "$0") || strings.Contains(got.Label, "#0")) {
				t.Fatalf("decision printed a zero total: %s", got.Label)
			}
		})
	}
}

func TestDecideRefreshLostProviderDoesNotPublish(t *testing.T) {
	updated := time.Date(2026, 9, 25, 8, 0, 0, 0, time.UTC)
	prev := gateSnap("sha256:prev", StatusAvailable, "2026-09-25", intPtr(1000), measured("claude", 400), measured("cursor", 600))
	next := gateSnap("sha256:next", StatusAvailable, "2026-09-26", intPtr(5000), measured("claude", 5000))
	got := DecideRefresh(RefreshInput{Local: next, Remote: &prev, RemoteUpdatedAt: updated, Now: updated.Add(2 * time.Hour)})
	if got.Publish {
		t.Fatalf("lost provider published: %+v", got)
	}
	if got.Label != "覆盖变弱，保留上次" {
		t.Fatalf("label %q", got.Label)
	}
}

func TestDecideRefreshLocalDateNotUTC(t *testing.T) {
	shanghai := time.FixedZone("CST", 8*3600)
	// 2026-09-24 18:00 UTC is 2026-09-25 02:00 in Shanghai.
	instant := time.Date(2026, 9, 24, 18, 0, 0, 0, time.UTC)
	local := instant.In(shanghai).Format("2006-01-02")
	utcDate := instant.UTC().Format("2006-01-02")
	if local != "2026-09-25" || utcDate != "2026-09-24" {
		t.Fatalf("fixture dates local=%s utc=%s", local, utcDate)
	}
	remote := gateSnap("sha256:prev", StatusAvailable, utcDate, intPtr(1000))
	updated := instant.Add(-2 * time.Hour)
	got := DecideRefresh(RefreshInput{
		Local:           gateSnap("sha256:next", StatusAvailable, local, intPtr(1000)),
		Remote:          &remote,
		RemoteUpdatedAt: updated,
		Now:             instant,
	})
	if !got.Publish || got.Code != CodeDateRollover {
		t.Fatalf("shanghai local date should publish: %+v", got)
	}
	if utc := DecideRefresh(RefreshInput{
		Local:           gateSnap("sha256:next", StatusAvailable, utcDate, intPtr(1000)),
		Remote:          &remote,
		RemoteUpdatedAt: updated,
		Now:             instant,
	}); utc.Publish {
		t.Fatal("UTC date matched the remote day and must not publish on a zero delta")
	}

	us := time.FixedZone("PDT", -7*3600)
	// 2026-09-25 05:00 UTC is 2026-09-24 22:00 in a UTC-7 zone.
	evening := time.Date(2026, 9, 25, 5, 0, 0, 0, time.UTC)
	usDate := evening.In(us).Format("2006-01-02")
	if usDate != "2026-09-24" || evening.UTC().Format("2006-01-02") != "2026-09-25" {
		t.Fatalf("us dates local=%s utc=%s", usDate, evening.UTC().Format("2006-01-02"))
	}
	prev := gateSnap("sha256:prev", StatusAvailable, "2026-09-24", intPtr(1000))
	held := DecideRefresh(RefreshInput{
		Local:           gateSnap("sha256:next", StatusAvailable, usDate, intPtr(1000)),
		Remote:          &prev,
		RemoteUpdatedAt: evening.Add(-2 * time.Hour),
		Now:             evening,
	})
	if held.Publish || held.Code != CodeSkippedDelta {
		t.Fatalf("negative offset local date must not roll over: %+v", held)
	}
}

func TestDecideRefreshMidnightDoesNotBypassCooldown(t *testing.T) {
	loc := time.FixedZone("CST", 8*3600)
	last := time.Date(2026, 9, 24, 23, 10, 0, 0, loc)
	early := time.Date(2026, 9, 25, 0, 5, 0, 0, loc)
	ready := time.Date(2026, 9, 25, 0, 10, 0, 0, loc)
	if early.Sub(last) >= time.Hour {
		t.Fatal("00:05 is inside the hour")
	}
	if ready.Sub(last) != time.Hour {
		t.Fatalf("00:10 sub=%s", ready.Sub(last))
	}
	remote := gateSnap("sha256:prev", StatusAvailable, "2026-09-24", intPtr(42_000))
	same := intPtr(42_000)
	earlyGot := DecideRefresh(RefreshInput{
		Local:           gateSnap("sha256:next", StatusAvailable, early.Format("2006-01-02"), same),
		Remote:          &remote,
		RemoteUpdatedAt: last,
		Now:             early,
	})
	if earlyGot.Publish || earlyGot.Code != CodeSkippedCooldown {
		t.Fatalf("midnight must not bypass cooldown: %+v", earlyGot)
	}
	readyGot := DecideRefresh(RefreshInput{
		Local:           gateSnap("sha256:next", StatusAvailable, ready.Format("2006-01-02"), same),
		Remote:          &remote,
		RemoteUpdatedAt: last,
		Now:             ready,
	})
	if !readyGot.Publish || readyGot.Code != CodeDateRollover {
		t.Fatalf("00:10 should publish the new local day: %+v", readyGot)
	}
	built, err := Build(Input{
		Now:     ready,
		Loc:     loc,
		Version: "test",
		Events: []event.UsageEvent{{
			Source: "claude", Vendor: "anthropic", Timestamp: last, Miss: 42_000, Quality: event.QualityAuthoritative,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if built.AsOfDate != "2026-09-25" {
		t.Fatalf("build as_of_date=%s", built.AsOfDate)
	}
}

func TestBuildAsOfDateFollowsLocalZone(t *testing.T) {
	ev := event.UsageEvent{Source: "claude", Vendor: "anthropic", Timestamp: time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC), Miss: 10, Quality: event.QualityAuthoritative}
	shanghai := time.FixedZone("CST", 8*3600)
	snap, err := Build(Input{Now: time.Date(2026, 9, 24, 18, 0, 0, 0, time.UTC), Loc: shanghai, Version: "test", Events: []event.UsageEvent{ev}})
	if err != nil {
		t.Fatal(err)
	}
	if snap.AsOfDate != "2026-09-25" {
		t.Fatalf("shanghai as_of=%s", snap.AsOfDate)
	}
	us := time.FixedZone("PDT", -7*3600)
	snap, err = Build(Input{Now: time.Date(2026, 9, 25, 5, 0, 0, 0, time.UTC), Loc: us, Version: "test", Events: []event.UsageEvent{ev}})
	if err != nil {
		t.Fatal(err)
	}
	if snap.AsOfDate != "2026-09-24" {
		t.Fatalf("us as_of=%s", snap.AsOfDate)
	}
}

func TestDecideRefreshMissingUpdatedAtDoesNotPublish(t *testing.T) {
	remote := gateSnap("sha256:prev", StatusAvailable, "2026-09-25", intPtr(1_000_000))
	now := time.Date(2026, 9, 26, 8, 0, 0, 0, time.UTC)
	got := DecideRefresh(RefreshInput{
		Local:           gateSnap("sha256:next", StatusAvailable, "2026-09-26", intPtr(1_000_000+50_000)),
		Remote:          &remote,
		RemoteUpdatedAt: time.Time{},
		Now:             now,
	})
	if got.Publish || got.Code != CodeSkippedCooldown {
		t.Fatalf("missing updated_at must not publish: %+v", got)
	}
}

func TestDecideRefreshEarlierLocalDateIsNotRollover(t *testing.T) {
	updated := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	remote := gateSnap("sha256:prev", StatusAvailable, "2026-09-26", intPtr(1_000_000))
	earlier := DecideRefresh(RefreshInput{
		Local:           gateSnap("sha256:next", StatusAvailable, "2026-09-25", intPtr(1_000_000)),
		Remote:          &remote,
		RemoteUpdatedAt: updated,
		Now:             updated.Add(2 * time.Hour),
	})
	if earlier.Publish || earlier.Code == CodeDateRollover {
		t.Fatalf("earlier local date published: %+v", earlier)
	}
	equal := DecideRefresh(RefreshInput{
		Local:           gateSnap("sha256:next", StatusAvailable, "2026-09-26", intPtr(1_000_000)),
		Remote:          &remote,
		RemoteUpdatedAt: updated,
		Now:             updated.Add(2 * time.Hour),
	})
	if equal.Publish || equal.Code == CodeDateRollover {
		t.Fatalf("equal date is not a rollover: %+v", equal)
	}
	invalid := DecideRefresh(RefreshInput{
		Local:           gateSnap("sha256:next", StatusAvailable, "09/27/2026", intPtr(1_000_000)),
		Remote:          &remote,
		RemoteUpdatedAt: updated,
		Now:             updated.Add(2 * time.Hour),
	})
	if invalid.Publish || invalid.Code == CodeDateRollover {
		t.Fatalf("invalid date published: %+v", invalid)
	}
}

func TestDecideRefreshDSTFallbackUsesAbsoluteHour(t *testing.T) {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	// 2026-11-01 01:30 EDT, then one absolute hour later is 01:30 EST.
	start := time.Date(2026, 11, 1, 5, 30, 0, 0, time.UTC)
	early := start.Add(time.Hour - time.Nanosecond)
	ready := start.Add(time.Hour)
	if ready.Sub(start) != time.Hour || early.Sub(start) >= time.Hour {
		t.Fatalf("absolute sub early=%s ready=%s", early.Sub(start), ready.Sub(start))
	}
	if _, off := start.In(loc).Zone(); off != -4*3600 {
		t.Fatalf("start zone offset %d", off)
	}
	if _, off := ready.In(loc).Zone(); off != -5*3600 {
		t.Fatalf("ready zone offset %d", off)
	}
	if start.In(loc).Format("15:04") != "01:30" || ready.In(loc).Format("15:04") != "01:30" {
		t.Fatalf("civil clocks %s %s", start.In(loc), ready.In(loc))
	}
	remote := gateSnap("sha256:prev", StatusAvailable, "2026-11-01", intPtr(1_000_000))
	cooldown := DecideRefresh(RefreshInput{
		Local:           gateSnap("sha256:next", StatusAvailable, "2026-11-01", intPtr(1_000_000+10_000)),
		Remote:          &remote,
		RemoteUpdatedAt: start,
		Now:             early,
	})
	if cooldown.Publish || cooldown.Code != CodeSkippedCooldown {
		t.Fatalf("1h-1ns during fallback published: %+v", cooldown)
	}
	small := DecideRefresh(RefreshInput{
		Local:           gateSnap("sha256:next", StatusAvailable, "2026-11-01", intPtr(1_000_000+9_999)),
		Remote:          &remote,
		RemoteUpdatedAt: start,
		Now:             ready,
	})
	if small.Publish || small.Code != CodeSkippedDelta {
		t.Fatalf("+9999 at 1h published: %+v", small)
	}
	got := DecideRefresh(RefreshInput{
		Local:           gateSnap("sha256:next", StatusAvailable, "2026-11-01", intPtr(1_000_000+10_000)),
		Remote:          &remote,
		RemoteUpdatedAt: start,
		Now:             ready,
	})
	if !got.Publish || got.Code != CodePublished {
		t.Fatalf("+10000 at absolute 1h: %+v", got)
	}
}
