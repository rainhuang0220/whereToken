// Package zcode reads ZCode (Z.ai's agentic IDE) request-level usage from
// the v2 SQLite store at ~/.zcode/cli/db/db.sqlite (model_usage rows).
//
// ZCode reports input_tokens inclusive of cache read + cache creation and
// output_tokens inclusive of reasoning, so both overlaps are subtracted;
// a computed_total_tokens column (newer schema) disambiguates rows that are
// already cache-exclusive. Legacy ~/.zcode/projects/*.jsonl transcripts are
// not read: lines without usage blocks would need token estimates, and
// whereToken does not estimate. Account sign-in and settings are never read.
package zcode

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"github.com/rainhuang0220/whereToken/internal/adapter"
	"github.com/rainhuang0220/whereToken/internal/event"
	"github.com/rainhuang0220/whereToken/internal/index"
	"github.com/rainhuang0220/whereToken/internal/vendor"
)

type Adapter struct{}

func (Adapter) ID() string { return "zcode" }

func (Adapter) Discover(home adapter.Home) []adapter.SourceRoot {
	if env := strings.TrimSpace(os.Getenv("ZCODE_DB")); env != "" {
		if st, err := os.Stat(env); err == nil && !st.IsDir() {
			return []adapter.SourceRoot{{ID: "zcode", Path: env}}
		}
	}
	dir := home.DotDir("zcode")
	if _, ok := dbFile(dir); ok {
		return []adapter.SourceRoot{{ID: "zcode", Path: dir}}
	}
	return nil
}

func (Adapter) Parse(root adapter.SourceRoot, emit func(event.UsageEvent), emitTurn func(event.TurnEvent)) error {
	path, ok := dbFile(root.Path)
	if !ok {
		return nil
	}
	evs, turns, _, err := index.LoadOrReplay("zcode", path, func(f *os.File) ([]event.UsageEvent, []event.TurnEvent, int64, error) {
		return parseDB(f.Name(), root)
	})
	return index.Forward(evs, turns, err, emit, emitTurn)
}

// dbFile accepts the ~/.zcode directory or the db file itself (ZCODE_DB).
func dbFile(root string) (string, bool) {
	if st, err := os.Stat(root); err == nil && !st.IsDir() {
		return root, true
	}
	p := filepath.Join(root, "cli", "db", "db.sqlite")
	if st, err := os.Stat(p); err == nil && !st.IsDir() {
		return p, true
	}
	return "", false
}

func parseDB(path string, root adapter.SourceRoot) ([]event.UsageEvent, []event.TurnEvent, int64, error) {
	db, err := adapter.OpenRO(path)
	if err != nil {
		return nil, nil, 0, err
	}
	defer db.Close()
	if !adapter.HasTable(db, "model_usage") {
		return nil, nil, 0, nil
	}
	// Older stores lack computed_total_tokens and their input/output are
	// always cache/reasoning-inclusive, so the overlap is subtracted
	// unconditionally there.
	legacy := !adapter.HasColumn(db, "model_usage", "computed_total_tokens")
	evs, err := emitUsage(db, root, legacy)
	if err != nil {
		return nil, nil, 0, err
	}
	turns, err := readTurns(db)
	return evs, turns, 0, err
}

func emitUsage(db *sql.DB, root adapter.SourceRoot, legacy bool) ([]event.UsageEvent, error) {
	totalCol := "mu.computed_total_tokens"
	if legacy {
		totalCol = "NULL"
	}
	// Session joins give the workspace label. Older or partial stores may
	// lack the session table or its directory/path columns; workspace then
	// degrades to unlabeled instead of failing the whole source.
	workspaceCol := "''"
	join := ""
	if adapter.HasTable(db, "session") && adapter.HasColumn(db, "session", "id") {
		hasDir := adapter.HasColumn(db, "session", "directory")
		hasPath := adapter.HasColumn(db, "session", "path")
		switch {
		case hasDir && hasPath:
			workspaceCol = "COALESCE(NULLIF(s.directory, ''), NULLIF(s.path, ''), '')"
		case hasDir:
			workspaceCol = "COALESCE(NULLIF(s.directory, ''), '')"
		case hasPath:
			workspaceCol = "COALESCE(NULLIF(s.path, ''), '')"
		}
		if hasDir || hasPath {
			join = " LEFT JOIN session s ON s.id = mu.session_id"
		}
	}
	rows, err := db.Query(`
		SELECT CAST(mu.id AS TEXT),
		       COALESCE(NULLIF(mu.session_id, ''), ''),
		       COALESCE(NULLIF(mu.model_id, ''), ''),
		       mu.started_at, mu.completed_at, mu.duration_ms,
		       mu.input_tokens, mu.output_tokens, mu.reasoning_tokens,
		       mu.cache_read_input_tokens, mu.cache_creation_input_tokens,
		       ` + totalCol + `, ` + workspaceCol + `
		FROM model_usage mu` + join + `
		WHERE COALESCE(mu.input_tokens, 0)
		    + COALESCE(mu.output_tokens, 0)
		    + COALESCE(mu.reasoning_tokens, 0)
		    + COALESCE(mu.cache_read_input_tokens, 0)
		    + COALESCE(mu.cache_creation_input_tokens, 0) > 0
		ORDER BY COALESCE(mu.completed_at, mu.started_at, 0), mu.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var evs []event.UsageEvent
	for rows.Next() {
		var (
			id                             sql.NullString
			sessionID, model, workspace    string
			started, completed, durationMS sql.NullInt64
			in, out, reason, cr, cw        sql.NullInt64
			total                          sql.NullInt64
		)
		if err := rows.Scan(&id, &sessionID, &model, &started, &completed, &durationMS,
			&in, &out, &reason, &cr, &cw, &total, &workspace); err != nil {
			return evs, err
		}
		miss, output := normalizeBuckets(in.Int64, out.Int64, reason.Int64, cr.Int64, cw.Int64, total, legacy)
		cacheRead := adapter.Clamp0(cr.Int64)
		cacheCreate := adapter.Clamp0(cw.Int64)
		reasoning := adapter.Clamp0(reason.Int64)
		// A NULL id must not fail the source; keep a distinct deterministic
		// RequestID so merge-by-request cannot collapse these rows together.
		requestID := "zcode:" + id.String
		if !id.Valid {
			requestID = fmt.Sprintf("zcode:null:%d", len(evs))
		}
		evs = append(evs, event.UsageEvent{
			Source:      "zcode",
			Vendor:      vendor.Lookup(model, ""),
			SourceRoot:  root.Path,
			RequestID:   requestID,
			SessionID:   sessionID,
			Model:       model,
			Workspace:   workspace,
			Timestamp:   rowTime(started, completed, durationMS),
			Miss:        miss,
			CacheRead:   cacheRead,
			CacheCreate: cacheCreate,
			Output:      output,
			Reasoning:   reasoning,
			Quality:     event.QualityAuthoritative,
			Derivation:  event.DeriveDerived,
		})
	}
	return evs, rows.Err()
}

// normalizeBuckets splits ZCode's inclusive counters into non-overlapping
// buckets: input_tokens absorbs cache read + creation, output_tokens absorbs
// reasoning. computed_total_tokens (modern schema) disambiguates: when the
// reported total equals input+output the row is inclusive and the overlap
// comes out; when it equals the fully additive sum the row is already
// exclusive. Legacy rows are always inclusive.
func normalizeBuckets(in, out, reasoning, cr, cw int64, total sql.NullInt64, legacy bool) (miss, output int64) {
	in = adapter.Clamp0(in)
	out = adapter.Clamp0(out)
	reasoning = adapter.Clamp0(reasoning)
	overlap := adapter.Clamp0(cr) + adapter.Clamp0(cw)
	if legacy {
		return subFloor(in, overlap), subFloor(out, reasoning)
	}
	if total.Valid {
		t := adapter.Clamp0(total.Int64)
		inclusive := in + out
		if (overlap > 0 || reasoning > 0) && t == inclusive && t != inclusive+overlap+reasoning {
			return subFloor(in, overlap), subFloor(out, reasoning)
		}
	}
	return in, out
}

func subFloor(v, minus int64) int64 {
	if minus > v {
		return 0
	}
	return v - minus
}

// rowTime prefers started_at (unix ms); a bare completed_at is anchored back
// by duration_ms. Undated rows stay zero rather than inventing a time.
func rowTime(started, completed, durationMS sql.NullInt64) time.Time {
	if started.Valid && started.Int64 > 0 {
		return time.UnixMilli(started.Int64).UTC()
	}
	if completed.Valid && completed.Int64 > 0 {
		if durationMS.Valid && durationMS.Int64 > 0 && durationMS.Int64 < completed.Int64 {
			return time.UnixMilli(completed.Int64 - durationMS.Int64).UTC()
		}
		return time.UnixMilli(completed.Int64).UTC()
	}
	return time.Time{}
}

// readTurns counts one user turn per distinct turn_id. Stores without a
// turn_id column still report usage; turns are simply unavailable there.
func readTurns(db *sql.DB) ([]event.TurnEvent, error) {
	if !adapter.HasColumn(db, "model_usage", "turn_id") {
		return nil, nil
	}
	rows, err := db.Query(`
		SELECT COALESCE(NULLIF(session_id, ''), ''), turn_id, MIN(started_at)
		FROM model_usage
		WHERE turn_id IS NOT NULL AND turn_id <> ''
		GROUP BY session_id, turn_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var turns []event.TurnEvent
	for rows.Next() {
		var sessionID, turnID string
		var ts sql.NullInt64
		if err := rows.Scan(&sessionID, &turnID, &ts); err != nil {
			return turns, err
		}
		var stamp time.Time
		if ts.Valid && ts.Int64 > 0 {
			stamp = time.UnixMilli(ts.Int64).UTC()
		}
		turns = append(turns, event.TurnEvent{Source: "zcode", SessionID: sessionID, Timestamp: stamp})
	}
	return turns, rows.Err()
}
