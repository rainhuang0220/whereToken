package cursor

import (
	"errors"
	"fmt"
	"time"

	"github.com/rainhuang0220/whereToken/internal/adapter"
	"github.com/rainhuang0220/whereToken/internal/event"
)

// AttributionBatch contains only complete, independently paged UTC windows.
// It is deliberately separate from ordinary scan coverage: revisiting history
// for attribution must not replace the protected historical token baseline.
type AttributionBatch struct {
	Events  []event.UsageEvent
	Windows []CoverageWindow
}

// FetchAttribution reads only the current session, not Cursor's conversation
// SQLite. The caller must supply an account guard before any authenticated
// request can be made. Session material never leaves this adapter.
func (a Adapter) FetchAttribution(home adapter.Home, from, to string) (AttributionBatch, error) {
	if a.Offline || a.AcceptSubject == nil {
		return AttributionBatch{}, errAccountUsageUnavailable
	}
	if a.ConsentPath == "" {
		a.ConsentPath = ConsentPath(home)
	}
	if !consentAllows(a.ConsentPath) {
		return AttributionBatch{}, errOSPermission
	}
	roots := a.Discover(home)
	if len(roots) != 1 {
		return AttributionBatch{}, errNoLocalAuth
	}
	path := resolveDB(roots[0].Path)
	token, _, err := authTokens(path, false)
	if err != nil || token == "" {
		return AttributionBatch{}, errNoLocalAuth
	}
	token, err = a.openSealed(token, path)
	if err != nil {
		return AttributionBatch{}, err
	}
	if !usableSessionToken(token) {
		return AttributionBatch{}, errTokenRejected
	}
	if err = a.AcceptSubject(JWTSubject(token)); err != nil {
		return AttributionBatch{}, err
	}
	return a.fetchAttributionWindows(token, from, to)
}

func (a Adapter) fetchAttributionWindows(token, from, to string) (AttributionBatch, error) {
	var out AttributionBatch
	start, err := time.Parse("2006-01-02", from)
	if err != nil {
		return out, fmt.Errorf("invalid UTC attribution start date")
	}
	last, err := time.Parse("2006-01-02", to)
	if err != nil || last.Before(start) || last.Sub(start) > 2*366*24*time.Hour {
		return out, fmt.Errorf("invalid UTC attribution range (maximum two years)")
	}
	end := last.AddDate(0, 0, 1)
	now := a.now().UTC()
	if !start.Before(now) {
		return out, fmt.Errorf("attribution range has no elapsed UTC time")
	}
	if end.After(now) {
		end = now
	}
	seen := map[string]struct{}{}
	var failures []error
	for cur := start; cur.Before(end); {
		next := time.Date(cur.Year(), cur.Month()+1, 1, 0, 0, 0, 0, time.UTC)
		if next.After(end) {
			next = end
		}
		span := usageSpan{start: cur, end: next, inclusive: next.Equal(now)}
		got, fetchErr := a.fetchFilteredWindow(token, span, 0)
		billing := errors.Is(fetchErr, errSchemaChanged)
		if billing {
			got, fetchErr = a.fetchBillingWindow(token, span)
		}
		unavailable := errors.Is(fetchErr, errBillingAvailability) || errors.Is(fetchErr, errTeamSelectionRequired)
		if billing && (unavailable || errors.Is(fetchErr, errSchemaChanged) || fetchErr == nil && len(got.events) == 0) {
			got, fetchErr = a.fetchCSVWindow(token, span)
		}
		if fetchErr == nil {
			for _, e := range got.events {
				if e.Timestamp.IsZero() || e.Timestamp.Before(span.start) || e.Timestamp.After(span.end) || !span.inclusive && e.Timestamp.Equal(span.end) {
					fetchErr = fmt.Errorf("%w [reason=attribution_event_outside_window]", errSchemaChanged)
					break
				}
			}
		}
		if fetchErr != nil {
			failures = append(failures, fmt.Errorf("UTC %s..%s: %w", span.coverage().From, span.coverage().To, fetchErr))
			// Availability is an independently missing month, not a reason to
			// discard later completed windows. Stop on auth/rate/network failures.
			if !errors.Is(fetchErr, errBillingAvailability) && !isNotFound(fetchErr) && !(unavailable && errors.Is(fetchErr, errSchemaChanged)) {
				break
			}
		} else if len(got.events) > 0 {
			var added []event.UsageEvent
			out.Events, added = dedupeUsage(out.Events, seen, got.events)
			cov := span.coverage()
			cov.Audit = auditWindow(cov, got.declared, got.raw, added, span.inclusive)
			out.Windows = append(out.Windows, cov)
		} else if !billing {
			cov := span.coverage()
			cov.Audit = auditWindow(cov, got.declared, got.raw, nil, span.inclusive)
			out.Windows = append(out.Windows, cov)
		} else {
			failures = append(failures, fmt.Errorf("UTC %s..%s: %w", span.coverage().From, span.coverage().To, errBillingAvailability))
		}
		cur = next
	}
	return out, errors.Join(failures...)
}
