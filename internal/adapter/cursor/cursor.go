package cursor

import (
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"github.com/rainhuang0220/whereToken/internal/adapter"
	"github.com/rainhuang0220/whereToken/internal/event"
	"github.com/rainhuang0220/whereToken/internal/vendor"
)

const thinkingCapability = 30

type Adapter struct {
	HTTP    *http.Client
	APIBase string
	Now     func() time.Time
}

func (Adapter) ID() string { return "cursor" }

func (Adapter) Discover(home adapter.Home) []adapter.SourceRoot {
	if p := adapter.VSCodeGlobalDB(home, "Cursor"); p != "" {
		return []adapter.SourceRoot{{ID: "cursor", Path: p}}
	}
	p := home.DotDir("cursor")
	if st, err := os.Stat(p); err == nil && st.IsDir() {
		return []adapter.SourceRoot{{ID: "cursor", Path: p}}
	}
	return nil
}

func (a Adapter) Parse(root adapter.SourceRoot, emit func(event.UsageEvent), emitTurn func(event.TurnEvent)) error {
	path := resolveDB(root.Path)
	if path == "" {
		return nil
	}
	return a.parseDB(path, root, emit, emitTurn)
}

func resolveDB(p string) string {
	st, err := os.Stat(p)
	if err != nil {
		return ""
	}
	if !st.IsDir() {
		return p
	}
	for _, rel := range []string{
		filepath.Join("User", "globalStorage", "state.vscdb"),
		"state.vscdb",
	} {
		cand := filepath.Join(p, rel)
		if st, err := os.Stat(cand); err == nil && !st.IsDir() {
			return cand
		}
	}
	return ""
}

func (a Adapter) parseDB(path string, root adapter.SourceRoot, emit func(event.UsageEvent), emitTurn func(event.TurnEvent)) error {
	db, err := openRO(path)
	if err != nil {
		return err
	}
	defer db.Close()

	composers, err := loadComposers(db)
	if err != nil {
		return err
	}
	if err := loadHeaders(db, composers); err != nil {
		return err
	}
	bubbles, err := loadBubbles(db)
	if err != nil {
		return err
	}
	token, err := readItem(db, authAccessTokenKey)
	if err != nil {
		return err
	}
	if token == "" {
		token = readStorageJSONToken(path)
	}

	var apiEvents []event.UsageEvent
	var apiErr error
	if token != "" {
		refresh, rerr := readItem(db, authRefreshTokenKey)
		if rerr != nil {
			return rerr
		}
		apiEvents, apiErr = a.fetchAccountUsage(root.Path, token, refresh)
	}

	useAPI := apiErr == nil && hasTokenTotals(apiEvents)
	emitLocal(composers, bubbles, root, emit, emitTurn, useAPI)
	if useAPI {
		for _, e := range apiEvents {
			emit(e)
		}
	}
	if token == "" {
		return errNoLocalAuth
	}
	return apiErr
}

func emitLocal(composers map[string]*composerMeta, bubbles []bubbleRow, root adapter.SourceRoot, emit func(event.UsageEvent), emitTurn func(event.TurnEvent), stripTokens bool) {
	bySess := map[string][]bubbleRow{}
	for _, b := range bubbles {
		bySess[b.composerID] = append(bySess[b.composerID], b)
	}
	for cid, list := range bySess {
		sort.Slice(list, func(i, j int) bool {
			if list[i].ts.Equal(list[j].ts) {
				return list[i].bubbleID < list[j].bubbleID
			}
			return list[i].ts.Before(list[j].ts)
		})
		meta := composers[cid]
		model := ""
		if meta != nil {
			model = meta.model
		}
		for _, b := range list {
			if b.typ == 1 {
				if b.model != "" {
					model = b.model
				}
				if meta == nil || !meta.subagent {
					emitTurn(event.TurnEvent{
						Source:    "cursor",
						SessionID: cid,
						Workspace: workspaceOf(meta),
						Timestamp: b.ts,
					})
				}
				continue
			}
			if b.typ != 2 || b.capability == thinkingCapability {
				continue
			}
			useModel := model
			if useModel == "" {
				useModel = b.model
			}
			miss, cacheRead, cacheCreate, out := b.miss, b.cacheRead, b.cacheCreate, b.output
			q := event.QualityDegraded
			if stripTokens {
				miss, cacheRead, cacheCreate, out = 0, 0, 0, 0
				q = ""
			} else if miss != 0 || cacheRead != 0 || cacheCreate != 0 || out != 0 {
				q = event.QualityAuthoritative
			}
			emit(event.UsageEvent{
				Source:      "cursor",
				Vendor:      vendor.Lookup(useModel, ""),
				SourceRoot:  root.Path,
				SessionID:   cid,
				RequestID:   cid + ":" + b.bubbleID,
				Model:       useModel,
				Workspace:   workspaceOf(meta),
				Timestamp:   b.ts,
				Miss:        miss,
				CacheRead:   cacheRead,
				CacheCreate: cacheCreate,
				Output:      out,
				Quality:     q,
			})
		}
	}
}

type composerMeta struct {
	model     string
	workspace string
	subagent  bool
}

type bubbleRow struct {
	composerID, bubbleID, model          string
	typ, capability                      int
	ts                                   time.Time
	miss, cacheRead, cacheCreate, output int64
}

func workspaceOf(m *composerMeta) string {
	if m == nil {
		return ""
	}
	return m.workspace
}

func loadComposers(db *sql.DB) (map[string]*composerMeta, error) {
	rows, err := db.Query(`
SELECT key,
  json_extract(value, '$.composerId'),
  json_extract(value, '$.modelConfig.modelName')
FROM cursorDiskKV
WHERE key LIKE 'composerData:%'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]*composerMeta{}
	for rows.Next() {
		var key string
		var id, model sql.NullString
		if err := rows.Scan(&key, &id, &model); err != nil {
			return nil, err
		}
		cid := id.String
		if cid == "" {
			cid = strings.TrimPrefix(key, "composerData:")
		}
		out[cid] = &composerMeta{model: model.String}
	}
	return out, rows.Err()
}

func loadHeaders(db *sql.DB, composers map[string]*composerMeta) error {
	var name string
	err := db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name='composerHeaders'`).Scan(&name)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return err
	}
	rows, err := db.Query(`
SELECT composerId, isSubagent, json_extract(value, '$.workspaceIdentifier.uri.fsPath')
FROM composerHeaders`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var cid string
		var sub sql.NullInt64
		var ws sql.NullString
		if err := rows.Scan(&cid, &sub, &ws); err != nil {
			return err
		}
		meta := composers[cid]
		if meta == nil {
			meta = &composerMeta{}
			composers[cid] = meta
		}
		meta.subagent = sub.Valid && sub.Int64 != 0
		if ws.String != "" {
			meta.workspace = ws.String
		}
	}
	return rows.Err()
}

func loadBubbles(db *sql.DB) ([]bubbleRow, error) {
	rows, err := db.Query(`
SELECT key,
  json_extract(value, '$.type'),
  json_extract(value, '$.createdAt'),
  json_extract(value, '$.tokenCount.inputTokens'),
  json_extract(value, '$.tokenCount.outputTokens'),
  json_extract(value, '$.tokenCount.cacheReadTokens'),
  json_extract(value, '$.tokenCount.cacheWriteTokens'),
  json_extract(value, '$.modelInfo.modelName'),
  json_extract(value, '$.capabilityType')
FROM cursorDiskKV
WHERE key LIKE 'bubbleId:%'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []bubbleRow
	for rows.Next() {
		var key string
		var typ, created, inn, output, cr, cw, model, cap sql.NullString
		if err := rows.Scan(&key, &typ, &created, &inn, &output, &cr, &cw, &model, &cap); err != nil {
			return nil, err
		}
		composerID, bubbleID, ok := splitBubbleKey(key)
		if !ok {
			continue
		}
		out = append(out, bubbleRow{
			composerID:  composerID,
			bubbleID:    bubbleID,
			model:       model.String,
			typ:         atoi(typ.String),
			capability:  atoi(cap.String),
			ts:          parseTime(created.String),
			miss:        atoi64(inn.String),
			cacheRead:   atoi64(cr.String),
			cacheCreate: atoi64(cw.String),
			output:      atoi64(output.String),
		})
	}
	return out, rows.Err()
}

func splitBubbleKey(key string) (composerID, bubbleID string, ok bool) {
	rest, found := strings.CutPrefix(key, "bubbleId:")
	if !found {
		return "", "", false
	}
	composerID, bubbleID, found = strings.Cut(rest, ":")
	if !found || composerID == "" || bubbleID == "" {
		return "", "", false
	}
	return composerID, bubbleID, true
}

func parseTime(s string) time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}
	}
	if ts, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return ts
	}
	if ts, err := time.Parse(time.RFC3339, s); err == nil {
		return ts
	}
	if n, err := strconv.ParseInt(s, 10, 64); err == nil {
		if n > 1e12 {
			return time.UnixMilli(n)
		}
		if n > 1e9 {
			return time.Unix(n, 0)
		}
	}
	return time.Time{}
}

func atoi(s string) int {
	return int(atoi64(s))
}

func atoi64(s string) int64 {
	s = strings.TrimSpace(s)
	if s == "" || s == "null" {
		return 0
	}
	if n, err := strconv.ParseInt(s, 10, 64); err == nil {
		return n
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return int64(f)
	}
	return 0
}

func openRO(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", fmt.Sprintf("file:%s?mode=ro", path))
	if err == nil {
		if pingErr := db.Ping(); pingErr == nil {
			return db, nil
		}
		db.Close()
	}
	db, err = sql.Open("sqlite", fmt.Sprintf("file:%s?mode=ro&immutable=1", path))
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}
