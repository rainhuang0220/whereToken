package opencode

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"

	"github.com/rainhuang0220/whereToken/internal/adapter"
	"github.com/rainhuang0220/whereToken/internal/event"
	"github.com/rainhuang0220/whereToken/internal/vendor"
)

type Adapter struct{}

func (Adapter) ID() string { return "opencode" }

func (Adapter) Discover(home adapter.Home) []adapter.SourceRoot {
	dir := home.XDGData("opencode")
	if _, ok := dbFile(dir); ok {
		return []adapter.SourceRoot{{ID: "opencode", Path: dir}}
	}
	return nil
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
	for _, name := range []string{"opencode.db", "opencode-stable.db"} {
		p := filepath.Join(path, name)
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p, true
		}
	}
	return "", false
}

func parseDB(path string, root adapter.SourceRoot, emit func(event.UsageEvent), emitTurn func(event.TurnEvent)) error {
	db, err := openRO(path)
	if err != nil {
		return err
	}
	defer db.Close()

	rows, err := db.Query(`SELECT session_id, data FROM message`)
	if err != nil {
		return err
	}
	defer rows.Close()
	i := 0
	for rows.Next() {
		var sessionID, raw string
		if err := rows.Scan(&sessionID, &raw); err != nil {
			return err
		}
		i++
		handleRow(raw, sessionID, path, i, root, emit, emitTurn)
	}
	return rows.Err()
}

func openRO(path string) (*sql.DB, error) {
	return adapter.OpenRO(path)
}

type msgData struct {
	Role       string `json:"role"`
	ModelID    string `json:"modelID"`
	ProviderID string `json:"providerID"`
	Time       struct {
		Created int64 `json:"created"`
	} `json:"time"`
	Tokens *struct {
		Input     int64 `json:"input"`
		Output    int64 `json:"output"`
		Reasoning int64 `json:"reasoning"`
		Cache     struct {
			Read  int64 `json:"read"`
			Write int64 `json:"write"`
		} `json:"cache"`
	} `json:"tokens"`
}

func handleRow(raw, sessionID, path string, seq int, root adapter.SourceRoot, emit func(event.UsageEvent), emitTurn func(event.TurnEvent)) {
	var m msgData
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return
	}
	ts := time.UnixMilli(m.Time.Created).UTC()
	if m.Role == "user" {
		emitTurn(event.TurnEvent{Source: "opencode", SessionID: sessionID, Timestamp: ts})
	}
	if m.Tokens == nil {
		return
	}
	out := m.Tokens.Output + m.Tokens.Reasoning
	emit(event.UsageEvent{
		Source:      "opencode",
		Vendor:      vendor.Lookup(m.ModelID, m.ProviderID),
		SourceRoot:  root.Path,
		RequestID:   fmt.Sprintf("%s:%d", path, seq),
		SessionID:   sessionID,
		Model:       m.ModelID,
		Provider:    m.ProviderID,
		Timestamp:   ts,
		Miss:        m.Tokens.Input,
		CacheRead:   m.Tokens.Cache.Read,
		CacheCreate: m.Tokens.Cache.Write,
		Output:      out,
		Reasoning:   m.Tokens.Reasoning,
		Quality:     event.QualityAuthoritative,
	})
}
