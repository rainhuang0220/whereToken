package cursor

import (
	"database/sql"
	"encoding/json"
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
	"github.com/rainhuang0220/whereToken/internal/index"
	"github.com/rainhuang0220/whereToken/internal/vendor"
)

const thinkingCapability = 30

type Adapter struct {
	HTTP    *http.Client
	APIBase string
	Now     func() time.Time
	Offline bool
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
	localEvs, localTurns, _, err := index.LoadOrReplay("cursor", path, func(f *os.File) ([]event.UsageEvent, []event.TurnEvent, int64, error) {
		return parseLocalDB(f.Name(), root)
	})
	if err != nil {
		return index.Forward(localEvs, localTurns, err, emit, emitTurn)
	}

	token, refresh, err := authTokens(path, !a.Offline)
	if err != nil {
		return err
	}

	var apiEvents []event.UsageEvent
	var apiErr error
	if token != "" && !a.Offline {
		apiEvents, apiErr = a.fetchAccountUsage(root.Path, token, refresh)
	}

	useAPI := hasTokenTotals(apiEvents)
	for _, e := range localEvs {
		if useAPI {
			e = stripLocalTokens(e)
		}
		emit(e)
	}
	for _, t := range localTurns {
		emitTurn(t)
	}
	if useAPI {
		for _, e := range apiEvents {
			emit(e)
		}
	}
	if token == "" {
		return errNoLocalAuth
	}
	if a.Offline {
		return nil
	}
	return apiErr
}

// parseLocalDB is the sqlite half of the source, and the unit LoadOrReplay
// caches: composer models, workspaces, and bubble tokens. Auth rows and the
// account API stay outside on purpose — credentials never enter the index
// cache, and the API stays authoritative for tokens on every scan.
func parseLocalDB(path string, root adapter.SourceRoot) ([]event.UsageEvent, []event.TurnEvent, int64, error) {
	db, err := adapter.OpenRO(path)
	if err != nil {
		return nil, nil, 0, err
	}
	defer db.Close()

	composers, err := loadComposers(db)
	if err != nil {
		return nil, nil, 0, err
	}
	if err := loadHeaders(db, composers); err != nil {
		return nil, nil, 0, err
	}
	bubbles, err := loadBubbles(db)
	if err != nil {
		return nil, nil, 0, err
	}
	var evs []event.UsageEvent
	var turns []event.TurnEvent
	emitLocal(composers, bubbles, root, func(e event.UsageEvent) {
		evs = append(evs, e)
	}, func(t event.TurnEvent) {
		turns = append(turns, t)
	})
	return evs, turns, 0, nil
}

// authTokens reads the keyed Cursor auth rows. needRefresh gates the refresh
// row so offline scans never touch it.
func authTokens(path string, needRefresh bool) (access, refresh string, err error) {
	db, err := adapter.OpenRO(path)
	if err != nil {
		return "", "", err
	}
	defer db.Close()
	access, err = readItem(db, authAccessTokenKey)
	if err != nil {
		return "", "", err
	}
	if access == "" {
		access = readStorageJSONToken(path)
	}
	if access != "" && needRefresh {
		if refresh, err = readItem(db, authRefreshTokenKey); err != nil {
			return "", "", err
		}
	}
	return access, refresh, nil
}

// stripLocalTokens zeroes local bubble tokens once the API supplied account
// totals: the event stays for request/turn counting only. Same shape the old
// strip-at-parse path produced.
func stripLocalTokens(e event.UsageEvent) event.UsageEvent {
	e.Miss, e.CacheRead, e.CacheCreate, e.Output = 0, 0, 0, 0
	e.Quality = ""
	e.Derivation = ""
	return e
}

func localDerivation(q event.Quality) string {
	if q == "" {
		return ""
	}
	return event.DeriveRaw
}

func emitLocal(composers map[string]*composerMeta, bubbles []bubbleRow, root adapter.SourceRoot, emit func(event.UsageEvent), emitTurn func(event.TurnEvent)) {
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
			if miss != 0 || cacheRead != 0 || cacheCreate != 0 || out != 0 {
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
				Derivation:  localDerivation(q),
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

// loadBubbles reads usage-bearing bubbles. Each blob is parsed once into a
// json_extract array instead of one extract per field, and the type filter
// runs SQL-side so non-usage bubbles never get decoded or cross the wire.
// The capability (thinking) filter stays in emitLocal; the array carries it.
func loadBubbles(db *sql.DB) ([]bubbleRow, error) {
	rows, err := db.Query(`
SELECT key,
  json_extract(value, '$.type', '$.createdAt',
    '$.tokenCount.inputTokens', '$.tokenCount.outputTokens',
    '$.tokenCount.cacheReadTokens', '$.tokenCount.cacheWriteTokens',
    '$.modelInfo.modelName', '$.capabilityType')
FROM cursorDiskKV
WHERE key LIKE 'bubbleId:%'
  AND CAST(json_extract(value, '$.type') AS INTEGER) IN (1, 2)`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []bubbleRow
	for rows.Next() {
		var key string
		var fields sql.NullString
		if err := rows.Scan(&key, &fields); err != nil {
			return nil, err
		}
		composerID, bubbleID, ok := splitBubbleKey(key)
		if !ok || !fields.Valid {
			continue
		}
		b, ok := decodeBubbleFields(fields.String)
		if !ok {
			continue
		}
		b.composerID = composerID
		b.bubbleID = bubbleID
		out = append(out, b)
	}
	return out, rows.Err()
}

// decodeBubbleFields unpacks the json_extract array loadBubbles selects, in
// path order: type, createdAt, input, output, cacheRead, cacheWrite, model,
// capability. It keeps the old per-column tolerance: numbers, numeric
// strings, and nulls all decode the same way.
func decodeBubbleFields(raw string) (bubbleRow, bool) {
	var arr [8]any
	dec := json.NewDecoder(strings.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&arr); err != nil {
		return bubbleRow{}, false
	}
	return bubbleRow{
		model:       adapter.FlexString(arr[6]),
		typ:         int(adapter.FlexInt(arr[0])),
		capability:  int(adapter.FlexInt(arr[7])),
		ts:          parseTime(adapter.FlexString(arr[1])),
		miss:        adapter.FlexInt(arr[2]),
		cacheRead:   adapter.FlexInt(arr[4]),
		cacheCreate: adapter.FlexInt(arr[5]),
		output:      adapter.FlexInt(arr[3]),
	}, true
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
