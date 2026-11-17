package scan

import (
	"time"

	"github.com/rainhuang0220/whereToken/internal/adapter/cursor"
	"github.com/rainhuang0220/whereToken/internal/event"
	"github.com/rainhuang0220/whereToken/internal/ledger"
	"github.com/rainhuang0220/whereToken/internal/publicprofile"
)

const collectionDay = "2006-01-02"

type collectionRun struct {
	at     time.Time
	now    func() time.Time
	checks []publicprofile.SourceCheck
}

func (r *collectionRun) add(id string, roots int, category string) {
	if r == nil || (roots == 0 && category == "") {
		return
	}
	check := publicprofile.SourceCheck{
		SourceCoverage: publicprofile.SourceCoverage{Source: id, Status: publicprofile.SourceFresh},
		LastAttemptAt:  r.at.UTC(),
		Category:       category,
	}
	if category != "" {
		check.Status = publicprofile.SourceUnavailable
	} else {
		check.LastObservationAt = &check.LastAttemptAt
	}
	r.checks = append(r.checks, check)
}

func (r *collectionRun) finish(lg *ledger.Ledger, cursorSource, cursorCategory string, cursorExpected, observedCursor bool) *publicprofile.CollectionEvidence {
	if r == nil {
		return nil
	}
	if cursorExpected || observedCursor {
		r.checks = append(r.checks, cursorCheck(r.at.UTC(), r.now().UTC(), lg, cursorSource, cursorCategory, observedCursor))
	}
	return &publicprofile.CollectionEvidence{Sources: r.checks}
}

func cursorCheck(at, now time.Time, lg *ledger.Ledger, namespace, category string, observed bool) publicprofile.SourceCheck {
	check := publicprofile.SourceCheck{
		SourceCoverage: publicprofile.SourceCoverage{Source: "cursor", Status: publicprofile.SourceUnknown},
		LastAttemptAt:  at,
		Category:       category,
	}
	// Start at UTC midnight so today is counted separately as an open day.
	to := now.Truncate(24 * time.Hour)
	from := to.AddDate(0, 0, -publicprofile.WallDays)
	if lg == nil {
		if category == "" {
			check.Category = "LEDGER"
		}
	} else {
		windows, err := lg.AuditedWindows(namespace, from.Format(collectionDay))
		if err != nil && category == "" {
			check.Category = "LEDGER"
		}
		latestZero := applyCursorCoverage(&check, windows, from, to, now)
		if observed && check.LastObservationAt != nil {
			switch {
			case len(check.GapsUTC) > 0:
				check.Status = publicprofile.SourceCoverageGap
			case latestZero:
				check.Status = publicprofile.SourceConfirmedZero
			default:
				check.Status = publicprofile.SourceFresh
			}
		} else if check.LastObservationAt != nil {
			check.Status = publicprofile.SourceStale
		}
	}
	switch check.Category {
	case "AUTH":
		check.Status = publicprofile.SourceAuthRequired
	case "COVERAGE":
		check.Status = publicprofile.SourceCoverageGap
	case "":
		if check.Status == publicprofile.SourceCoverageGap {
			check.Category = "COVERAGE"
		}
	default:
		check.Status = publicprofile.SourceUnavailable
	}
	return check
}

func applyCursorCoverage(check *publicprofile.SourceCheck, windows []ledger.AuditedWindow, from, to, now time.Time) bool {
	covered := make(map[string]bool, publicprofile.WallDays+1)
	var latestOpen time.Time
	var latestZero bool
	for _, w := range windows {
		start, err := time.Parse(collectionDay, w.From)
		if err != nil {
			continue
		}
		end, err := time.Parse(collectionDay, w.To)
		if err != nil || w.ObservedAt.After(now) {
			continue
		}
		if check.LastObservationAt == nil || w.ObservedAt.After(*check.LastObservationAt) {
			observed := w.ObservedAt
			check.LastObservationAt = &observed
		}
		if end.Equal(to) && w.ObservedAt.After(latestOpen) {
			latestOpen, latestZero = w.ObservedAt, w.Zero
		}
		if start.Before(from) {
			start = from
		}
		for day := start; !day.After(end) && !day.After(to); day = day.AddDate(0, 0, 1) {
			// An open window covers the current day only as of its
			// observation. A previous open day must be queried again.
			if !w.Open || day.Before(end) || day.Equal(to) {
				covered[day.Format(collectionDay)] = true
			}
		}
	}
	for day := from; !day.After(to); day = day.AddDate(0, 0, 1) {
		key := day.Format(collectionDay)
		if covered[key] {
			if day.Before(to) {
				check.CoverageThroughUTC = key
			}
			continue
		}
		gaps := check.GapsUTC
		if len(gaps) > 0 && gaps[len(gaps)-1].To == day.AddDate(0, 0, -1).Format(collectionDay) {
			check.GapsUTC[len(gaps)-1].To = key
		} else {
			check.GapsUTC = append(check.GapsUTC, publicprofile.UTCGap{From: key, To: key})
		}
	}
	return latestZero
}

func cursorCollectionExpected(c cursor.Adapter, lg *ledger.Ledger, namespace string) bool {
	if cursor.ConsentOn(c.ConsentPath) {
		return true
	}
	if lg != nil {
		exists, err := lg.HasObservation(namespace)
		return err == nil && exists
	}
	return false
}

func observedAt(c *collectionRun) time.Time {
	if c != nil {
		return c.now()
	}
	return time.Now()
}

func closedCursorWindows(lg *ledger.Ledger, source string) []cursor.CoverageWindow {
	audits, err := lg.AuditedWindows(source, "")
	if err != nil {
		return nil
	}
	out := make([]cursor.CoverageWindow, 0, len(audits))
	for _, window := range audits {
		to := window.To
		if window.Open {
			end, err := time.Parse(collectionDay, to)
			if err != nil {
				continue
			}
			to = end.AddDate(0, 0, -1).Format(collectionDay)
		}
		if window.From <= to {
			out = append(out, cursor.CoverageWindow{From: window.From, To: to})
		}
	}
	return out
}

func withoutRejectedCursorWindows(events []event.UsageEvent, rejected []ledger.DayRange) []event.UsageEvent {
	out := events[:0]
	for _, e := range events {
		if e.Source == "cursor" && e.Derivation == event.DeriveProviderAPI && !e.Timestamp.IsZero() {
			day := e.Timestamp.UTC().Format(collectionDay)
			skip := false
			for _, r := range rejected {
				if r.From <= day && day <= r.To {
					skip = true
					break
				}
			}
			if skip {
				continue
			}
		}
		out = append(out, e)
	}
	return out
}

func (r *collectionRun) ledgerFailure() {
	if r == nil {
		return
	}
	for i := range r.checks {
		check := &r.checks[i]
		if check.Category == "" {
			check.Status, check.Category = publicprofile.SourceUnavailable, "LEDGER"
			check.LastObservationAt = nil
		}
	}
}
