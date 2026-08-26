package adapter

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/rainhuang0220/whereToken/internal/event"
	"github.com/rainhuang0220/whereToken/internal/vendor"
)

// ParseOpenDB reads an OpenCode-shaped SQLite log (opencode, kilo):
// each message.data row is one JSON blob with role, model, and token
// counts. A row with a tokens block emits even when every counter is
// zero. Callers wrap it in index.LoadOrReplay for caching.
func ParseOpenDB(source, path string, root SourceRoot) ([]event.UsageEvent, []event.TurnEvent, int64, error) {
	var evs []event.UsageEvent
	var turns []event.TurnEvent
	err := scanOpenDB(source, path, root, func(e event.UsageEvent) {
		evs = append(evs, e)
	}, func(t event.TurnEvent) {
		turns = append(turns, t)
	})
	return evs, turns, 0, err
}

func scanOpenDB(source, path string, root SourceRoot, emit func(event.UsageEvent), emitTurn func(event.TurnEvent)) error {
	db, err := OpenRO(path)
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
		handleOpenDBRow(source, raw, sessionID, path, i, root, emit, emitTurn)
	}
	return rows.Err()
}

type openDBMsg struct {
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

func handleOpenDBRow(source, raw, sessionID, path string, seq int, root SourceRoot, emit func(event.UsageEvent), emitTurn func(event.TurnEvent)) {
	var m openDBMsg
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return
	}
	var ts time.Time
	if m.Time.Created > 0 {
		ts = time.UnixMilli(m.Time.Created).UTC()
	}
	if m.Role == "user" {
		emitTurn(event.TurnEvent{Source: source, SessionID: sessionID, Timestamp: ts})
	}
	if m.Tokens == nil {
		return
	}
	out := m.Tokens.Output + m.Tokens.Reasoning
	emit(event.UsageEvent{
		Source:      source,
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
		Derivation:  event.DeriveDerived,
	})
}
