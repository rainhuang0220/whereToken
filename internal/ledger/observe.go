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
	if err != nil {
		return err
	}
	_, err = l.db.Exec(`CREATE TABLE IF NOT EXISTS api_window_audits (
  source TEXT NOT NULL,
  window_from TEXT NOT NULL,
  window_to TEXT NOT NULL,
  declared INTEGER NOT NULL,
  raw_rows INTEGER NOT NULL,
  deduped INTEGER NOT NULL,
  days INTEGER NOT NULL,
  miss INTEGER NOT NULL,
  cache_read INTEGER NOT NULL,
  cache_create INTEGER NOT NULL,
  output INTEGER NOT NULL,
  validation TEXT NOT NULL,
  observed_at TEXT NOT NULL,
  PRIMARY KEY (source, window_from, window_to)
)`)
	return err
}

// WindowAudit is the desensitized count check for one filtered window.
// Validation "missing" is computed when no row exists. It is not stored,
// and its zero components are not a measured zero.
type WindowAudit struct {
	From, To                             string
	Declared, RawRows, Deduped, Days     int
	Miss, CacheRead, CacheCreate, Output int64
	Validation                           string
}

// NoteWindowAudit stores one successful window check. The same window
// replaces its previous row. "missing" is not a value this method accepts.
func (l *Ledger) NoteWindowAudit(source string, a WindowAudit) error {
	if source == "" || a.From == "" || a.To == "" || a.From > a.To {
		return errors.New("ledger observation rejected")
	}
	if a.Validation != "ok" && a.Validation != "open" {
		return errors.New("ledger observation rejected")
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	_, err := l.db.Exec(`INSERT OR REPLACE INTO api_window_audits(
		source,window_from,window_to,declared,raw_rows,deduped,days,
		miss,cache_read,cache_create,output,validation,observed_at
	) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		source, a.From, a.To, a.Declared, a.RawRows, a.Deduped, a.Days,
		a.Miss, a.CacheRead, a.CacheCreate, a.Output, a.Validation,
		time.Now().UTC().Format(time.RFC3339Nano))
	return err
}

// WindowAudit returns the stored check for one window. A window this
// process never audited comes back as validation "missing" and is not
// written. Callers must not treat that row as a measured zero.
func (l *Ledger) WindowAudit(source, from, to string) (WindowAudit, error) {
	missing := WindowAudit{From: from, To: to, Validation: "missing"}
	if source == "" || from == "" || to == "" {
		return missing, nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	var a WindowAudit
	err := l.db.QueryRow(`SELECT window_from, window_to, declared, raw_rows, deduped, days,
		miss, cache_read, cache_create, output, validation
		FROM api_window_audits WHERE source=? AND window_from=? AND window_to=?`,
		source, from, to).Scan(&a.From, &a.To, &a.Declared, &a.RawRows, &a.Deduped, &a.Days,
		&a.Miss, &a.CacheRead, &a.CacheCreate, &a.Output, &a.Validation)
	if errors.Is(err, sql.ErrNoRows) {
		return missing, nil
	}
	if err != nil {
		return WindowAudit{}, err
	}
	return a, nil
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
	already := map[string]struct{}{}
	for _, e := range events {
		if e.Source == "" || e.Timestamp.IsZero() {
			continue
		}
		if e.Derivation != event.DeriveProviderAPI && e.Derivation != event.DeriveRetained {
			continue
		}
		already[e.Source+"|"+e.Timestamp.UTC().Format("2006-01-02")] = struct{}{}
	}
	out := append([]event.UsageEvent(nil), events...)
	for source, evs := range fresh {
		extra, err := l.daysBeforeFresh(source, evs, already)
		if err != nil {
			return nil, err
		}
		out = append(out, extra...)
	}
	return out, nil
}

func (l *Ledger) daysBeforeFresh(source string, fresh []event.UsageEvent, already map[string]struct{}) ([]event.UsageEvent, error) {
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
	l.mu.Lock()
	defer l.mu.Unlock()
	obs, err := l.observationsAsc(source, apiPayloadHash(fresh))
	if err != nil || len(obs) == 0 {
		return nil, err
	}
	days := chainDayMap(obs)
	var keys []string
	for day, d := range days {
		if day >= freshFrom {
			continue
		}
		if _, ok := already[source+"|"+day]; ok {
			continue
		}
		if d.Miss+d.CacheRead+d.CacheCreate+d.Output == 0 {
			continue
		}
		keys = append(keys, day)
	}
	sort.Strings(keys)
	var out []event.UsageEvent
	for _, day := range keys {
		d := days[day]
		out = append(out, retainedEvent(source, day, d.Miss, d.CacheRead, d.CacheCreate, d.Output))
	}
	return out, nil
}

// DayRange is an inclusive UTC calendar span that a fresh account window covered.
type DayRange struct {
	From, To string
}

// NoteCoveredWindow stores one completed filtered window, including a
// measured-empty window. Days inside the window replace older values.
// Days outside it stay on earlier observations.
func (l *Ledger) NoteCoveredWindow(source, from, to string, events []event.UsageEvent) error {
	if source == "" || from == "" || to == "" || from > to {
		return errors.New("ledger observation rejected")
	}
	var kept []dayComp
	var tokens int64
	for _, d := range groupDays(events) {
		if d.Day < from || d.Day > to {
			continue
		}
		sum := d.Miss + d.CacheRead + d.CacheCreate + d.Output
		if sum == 0 {
			continue
		}
		tokens += sum
		kept = append(kept, d)
	}
	lines := []string{"window|" + from + "|" + to}
	for _, d := range kept {
		lines = append(lines, fmt.Sprintf("%s|%d|%d|%d|%d", d.Day, d.Miss, d.CacheRead, d.CacheCreate, d.Output))
	}
	sum := sha256.New()
	for _, line := range lines {
		sum.Write([]byte(line))
		sum.Write([]byte{'\n'})
	}
	hash := hex.EncodeToString(sum.Sum(nil))
	meta := observationMeta{
		ObservedAt: time.Now().UTC().Format(time.RFC3339Nano),
		WindowFrom: from,
		WindowTo:   to,
		Coverage:   "window",
		Origin:     "api",
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	tx, err := l.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := insertObservation(tx, source, hash, tokens, len(kept), meta, kept); err != nil {
		return err
	}
	return tx.Commit()
}

// HasObservation reports whether source has a stored observation.
// A retained snapshot counts. An empty ledger does not.
func (l *Ledger) HasObservation(source string) (bool, error) {
	if source == "" {
		return false, nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	var one int
	err := l.db.QueryRow(`SELECT 1 FROM api_observations WHERE source=? AND observed_at != '' LIMIT 1`, source).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

// CoveredRanges lists finished filtered windows for source, oldest first.
// A retained snapshot's first and last day is not one of these windows:
// days between those endpoints were not given a token total.
func (l *Ledger) CoveredRanges(source string) ([]DayRange, error) {
	if source == "" {
		return nil, nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.finishedWindowsLocked(source)
}

func (l *Ledger) finishedWindowsLocked(source string) ([]DayRange, error) {
	rows, err := l.db.Query(`SELECT window_from, window_to FROM api_observations
		WHERE source=? AND observed_at != '' AND window_from != '' AND window_to != ''
		  AND origin='api' AND coverage='window'
		ORDER BY observed_at, rowid`, source)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []DayRange
	for rows.Next() {
		var r DayRange
		if err := rows.Scan(&r.From, &r.To); err != nil {
			return nil, err
		}
		if r.From == "" || r.To == "" || r.From > r.To {
			continue
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// DaysOutside returns chained observation days that this scan did not cover.
// A day already present on events is not returned again.
func (l *Ledger) DaysOutside(source string, ranges []DayRange, already []event.UsageEvent) ([]event.UsageEvent, error) {
	if source == "" {
		return nil, nil
	}
	have := map[string]struct{}{}
	for _, e := range already {
		if e.Source != source || e.Timestamp.IsZero() {
			continue
		}
		if e.Derivation != event.DeriveProviderAPI && e.Derivation != event.DeriveRetained {
			continue
		}
		have[e.Timestamp.UTC().Format("2006-01-02")] = struct{}{}
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	obs, err := l.observationsAsc(source, "")
	if err != nil || len(obs) == 0 {
		return nil, err
	}
	days := chainDayMap(obs)
	var keys []string
	for day, d := range days {
		if _, ok := have[day]; ok || dayInside(day, ranges) {
			continue
		}
		if d.Miss+d.CacheRead+d.CacheCreate+d.Output == 0 {
			continue
		}
		keys = append(keys, day)
	}
	sort.Strings(keys)
	var out []event.UsageEvent
	for _, day := range keys {
		d := days[day]
		out = append(out, retainedEvent(source, day, d.Miss, d.CacheRead, d.CacheCreate, d.Output))
	}
	return out, nil
}

// AccountSupplement returns stored days this scan did not fetch.
// A day inside the current lookback and inside a finished filtered window
// is a cached API total. A day outside that lookback, or a day that only
// exists on a retained snapshot, stays a retained observation. A calendar
// day with no component row is not created.
func (l *Ledger) AccountSupplement(source string, lookback DayRange, fetched []DayRange, already []event.UsageEvent) ([]event.UsageEvent, error) {
	if source == "" {
		return nil, nil
	}
	have := map[string]struct{}{}
	for _, e := range already {
		if e.Source != source || e.Timestamp.IsZero() {
			continue
		}
		if e.Derivation != event.DeriveProviderAPI && e.Derivation != event.DeriveRetained {
			continue
		}
		have[e.Timestamp.UTC().Format("2006-01-02")] = struct{}{}
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	obs, err := l.observationsAsc(source, "")
	if err != nil || len(obs) == 0 {
		return nil, err
	}
	finished, err := l.finishedWindowsLocked(source)
	if err != nil {
		return nil, err
	}
	days := chainDayMap(obs)
	var keys []string
	for day, d := range days {
		if _, ok := have[day]; ok || dayInside(day, fetched) {
			continue
		}
		if d.Miss+d.CacheRead+d.CacheCreate+d.Output == 0 {
			continue
		}
		keys = append(keys, day)
	}
	sort.Strings(keys)
	insideLookback := lookback.From != "" && lookback.To != "" && lookback.From <= lookback.To
	var out []event.UsageEvent
	for _, day := range keys {
		d := days[day]
		cached := insideLookback && day >= lookback.From && day <= lookback.To && dayInside(day, finished)
		ev := retainedEvent(source, day, d.Miss, d.CacheRead, d.CacheCreate, d.Output)
		if cached {
			ev.Derivation = event.DeriveProviderAPI
			ev.Quality = event.QualityAuthoritative
			ev.RequestID = "cursor-api:covered:" + day
		}
		out = append(out, ev)
	}
	return out, nil
}

// ChainedRetainedEvents folds every positive observation for source.
// A later window replaces its own days. Earlier days stay. A missing
// chain is false and no events, never a synthetic zero.
func (l *Ledger) ChainedRetainedEvents(source string) ([]event.UsageEvent, bool, error) {
	if source == "" {
		return nil, false, nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	obs, err := l.observationsAsc(source, "")
	if err != nil || len(obs) == 0 {
		return nil, false, err
	}
	days := chainDayMap(obs)
	var keys []string
	for day, d := range days {
		if d.Miss+d.CacheRead+d.CacheCreate+d.Output == 0 {
			continue
		}
		keys = append(keys, day)
	}
	if len(keys) == 0 {
		return nil, false, nil
	}
	sort.Strings(keys)
	out := make([]event.UsageEvent, 0, len(keys))
	for _, day := range keys {
		d := days[day]
		out = append(out, retainedEvent(source, day, d.Miss, d.CacheRead, d.CacheCreate, d.Output))
	}
	return out, true, nil
}

type storedObservation struct {
	hash     string
	from, to string
	days     []dayComp
}

func (l *Ledger) observationsAsc(source, exclude string) ([]storedObservation, error) {
	q := `SELECT payload_hash, window_from, window_to FROM api_observations WHERE source=? AND observed_at != ''`
	args := []any{source}
	if exclude != "" {
		q += ` AND payload_hash != ?`
		args = append(args, exclude)
	}
	q += ` ORDER BY observed_at ASC, rowid ASC`
	rows, err := l.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	var obs []storedObservation
	for rows.Next() {
		var o storedObservation
		if err := rows.Scan(&o.hash, &o.from, &o.to); err != nil {
			rows.Close()
			return nil, err
		}
		obs = append(obs, o)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	for i := range obs {
		days, err := l.observationDays(source, obs[i].hash)
		if err != nil {
			return nil, err
		}
		obs[i].days = days
	}
	return obs, nil
}

func (l *Ledger) observationDays(source, hash string) ([]dayComp, error) {
	rows, err := l.db.Query(`SELECT day, miss, cache_read, cache_create, output FROM api_observation_days
		WHERE source=? AND payload_hash=? ORDER BY day`, source, hash)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []dayComp
	for rows.Next() {
		var d dayComp
		if err := rows.Scan(&d.Day, &d.Miss, &d.CacheRead, &d.CacheCreate, &d.Output); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func chainDayMap(obs []storedObservation) map[string]dayComp {
	days := map[string]dayComp{}
	for _, ob := range obs {
		if ob.from != "" && ob.to != "" && ob.from <= ob.to {
			for day := range days {
				if day >= ob.from && day <= ob.to {
					delete(days, day)
				}
			}
		}
		for _, d := range ob.days {
			if d.Miss+d.CacheRead+d.CacheCreate+d.Output == 0 {
				continue
			}
			days[d.Day] = d
		}
	}
	return days
}

func dayInside(day string, ranges []DayRange) bool {
	for _, r := range ranges {
		if r.From != "" && r.To != "" && day >= r.From && day <= r.To {
			return true
		}
	}
	return false
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
