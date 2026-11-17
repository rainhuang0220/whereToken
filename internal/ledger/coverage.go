package ledger

import (
	"database/sql"
	"errors"
	"time"

	"github.com/rainhuang0220/whereToken/internal/event"
)

// AuditedWindow is a persisted, paginated observation. To may include
// the open UTC day when collected before midnight.
type AuditedWindow struct {
	DayRange
	ObservedAt time.Time
	Open       bool
	Zero       bool
}

// AuditedWindows reads window metadata only. Retained public snapshots,
// provisional results and legacy aggregate fallbacks prove no coverage.
func (l *Ledger) AuditedWindows(source, since string) ([]AuditedWindow, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	rows, err := l.db.Query(`SELECT a.window_from, a.window_to, a.observed_at, a.validation, a.declared, a.raw_rows
		FROM api_window_audits a
		WHERE a.source=? AND a.window_to>=? AND a.validation IN ('ok','open')
		AND EXISTS (SELECT 1 FROM api_observations o WHERE o.source=a.source AND o.window_from=a.window_from
		  AND o.window_to=a.window_to AND o.coverage='window' AND o.origin='api')
		ORDER BY a.window_from, a.window_to`, source, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var windows []AuditedWindow
	for rows.Next() {
		var w AuditedWindow
		var observed, validation string
		var declared, raw int
		if err := rows.Scan(&w.From, &w.To, &observed, &validation, &declared, &raw); err != nil {
			return nil, err
		}
		w.ObservedAt, err = time.Parse(time.RFC3339Nano, observed)
		if err != nil {
			return nil, err
		}
		w.Open = validation == "open"
		w.Zero = declared == 0 && raw == 0
		windows = append(windows, w)
	}
	return windows, rows.Err()
}

// ErrObservationRegression leaves the original evidence intact. API window
// reconciliation may add or reattribute usage. Reductions require a separate
// audited correction path.
var ErrObservationRegression = errors.New("account observation would reduce confirmed daily usage")

// NoteAuditedWindow persists coverage and the measured response together.
func (l *Ledger) NoteAuditedWindow(source string, a WindowAudit, at time.Time, events []event.UsageEvent) error {
	if source == "" || a.From == "" || a.To == "" || a.From > a.To || at.IsZero() ||
		(a.Validation != "ok" && a.Validation != "open") {
		return errors.New("ledger observation rejected")
	}
	return l.noteAccountWindow(source, a.From, a.To, at, events, &a)
}

func checkWindowTotals(tx *sql.Tx, source, from, to string, days []dayComp) error {
	newTotal := make(map[string]int64, len(days))
	for _, d := range days {
		newTotal[d.Day] = d.Miss + d.CacheRead + d.CacheCreate + d.Output
	}
	rows, err := tx.Query(`SELECT day, MAX(d.miss+d.cache_read+d.cache_create+d.output)
		FROM api_observation_days d JOIN api_observations o ON o.source=d.source AND o.payload_hash=d.payload_hash
		WHERE d.source=? AND d.day BETWEEN ? AND ? AND o.origin IN ('api','verified_snapshot')
		GROUP BY d.day`, source, from, to)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var day string
		var total int64
		if err := rows.Scan(&day, &total); err != nil {
			return err
		}
		if newTotal[day] < total {
			return ErrObservationRegression
		}
	}
	return rows.Err()
}
