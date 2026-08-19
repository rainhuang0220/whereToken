package trae

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"github.com/rainhuang0220/whereToken/internal/adapter"
	"github.com/rainhuang0220/whereToken/internal/event"
)

type Adapter struct {
	HTTP        *http.Client
	APIBase     string
	Now         func() time.Time
	Offline     bool
	FetchBudget time.Duration
}

func (Adapter) ID() string { return "trae" }

var traeProducts = []string{
	"Trae", "Trae CN", "Trae-CN", "TraeCN",
	"TRAE SOLO", "TRAE SOLO CN", "TRAE SOLO-CN",
	"Trae SOLO", "Trae SOLO CN",
}

var errNoLocalAuth = errors.New("未找到本机登录态")
var errEncryptedLocalAuth = errors.New("登录态在加密存储中，没有可读的 JWT 文件")

func (Adapter) Discover(home adapter.Home) []adapter.SourceRoot {
	jwt := firstJWT(home)
	var out []adapter.SourceRoot
	var seenDB, seenParent []os.FileInfo
	for _, product := range traeProducts {
		p := adapter.VSCodeGlobalDB(home, product)
		if p == "" {
			continue
		}
		info, err := os.Stat(p)
		if err != nil {
			continue
		}
		if fileSeen(seenDB, info) {
			continue
		}
		parent := productParent(p)
		if pInfo, err := os.Stat(parent); err == nil && fileSeen(seenParent, pInfo) {
			continue
		}
		seenDB = append(seenDB, info)
		if pInfo, err := os.Stat(parent); err == nil {
			seenParent = append(seenParent, pInfo)
		}
		out = append(out, adapter.SourceRoot{ID: "trae", Path: p, AuthPath: jwt})
	}
	return out
}

func productParent(globalDB string) string {
	return filepath.Dir(filepath.Dir(filepath.Dir(filepath.Dir(globalDB))))
}

func fileSeen(have []os.FileInfo, info os.FileInfo) bool {
	for _, h := range have {
		if os.SameFile(h, info) {
			return true
		}
	}
	return false
}

func isTraeProductName(name string) bool {
	n := strings.ToLower(strings.TrimSpace(name))
	switch {
	case n == "trae", n == "traecn", n == "trae_cn":
		return true
	case strings.HasPrefix(n, "trae "), strings.HasPrefix(n, "trae-"), strings.HasPrefix(n, "trae_"):
		return true
	default:
		return false
	}
}

func firstJWT(home adapter.Home) string {
	return adapter.FirstFile(
		filepath.Join(home.DotDir("trae-cn"), "trae-jwt-token"),
		filepath.Join(home.DotDir("trae"), "trae-jwt-token"),
	)
}

func (a Adapter) Parse(root adapter.SourceRoot, emit func(event.UsageEvent), emitTurn func(event.TurnEvent)) error {
	path := resolveDB(root.Path)
	if path == "" {
		return nil
	}
	sessions, err := collectSessionIDs(path)
	if err != nil {
		return err
	}
	token := strings.TrimSpace(readAuthFile(root.AuthPath))
	region := ""
	encrypted := false
	if token == "" {
		var info traeUserInfo
		token, info, encrypted = inspectStorageJSONAuth(path)
		region = strings.TrimSpace(info.UserRegion.Region)
		if region == "" {
			region = strings.TrimSpace(info.Account.StoreRegion)
		}
		if region == "" {
			region = strings.TrimSpace(info.Account.UserTag)
		}
	}
	token = stripJWTPrefix(token)
	if token == "" {
		if len(sessions) == 0 {
			return nil
		}
		if encrypted {
			return errEncryptedLocalAuth
		}
		return errNoLocalAuth
	}
	if a.Offline {
		return nil
	}
	events, apiErr := a.fetchAccountUsage(path, root.AuthPath, token, region, sessions)
	seenTurn := map[string]struct{}{}
	for _, e := range events {
		emit(e)
		if e.SessionID == "" {
			continue
		}
		if _, ok := seenTurn[e.SessionID]; ok {
			continue
		}
		seenTurn[e.SessionID] = struct{}{}
		emitTurn(event.TurnEvent{Source: "trae", SessionID: e.SessionID, Timestamp: e.Timestamp})
	}
	return apiErr
}

func resolveDB(p string) string {
	st, err := os.Stat(p)
	if err != nil {
		return ""
	}
	if !st.IsDir() {
		return p
	}
	return adapter.FirstFile(
		filepath.Join(p, "User", "globalStorage", "state.vscdb"),
		filepath.Join(p, "state.vscdb"),
	)
}

func collectSessionIDs(globalDB string) ([]string, error) {
	seen := map[string]struct{}{}
	var out []string
	var addErr error
	add := func(path string) {
		ids, err := sessionIDsFromDB(path)
		if err != nil {
			addErr = err
			return
		}
		for _, id := range ids {
			id = strings.TrimSpace(id)
			if id == "" {
				continue
			}
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			out = append(out, id)
		}
	}
	parent := productParent(globalDB)
	entries, err := os.ReadDir(parent)
	if err != nil {
		add(globalDB)
		addWorkspaceDBs(filepath.Join(filepath.Dir(globalDB), "..", "workspaceStorage"), add)
	} else {
		for _, e := range entries {
			if !e.IsDir() || !isTraeProductName(e.Name()) {
				continue
			}
			prod := filepath.Join(parent, e.Name())
			add(filepath.Join(prod, "User", "globalStorage", "state.vscdb"))
			addWorkspaceDBs(filepath.Join(prod, "User", "workspaceStorage"), add)
		}
	}
	if addErr != nil {
		return nil, addErr
	}
	return out, nil
}

func addWorkspaceDBs(wsDir string, add func(string)) {
	entries, err := os.ReadDir(wsDir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		add(filepath.Join(wsDir, e.Name(), "state.vscdb"))
	}
}

func sessionIDsFromDB(path string) ([]string, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, nil
	}
	db, err := openRO(path)
	if err != nil {
		return nil, nil
	}
	defer db.Close()

	seen := map[string]struct{}{}
	var out []string
	add := func(id string) {
		id = strings.TrimSpace(id)
		if id == "" {
			return
		}
		if _, ok := seen[id]; ok {
			return
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}

	var current sql.NullString
	_ = db.QueryRow(`SELECT json_extract(value, '$.currentSessionId') FROM ItemTable WHERE key = ?`, "memento/icube-ai-agent-storage").Scan(&current)
	add(current.String)

	rows, err := db.Query(`
SELECT json_extract(j.value, '$.sessionId')
FROM ItemTable, json_each(json_extract(ItemTable.value, '$.list')) AS j
WHERE ItemTable.key = 'memento/icube-ai-agent-storage'
ORDER BY CAST(j.key AS INTEGER)`)
	if err == nil {
		for rows.Next() {
			var id sql.NullString
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return nil, err
			}
			add(id.String)
		}
		rows.Close()
	}

	var extras []string
	rows, err = db.Query(`
SELECT json_each.key
FROM ItemTable, json_each(ItemTable.value)
WHERE ItemTable.key = 'icube_session_agent_map'`)
	if err == nil {
		for rows.Next() {
			var id sql.NullString
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return nil, err
			}
			id.String = strings.TrimSpace(id.String)
			if id.String == "" {
				continue
			}
			if _, ok := seen[id.String]; ok {
				continue
			}
			extras = append(extras, id.String)
		}
		rows.Close()
	}

	rows, err = db.Query(`SELECT key FROM ItemTable WHERE key LIKE 'all_session_badges_%'`)
	if err == nil {
		for rows.Next() {
			var key string
			if err := rows.Scan(&key); err != nil {
				rows.Close()
				return nil, err
			}
			id := strings.TrimSpace(strings.TrimPrefix(key, "all_session_badges_"))
			if id == "" {
				continue
			}
			if _, ok := seen[id]; ok {
				continue
			}
			extras = append(extras, id)
		}
		rows.Close()
	}
	sort.Strings(extras)
	for _, id := range extras {
		add(id)
	}
	return out, nil
}

func readAuthFile(path string) string {
	if path == "" {
		return ""
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(raw))
}

func stripJWTPrefix(token string) string {
	token = strings.TrimSpace(token)
	for _, p := range []string{"Cloud-IDE-JWT ", "Bearer "} {
		if strings.HasPrefix(token, p) {
			return strings.TrimSpace(strings.TrimPrefix(token, p))
		}
	}
	return token
}

func plaintextStorageToken(s string) string {
	s = strings.TrimSpace(s)
	s = stripJWTPrefix(s)
	if strings.HasPrefix(s, "eyJ") && strings.Count(s, ".") >= 2 {
		return s
	}
	return ""
}

func inspectStorageJSONAuth(vscdbPath string) (token string, info traeUserInfo, encrypted bool) {
	p := filepath.Join(filepath.Dir(vscdbPath), "storage.json")
	raw, err := os.ReadFile(p)
	if err != nil {
		return "", traeUserInfo{}, false
	}
	var obj map[string]any
	if json.Unmarshal(raw, &obj) != nil {
		return "", traeUserInfo{}, false
	}
	for k, v := range obj {
		if !strings.Contains(k, "iCubeAuthInfo://") {
			continue
		}
		s, _ := v.(string)
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if !strings.HasPrefix(s, "{") {
			if tok := plaintextStorageToken(s); tok != "" {
				token = tok
				continue
			}
			if u, ok := decryptUserInfo(s); ok {
				token = u.Token
				info = u
				continue
			}
			encrypted = true
			continue
		}
		var auth traeUserInfo
		if json.Unmarshal([]byte(s), &auth) != nil {
			continue
		}
		if tok := strings.TrimSpace(auth.Token); tok != "" {
			token = tok
			info = auth
		}
	}
	return token, info, encrypted
}

func openRO(path string) (*sql.DB, error) {
	return adapter.OpenRO(path)
}
