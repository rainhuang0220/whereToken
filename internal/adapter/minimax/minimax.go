package minimax

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"time"

	_ "modernc.org/sqlite"

	"github.com/rainhuang0220/whereToken/internal/adapter"
	"github.com/rainhuang0220/whereToken/internal/event"
	"github.com/rainhuang0220/whereToken/internal/index"
	"github.com/rainhuang0220/whereToken/internal/vendor"
)

// Adapter reads MiniMax Agent's local token ledger.
//
// Only local_runtime_token_usage (and user-turn rows) are queried.
// Login files, session names, and message bodies are ignored.
type Adapter struct{}

func (Adapter) ID() string { return "minimax" }

func (Adapter) Discover(home adapter.Home) []adapter.SourceRoot {
	dir := home.DotDir("minimax")
	st, err := os.Stat(dir)
	if err != nil || !st.IsDir() {
		return nil
	}
	return []adapter.SourceRoot{{ID: "minimax", Path: dir}}
}

func (Adapter) Parse(root adapter.SourceRoot, emit func(event.UsageEvent), emitTurn func(event.TurnEvent)) error {
	path, ok := dbFile(root.Path)
	if !ok {
		return nil
	}
	return parseDB(path, root, emit, emitTurn)
}

func dbFile(path string) (string, bool) {
	if st, err := os.Stat(path); err == nil && !st.IsDir() {
		return path, true
	}
	p := filepath.Join(path, "v2", "sqlite", "runtime-state.sqlite")
	if st, err := os.Stat(p); err == nil && !st.IsDir() {
		return p, true
	}
	return "", false
}

func parseDB(path string, root adapter.SourceRoot, emit func(event.UsageEvent), emitTurn func(event.TurnEvent)) error {
	evs, turns, _, err := index.LoadOrReplay("minimax", path, func(f *os.File) ([]event.UsageEvent, []event.TurnEvent, int64, error) {
		return parseDBPath(f.Name(), root)
	})
	if err != nil {
		return err
	}
	for _, e := range evs {
		emit(e)
	}
	for _, t := range turns {
		emitTurn(t)
	}
	return nil
}

func parseDBPath(path string, root adapter.SourceRoot) ([]event.UsageEvent, []event.TurnEvent, int64, error) {
	var evs []event.UsageEvent
	var turns []event.TurnEvent
	err := parseDBOpen(path, root, func(e event.UsageEvent) {
		evs = append(evs, e)
	}, func(t event.TurnEvent) {
		turns = append(turns, t)
	})
	return evs, turns, 0, err
}

func parseDBOpen(path string, root adapter.SourceRoot, emit func(event.UsageEvent), emitTurn func(event.TurnEvent)) error {
	db, err := adapter.OpenRO(path)
	if err != nil {
		return err
	}
	defer db.Close()

	if !hasTable(db, "local_runtime_token_usage") {
		return nil
	}
	ws := loadWorkspaces(db)
	if err := emitUsage(db, root, ws, emit); err != nil {
		return err
	}
	return emitTurns(db, emitTurn)
}

func hasTable(db *sql.DB, name string) bool {
	var n int
	err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?`, name).Scan(&n)
	return err == nil && n > 0
}

type sessionMeta struct {
	WorkspaceDir string `json:"workspaceDir"`
}

func loadWorkspaces(db *sql.DB) map[string]string {
	out := map[string]string{}
	if !hasTable(db, "local_runtime_sessions") {
		return out
	}
	rows, err := db.Query(`SELECT session_id, record_json FROM local_runtime_sessions`)
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var id, raw string
		if err := rows.Scan(&id, &raw); err != nil {
			continue
		}
		var meta sessionMeta
		if json.Unmarshal([]byte(raw), &meta) != nil {
			continue
		}
		if meta.WorkspaceDir != "" {
			out[id] = meta.WorkspaceDir
		}
	}
	return out
}

func emitUsage(db *sql.DB, root adapter.SourceRoot, ws map[string]string, emit func(event.UsageEvent)) error {
	rows, err := db.Query(`
		SELECT id, session_id, model, ts,
		       input_tokens, output_tokens, reasoning_tokens,
		       cache_read_tokens, cache_write_tokens
		FROM local_runtime_token_usage`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var (
			id                            int64
			sessionID, model              sql.NullString
			ts, miss, out, reason, cr, cw int64
		)
		if err := rows.Scan(&id, &sessionID, &model, &ts, &miss, &out, &reason, &cr, &cw); err != nil {
			return err
		}
		miss = clamp0(miss)
		out = clamp0(out)
		reason = clamp0(reason)
		cr = clamp0(cr)
		cw = clamp0(cw)
		if miss+out+reason+cr+cw == 0 {
			continue
		}
		var stamp time.Time
		if ts > 0 {
			stamp = time.UnixMilli(ts).UTC()
		}
		emit(event.UsageEvent{
			Source:      "minimax",
			Vendor:      vendor.Lookup(model.String, ""),
			SourceRoot:  root.Path,
			RequestID:   strconv.FormatInt(id, 10),
			SessionID:   sessionID.String,
			Model:       model.String,
			Workspace:   ws[sessionID.String],
			Timestamp:   stamp,
			Miss:        miss,
			CacheRead:   cr,
			CacheCreate: cw,
			Output:      out,
			Reasoning:   reason,
			Quality:     event.QualityAuthoritative,
			Derivation:  event.DeriveRaw,
		})
	}
	return rows.Err()
}

func emitTurns(db *sql.DB, emitTurn func(event.TurnEvent)) error {
	if !hasTable(db, "local_runtime_message_rows") {
		return nil
	}
	rows, err := db.Query(`SELECT session_id, created_at_ms FROM local_runtime_message_rows WHERE role = 'user'`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var sessionID string
		var ts int64
		if err := rows.Scan(&sessionID, &ts); err != nil {
			return err
		}
		var stamp time.Time
		if ts > 0 {
			stamp = time.UnixMilli(ts).UTC()
		}
		emitTurn(event.TurnEvent{Source: "minimax", SessionID: sessionID, Timestamp: stamp})
	}
	return rows.Err()
}

func clamp0(n int64) int64 {
	if n < 0 {
		return 0
	}
	return n
}
