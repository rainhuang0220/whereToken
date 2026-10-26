package ledger

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/rainhuang0220/whereToken/internal/event"
)

// An account-API observation is one confirmed response, kept whole.
// Reuse never max-merges components from two versions, and a missing
// observation is not a zero event.
type observationMeta struct {
	ObservedAt string
	WindowFrom string
	WindowTo   string
	Coverage   string
	Origin     string
}

type dayComp struct {
	Day                                  string
	Miss, CacheRead, CacheCreate, Output int64
}

func (l *Ledger) ensureObservationColumns() error {
	have := map[string]bool{}
	rows, err := l.db.Query(`PRAGMA table_info(api_observations)`)
	if err != nil {
		return err
	}
	for rows.Next() {
		var cid, notnull, pk int
		var name, ctype string
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			rows.Close()
			return err
		}
		have[name] = true
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	adds := []struct{ name, ddl string }{
		{"observed_at", `ALTER TABLE api_observations ADD COLUMN observed_at TEXT NOT NULL DEFAULT ''`},
		{"window_from", `ALTER TABLE api_observations ADD COLUMN window_from TEXT NOT NULL DEFAULT ''`},
		{"window_to", `ALTER TABLE api_observations ADD COLUMN window_to TEXT NOT NULL DEFAULT ''`},
		{"coverage", `ALTER TABLE api_observations ADD COLUMN coverage TEXT NOT NULL DEFAULT ''`},
		{"origin", `ALTER TABLE api_observations ADD COLUMN origin TEXT NOT NULL DEFAULT ''`},
	}
	for _, add := range adds {
		if have[add.name] {
			continue
		}
		if _, err := l.db.Exec(add.ddl); err != nil {
			return err
		}
	}
	_, err = l.db.Exec(`CREATE TABLE IF NOT EXISTS api_observation_days (
  source TEXT NOT NULL,
  payload_hash TEXT NOT NULL,
  day TEXT NOT NULL,
  miss INTEGER NOT NULL,
  cache_read INTEGER NOT NULL,
  cache_create INTEGER NOT NULL,
  output INTEGER NOT NULL,
  PRIMARY KEY (source, payload_hash, day)
)`)
	return err
}

func observationFromAPI(events []event.UsageEvent) (observationMeta, []dayComp) {
	meta := observationMeta{
		ObservedAt: time.Now().UTC().Format(time.RFC3339Nano),
		Coverage:   "complete",
		Origin:     "api",
	}
	days := groupDays(events)
	if len(days) > 0 {
		meta.WindowFrom = days[0].Day
		meta.WindowTo = days[len(days)-1].Day
	}
	return meta, days
}

func groupDays(events []event.UsageEvent) []dayComp {
	by := map[string]*dayComp{}
	for _, e := range events {
		if e.Timestamp.IsZero() {
			continue
		}
		day := e.Timestamp.UTC().Format("2006-01-02")
		row := by[day]
		if row == nil {
			row = &dayComp{Day: day}
			by[day] = row
		}
		row.Miss += e.Miss
		row.CacheRead += e.CacheRead
		row.CacheCreate += e.CacheCreate
		row.Output += e.Output
	}
	out := make([]dayComp, 0, len(by))
	for _, row := range by {
		out = append(out, *row)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Day < out[j].Day })
	return out
}

func insertObservation(tx *sql.Tx, source, hash string, tokens int64, n int, meta observationMeta, days []dayComp) error {
	res, err := tx.Exec(`INSERT OR IGNORE INTO api_observations(
		source,payload_hash,tokens,events,parser_version,observed_at,window_from,window_to,coverage,origin
	) VALUES(?,?,?,?,?,?,?,?,?,?)`,
		source, hash, tokens, n, ParserVersion, meta.ObservedAt, meta.WindowFrom, meta.WindowTo, meta.Coverage, meta.Origin)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return nil
	}
	for _, d := range days {
		if d.Miss+d.CacheRead+d.CacheCreate+d.Output == 0 {
			continue
		}
		if _, err := tx.Exec(`INSERT INTO api_observation_days(source,payload_hash,day,miss,cache_read,cache_create,output) VALUES(?,?,?,?,?,?,?)`,
			source, hash, d.Day, d.Miss, d.CacheRead, d.CacheCreate, d.Output); err != nil {
			return err
		}
	}
	return nil
}

// SaveObservation records one confirmed account observation. The same day
// vector twice is one row. A non-positive total is not stored and is not
// turned into a zero event. observedAt is the time the observation was
// confirmed, not the time it is reused.
func (l *Ledger) SaveObservation(source, origin, coverage string, observedAt time.Time, events []event.UsageEvent) error {
	if source == "" || origin == "" || coverage == "" || observedAt.IsZero() {
		return errors.New("ledger observation rejected")
	}
	days := groupDays(events)
	if len(days) == 0 {
		return nil
	}
	var tokens int64
	lines := make([]string, 0, len(days))
	for _, d := range days {
		tokens += d.Miss + d.CacheRead + d.CacheCreate + d.Output
		lines = append(lines, fmt.Sprintf("%s|%d|%d|%d|%d", d.Day, d.Miss, d.CacheRead, d.CacheCreate, d.Output))
	}
	if tokens <= 0 {
		return nil
	}
	sum := sha256.New()
	for _, line := range lines {
		sum.Write([]byte(line))
		sum.Write([]byte{'\n'})
	}
	hash := hex.EncodeToString(sum.Sum(nil))
	meta := observationMeta{
		ObservedAt: observedAt.UTC().Format(time.RFC3339Nano),
		WindowFrom: days[0].Day,
		WindowTo:   days[len(days)-1].Day,
		Coverage:   coverage,
		Origin:     origin,
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	tx, err := l.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := insertObservation(tx, source, hash, tokens, len(days), meta, days); err != nil {
		return err
	}
	return tx.Commit()
}

// LatestRetainedEvents returns the newest positive observation for source.
// The events are marked retained. A missing row returns false and no events,
// never a synthetic zero. Components come from that one observation.
func (l *Ledger) LatestRetainedEvents(source string) ([]event.UsageEvent, bool, error) {
	if source == "" {
		return nil, false, nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	var hash string
	err := l.db.QueryRow(`SELECT payload_hash FROM api_observations
		WHERE source=? AND observed_at != '' AND tokens > 0
		ORDER BY observed_at DESC, rowid DESC LIMIT 1`, source).Scan(&hash)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	rows, err := l.db.Query(`SELECT day, miss, cache_read, cache_create, output FROM api_observation_days
		WHERE source=? AND payload_hash=? ORDER BY day`, source, hash)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()
	var out []event.UsageEvent
	for rows.Next() {
		var day string
		var miss, cr, cc, outN int64
		if err := rows.Scan(&day, &miss, &cr, &cc, &outN); err != nil {
			return nil, false, err
		}
		if miss+cr+cc+outN == 0 {
			continue
		}
		out = append(out, retainedEvent(source, day, miss, cr, cc, outN))
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}
	if len(out) == 0 {
		return nil, false, nil
	}
	return out, true, nil
}

// SupplementNarrowWindow keeps days from the previous positive observation
// that fall before a newer, narrower account-API window. Days inside the new
// window stay on the new observation. Components are not max-merged.
func (l *Ledger) SupplementNarrowWindow(events []event.UsageEvent) ([]event.UsageEvent, error) {
	fresh := map[string][]event.UsageEvent{}
	for _, e := range events {
		if e.Derivation == event.DeriveProviderAPI && e.Source != "" {
			fresh[e.Source] = append(fresh[e.Source], e)
		}
	}
	if len(fresh) == 0 {
		return events, nil
	}
	out := append([]event.UsageEvent(nil), events...)
	for source, evs := range fresh {
		extra, err := l.daysBeforeFresh(source, evs)
		if err != nil {
			return nil, err
		}
		out = append(out, extra...)
	}
	return out, nil
}

func (l *Ledger) daysBeforeFresh(source string, fresh []event.UsageEvent) ([]event.UsageEvent, error) {
	freshFrom := ""
	for _, e := range fresh {
		if e.Timestamp.IsZero() {
			continue
		}
		day := e.Timestamp.UTC().Format("2006-01-02")
		if freshFrom == "" || day < freshFrom {
			freshFrom = day
		}
	}
	if freshFrom == "" {
		return nil, nil
	}
	hash := apiPayloadHash(fresh)
	var prevHash, prevFrom string
	err := l.db.QueryRow(`SELECT payload_hash, window_from FROM api_observations
		WHERE source=? AND observed_at != '' AND tokens > 0 AND payload_hash != ?
		ORDER BY observed_at DESC, rowid DESC LIMIT 1`, source, hash).Scan(&prevHash, &prevFrom)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if prevFrom == "" || prevFrom >= freshFrom {
		return nil, nil
	}
	rows, err := l.db.Query(`SELECT day, miss, cache_read, cache_create, output FROM api_observation_days
		WHERE source=? AND payload_hash=? AND day < ? ORDER BY day`, source, prevHash, freshFrom)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []event.UsageEvent
	for rows.Next() {
		var day string
		var miss, cr, cc, outN int64
		if err := rows.Scan(&day, &miss, &cr, &cc, &outN); err != nil {
			return nil, err
		}
		if miss+cr+cc+outN == 0 {
			continue
		}
		out = append(out, retainedEvent(source, day, miss, cr, cc, outN))
	}
	return out, rows.Err()
}

func apiPayloadHash(events []event.UsageEvent) string {
	lines := make([]string, 0, len(events))
	for _, e := range events {
		if e.Derivation != event.DeriveProviderAPI || e.Source == "" {
			continue
		}
		lines = append(lines, fmt.Sprintf("%s|%d|%d|%d|%d|%d", e.Model, e.Timestamp.UnixNano(), e.Miss, e.CacheRead, e.CacheCreate, e.Output))
	}
	sort.Strings(lines)
	sum := sha256.New()
	for _, line := range lines {
		sum.Write([]byte(line))
		sum.Write([]byte{'\n'})
	}
	return hex.EncodeToString(sum.Sum(nil))
}

func retainedEvent(source, day string, miss, cacheRead, cacheCreate, output int64) event.UsageEvent {
	ts, _ := time.Parse("2006-01-02", day)
	// Noon UTC keeps the calendar day stable for the time zones the profile uses.
	ts = ts.Add(12 * time.Hour)
	return event.UsageEvent{
		Source:      source,
		RequestID:   "retained:" + day,
		Timestamp:   ts,
		Miss:        miss,
		CacheRead:   cacheRead,
		CacheCreate: cacheCreate,
		Output:      output,
		Quality:     event.QualityDegraded,
		Derivation:  event.DeriveRetained,
		SkipRequest: true,
	}
}
