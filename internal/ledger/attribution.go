package ledger

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/rainhuang0220/whereToken/internal/event"
)

// Detail rows contain only accounting fields. Request identities are hashed
// before persistence; the original identity, workspace, and path never enter
// the observation tables.
type observationDetail struct {
	Model, Vendor, RequestHash, Provenance string
	TS                                     int64
	Miss, CacheRead, CacheCreate, Output   int64
}

func (l *Ledger) ensureAttributionTables() error {
	_, err := l.db.Exec(`CREATE TABLE IF NOT EXISTS api_observation_details (
	 source TEXT NOT NULL, payload_hash TEXT NOT NULL, day TEXT NOT NULL,
	 ordinal INTEGER NOT NULL, model TEXT NOT NULL, vendor TEXT NOT NULL,
	 request_hash TEXT NOT NULL, ts INTEGER NOT NULL, provenance TEXT NOT NULL,
	 miss INTEGER NOT NULL, cache_read INTEGER NOT NULL, cache_create INTEGER NOT NULL, output INTEGER NOT NULL,
	 PRIMARY KEY(source,payload_hash,day,ordinal)
	)`)
	if err != nil {
		return err
	}
	_, err = l.db.Exec(`CREATE TABLE IF NOT EXISTS api_attribution_days (
	 source TEXT NOT NULL, day TEXT NOT NULL, baseline_hash TEXT NOT NULL, baseline_checksum TEXT NOT NULL,
	 PRIMARY KEY(source,day)
	);
	CREATE TABLE IF NOT EXISTS api_attribution_details (
	 source TEXT NOT NULL, day TEXT NOT NULL, ordinal INTEGER NOT NULL,
	 model TEXT NOT NULL, vendor TEXT NOT NULL, request_hash TEXT NOT NULL, ts INTEGER NOT NULL,
	 provenance TEXT NOT NULL, miss INTEGER NOT NULL, cache_read INTEGER NOT NULL,
	 cache_create INTEGER NOT NULL, output INTEGER NOT NULL,
	 PRIMARY KEY(source,day,ordinal)
	)`)
	return err
}

func digestString(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

func detailFromEvent(e event.UsageEvent) observationDetail {
	d := observationDetail{Model: e.Model, Vendor: e.Vendor, TS: e.Timestamp.UnixNano(),
		Miss: e.Miss, CacheRead: e.CacheRead, CacheCreate: e.CacheCreate, Output: e.Output,
		Provenance: endpointProvenance(e.Provenance)}
	if e.RequestID != "" {
		d.RequestHash = digestString(e.Source + "\x00" + e.RequestID)
	}
	return d
}

func detailEncoding(d observationDetail) string {
	b, _ := json.Marshal(d)
	return string(b)
}

func observationHash(prefix string, days []dayComp) string {
	lines := []string{prefix}
	for _, d := range days {
		lines = append(lines, fmt.Sprintf("%s|%d|%d|%d|%d", d.Day, d.Miss, d.CacheRead, d.CacheCreate, d.Output))
		for _, detail := range d.Details {
			lines = append(lines, detailEncoding(detail))
		}
	}
	return digestString(strings.Join(lines, "\n"))
}

func safeProvenance(origin string) string {
	switch origin {
	case "api", "verified_snapshot":
		return origin
	default:
		return "confirmed_observation"
	}
}

func endpointProvenance(value string) string {
	switch value {
	case "cursor_dashboard_csv", "cursor_billing_v2", "cursor_filtered_usage":
		return value
	default:
		return ""
	}
}

func storedProvenance(value string) string {
	if endpoint := endpointProvenance(value); endpoint != "" {
		return endpoint
	}
	switch value {
	case "api", "verified_snapshot", "confirmed_observation", "attribution_backfill", "history_unallocated":
		return value
	default:
		return ""
	}
}

func insertObservationDetails(tx *sql.Tx, source, hash, origin string, days []dayComp) error {
	for _, d := range days {
		for i, detail := range d.Details {
			provenance := endpointProvenance(detail.Provenance)
			if provenance == "" {
				provenance = safeProvenance(origin)
			}
			if _, err := tx.Exec(`INSERT INTO api_observation_details
			 (source,payload_hash,day,ordinal,model,vendor,request_hash,ts,provenance,miss,cache_read,cache_create,output)
			 VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)`, source, hash, d.Day, i, detail.Model, detail.Vendor, detail.RequestHash, detail.TS,
				provenance, detail.Miss, detail.CacheRead, detail.CacheCreate, detail.Output); err != nil {
				return err
			}
		}
	}
	return nil
}

func detailTotals(details []observationDetail) dayComp {
	var d dayComp
	for _, row := range details {
		d.Miss += row.Miss
		d.CacheRead += row.CacheRead
		d.CacheCreate += row.CacheCreate
		d.Output += row.Output
	}
	return d
}

func sameComponents(a, b dayComp) bool {
	return a.Miss == b.Miss && a.CacheRead == b.CacheRead && a.CacheCreate == b.CacheCreate && a.Output == b.Output
}

func retainedDayEvents(source string, d dayComp) []event.UsageEvent {
	if len(d.Details) == 0 || !sameComponents(d, detailTotals(d.Details)) {
		e := retainedEvent(source, d.Day, d.Miss, d.CacheRead, d.CacheCreate, d.Output)
		e.RequestID += ":history-unallocated"
		e.Provenance = "history_unallocated"
		return []event.UsageEvent{e}
	}
	out := make([]event.UsageEvent, 0, len(d.Details))
	for i, row := range d.Details {
		if row.Miss+row.CacheRead+row.CacheCreate+row.Output == 0 {
			continue
		}
		e := retainedEvent(source, d.Day, row.Miss, row.CacheRead, row.CacheCreate, row.Output)
		e.Model, e.Vendor = row.Model, row.Vendor
		row.Provenance = storedProvenance(row.Provenance)
		e.Provenance = row.Provenance
		e.Timestamp = time.Unix(0, row.TS).UTC()
		e.RequestID = fmt.Sprintf("retained:%s:%s:%s:%d", d.Day, row.Provenance, digestString(detailEncoding(row)), i)
		if row.Model == "" {
			e.RequestID += ":history-unallocated"
		}
		out = append(out, e)
	}
	return out
}

func sortDetails(details []observationDetail) {
	sort.Slice(details, func(i, j int) bool { return detailEncoding(details[i]) < detailEncoding(details[j]) })
}

// AttributionResult counts UTC days. RejectedDays includes days whose
// measured model attribution already has authority over a backfill.
type AttributionResult struct {
	AllocatedDays int
	RejectedDays  int
	MissingDays   int
}

func dayChecksum(d dayComp) string {
	return digestString(fmt.Sprintf("%s|%s|%d|%d|%d|%d", d.Hash, d.Day, d.Miss, d.CacheRead, d.CacheCreate, d.Output))
}

func hasMeasuredAttribution(d dayComp) bool {
	// Endpoint evidence describes the measurement, not whether it came from
	// an overlay. The table that supplied these rows establishes that role.
	if d.Overlay {
		return false
	}
	for _, row := range d.Details {
		if row.Model != "" {
			return true
		}
	}
	return false
}

// BackfillAttribution splits existing confirmed UTC-day totals into measured
// model rows and an explicitly unallocated residual. It never changes any
// observation's four components, creates a missing day, or grants a correction
// permit. The caller must first verify account identity and the completeness
// of the API windows; this method additionally enforces each day component's
// upper bound. Current measured attribution always takes precedence.
func (l *Ledger) BackfillAttribution(source string, events []event.UsageEvent) (AttributionResult, error) {
	var result AttributionResult
	if source == "" {
		return result, errors.New("ledger attribution rejected")
	}
	// Repeated API pages may repeat an identical stable request. An ID with
	// conflicting accounting fields makes the input ambiguous, so reject the
	// complete call before any attribution write. Unkeyed rows stay distinct.
	type requestPayload struct {
		detail    observationDetail
		reasoning int64
		quality   event.Quality
	}
	seen := map[string]requestPayload{}
	unique := make([]event.UsageEvent, 0, len(events))
	dayTotals := map[string][4]int64{}
	for _, e := range events {
		if e.Source != source || e.Derivation != event.DeriveProviderAPI || e.Timestamp.IsZero() || e.Miss < 0 || e.CacheRead < 0 || e.CacheCreate < 0 || e.Output < 0 {
			return result, errors.New("ledger attribution rejected")
		}
		if e.RequestID != "" {
			payload := requestPayload{detailFromEvent(e), e.Reasoning, e.Quality}
			if previous, ok := seen[e.RequestID]; ok {
				if previous != payload {
					return result, errors.New("ledger attribution rejected")
				}
				continue
			}
			seen[e.RequestID] = payload
		}
		unique = append(unique, e)
		day := e.Timestamp.UTC().Format("2006-01-02")
		total := dayTotals[day]
		for i, n := range [4]int64{e.Miss, e.CacheRead, e.CacheCreate, e.Output} {
			if n > math.MaxInt64-total[i] {
				return result, errors.New("ledger attribution rejected")
			}
			total[i] += n
		}
		dayTotals[day] = total
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	obs, err := l.observationsAsc(source, "")
	if err != nil {
		return result, err
	}
	base := chainDayMap(obs)
	if err := l.hydrateDayMap(source, base); err != nil {
		return result, err
	}
	tx, err := l.db.Begin()
	if err != nil {
		return result, err
	}
	defer tx.Rollback()
	for _, d := range groupDays(unique) {
		original, ok := base[d.Day]
		if !ok {
			result.MissingDays++
			continue
		}
		if hasMeasuredAttribution(original) || d.Miss > original.Miss || d.CacheRead > original.CacheRead || d.CacheCreate > original.CacheCreate || d.Output > original.Output {
			result.RejectedDays++
			continue
		}
		for i := range d.Details {
			if endpointProvenance(d.Details[i].Provenance) == "" {
				d.Details[i].Provenance = "attribution_backfill"
			}
		}
		residual := observationDetail{Provenance: "history_unallocated", Miss: original.Miss - d.Miss, CacheRead: original.CacheRead - d.CacheRead, CacheCreate: original.CacheCreate - d.CacheCreate, Output: original.Output - d.Output}
		residual.TS = retainedEvent(source, d.Day, 0, 0, 0, 0).Timestamp.UnixNano()
		if residual.Miss+residual.CacheRead+residual.CacheCreate+residual.Output > 0 {
			d.Details = append(d.Details, residual)
		}
		if _, err := tx.Exec(`INSERT INTO api_attribution_days(source,day,baseline_hash,baseline_checksum) VALUES(?,?,?,?)
		 ON CONFLICT(source,day) DO UPDATE SET baseline_hash=excluded.baseline_hash,baseline_checksum=excluded.baseline_checksum`, source, d.Day, original.Hash, dayChecksum(original)); err != nil {
			return AttributionResult{}, err
		}
		if _, err := tx.Exec(`DELETE FROM api_attribution_details WHERE source=? AND day=?`, source, d.Day); err != nil {
			return AttributionResult{}, err
		}
		for i, row := range d.Details {
			if _, err := tx.Exec(`INSERT INTO api_attribution_details
			 (source,day,ordinal,model,vendor,request_hash,ts,provenance,miss,cache_read,cache_create,output)
			 VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`, source, d.Day, i, row.Model, row.Vendor, row.RequestHash, row.TS, row.Provenance, row.Miss, row.CacheRead, row.CacheCreate, row.Output); err != nil {
				return AttributionResult{}, err
			}
		}
		result.AllocatedDays++
	}
	if err := tx.Commit(); err != nil {
		return AttributionResult{}, err
	}
	return result, nil
}

// hydrateDayMap reads only detail rows selected by the current chain, never
// every event in every historical observation. Batches keep the parameter
// count compatible with older SQLite builds. The day totals remain the
// authority, including when an older read-only ledger has no detail tables.
func (l *Ledger) hydrateDayMap(source string, days map[string]dayComp) error {
	keys := make([]string, 0, len(days))
	for day := range days {
		keys = append(keys, day)
	}
	sort.Strings(keys)
	for start := 0; start < len(keys); start += 300 {
		end := start + 300
		if end > len(keys) {
			end = len(keys)
		}
		batch := keys[start:end]
		placeholders := make([]string, 0, len(batch))
		args := make([]any, 0, len(batch)*2+1)
		for _, day := range batch {
			placeholders = append(placeholders, "(?,?)")
			args = append(args, days[day].Hash, day)
		}
		cte := "WITH selected(payload_hash,day) AS (VALUES " + strings.Join(placeholders, ",") + ") "
		args = append(args, source)
		// CROSS JOIN fixes the selected tuples as the outer loop, so SQLite
		// probes the composite primary key instead of scanning every historical
		// event for this source and comparing it to every selected day.
		details, _, err := l.selectedDetails(cte+`SELECT s.day,'',d.model,d.vendor,d.request_hash,d.ts,d.provenance,
		 d.miss,d.cache_read,d.cache_create,d.output FROM selected s
		 CROSS JOIN api_observation_details d ON d.payload_hash=s.payload_hash AND d.day=s.day
		 WHERE d.source=? ORDER BY s.day,d.ordinal`, args)
		if err != nil {
			return err
		}
		for _, day := range batch {
			d := days[day]
			d.Details = details[day]
			d.Overlay = false
			days[day] = d
		}
		overlays, checks, err := l.selectedDetails(cte+`SELECT a.day,a.baseline_checksum,d.model,d.vendor,d.request_hash,d.ts,d.provenance,
		 d.miss,d.cache_read,d.cache_create,d.output FROM selected s
		 CROSS JOIN api_attribution_days a ON a.baseline_hash=s.payload_hash AND a.day=s.day
		 CROSS JOIN api_attribution_details d ON a.source=d.source AND a.day=d.day
		 WHERE a.source=? ORDER BY a.day,d.ordinal`, args)
		if err != nil {
			return err
		}
		for _, day := range batch {
			d := days[day]
			if hasMeasuredAttribution(d) || checks[day] != dayChecksum(d) || !sameComponents(d, detailTotals(overlays[day])) {
				continue
			}
			d.Details = overlays[day]
			d.Overlay = true
			days[day] = d
		}
	}
	return nil
}

func (l *Ledger) selectedDetails(query string, args []any) (map[string][]observationDetail, map[string]string, error) {
	rows, err := l.db.Query(query, args...)
	// A read-only pre-attribution ledger must not need a schema migration.
	if err != nil && strings.Contains(err.Error(), "no such table") {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	details := map[string][]observationDetail{}
	checks := map[string]string{}
	for rows.Next() {
		var day, checksum string
		var row observationDetail
		if err := rows.Scan(&day, &checksum, &row.Model, &row.Vendor, &row.RequestHash, &row.TS, &row.Provenance, &row.Miss, &row.CacheRead, &row.CacheCreate, &row.Output); err != nil {
			return nil, nil, err
		}
		row.Provenance = storedProvenance(row.Provenance)
		details[day] = append(details[day], row)
		checks[day] = checksum
	}
	return details, checks, rows.Err()
}
