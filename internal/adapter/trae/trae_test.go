package trae

import (
	"database/sql"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/rainhuang0220/whereToken/internal/adapter"
	"github.com/rainhuang0220/whereToken/internal/adapter/testhome"
	"github.com/rainhuang0220/whereToken/internal/event"
)

func TestDiscoverSkipsMissingQuietly(t *testing.T) {
	roots := Adapter{}.Discover(testhome.New(t.TempDir()))
	if len(roots) != 0 {
		t.Fatalf("roots=%v", roots)
	}
}

func TestDiscoverMacAppSupportTraeCN(t *testing.T) {
	dir := t.TempDir()
	db := writeProductVscdb(t, dir, "Trae CN", nil)
	roots := Adapter{}.Discover(testhome.New(dir))
	if len(roots) != 1 || roots[0].ID != "trae" {
		t.Fatalf("roots=%v", roots)
	}
	if roots[0].Path != db && !strings.Contains(roots[0].Path, "Trae CN") {
		t.Fatalf("path=%q want Trae CN db %q", roots[0].Path, db)
	}
}

func TestDiscoverOneRootForSiblingMacProducts(t *testing.T) {
	dir := t.TempDir()
	writeProductVscdb(t, dir, "Trae CN", nil)
	writeProductVscdb(t, dir, "TRAE SOLO CN", nil)
	roots := Adapter{}.Discover(testhome.New(dir))
	if len(roots) != 1 {
		t.Fatalf("sibling Trae apps under Application Support must be one source, got %d: %v", len(roots), roots)
	}
}

func TestParseUnionsSiblingProductSessions(t *testing.T) {
	var got []string
	var gotMu sync.Mutex
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		gotMu.Lock()
		got = append(got, string(body))
		gotMu.Unlock()
		io.WriteString(w, `{"user_usage_group_by_session":{"session_id":"x","model_name":"DeepSeek-V4-Flash","extra_info":{"input_token":1,"output_token":1,"cache_read_token":0,"cache_write_token":0}}}`)
	}))
	t.Cleanup(srv.Close)
	dir := t.TempDir()
	writeProductVscdb(t, dir, "Trae CN", []kv{
		{key: "memento/icube-ai-agent-storage", value: `{"list":[{"sessionId":"sess-cn"}]}`},
	})
	writeProductVscdb(t, dir, "TRAE SOLO CN", []kv{
		{key: "memento/icube-ai-agent-storage", value: `{"list":[{"sessionId":"sess-solo"}]}`},
	})
	jwt := filepath.Join(dir, "jwt")
	if err := os.WriteFile(jwt, []byte(fakeJWT), 0o600); err != nil {
		t.Fatal(err)
	}
	roots := Adapter{}.Discover(testhome.New(dir))
	if len(roots) != 1 {
		t.Fatalf("roots=%d", len(roots))
	}
	roots[0].AuthPath = jwt
	a := Adapter{HTTP: srv.Client(), APIBase: srv.URL}
	if err := a.Parse(roots[0], func(event.UsageEvent) {}, func(event.TurnEvent) {}); err != nil {
		t.Fatal(err)
	}
	blob := strings.Join(got, "\n")
	if !strings.Contains(blob, "sess-cn") || !strings.Contains(blob, "sess-solo") {
		t.Fatalf("expected both sibling sessions, requests=%v", got)
	}
}

func TestDiscoverLinuxConfigAndWindowsAppData(t *testing.T) {
	dir := t.TempDir()
	linux := filepath.Join(dir, ".config", "Trae", "User", "globalStorage")
	if err := os.MkdirAll(linux, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(linux, "state.vscdb"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	win := filepath.Join(dir, "AppData", "Roaming", "Trae-CN", "User", "globalStorage")
	if err := os.MkdirAll(win, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(win, "state.vscdb"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	roots := Adapter{}.Discover(testhome.New(dir))
	if len(roots) < 2 {
		t.Fatalf("want linux+windows roots, got %v", roots)
	}
	var sawLinux, sawWin bool
	for _, r := range roots {
		if strings.Contains(r.Path, filepath.Join(".config", "Trae")) {
			sawLinux = true
		}
		if strings.Contains(r.Path, filepath.Join("AppData", "Roaming", "Trae-CN")) {
			sawWin = true
		}
	}
	if !sawLinux || !sawWin {
		t.Fatalf("linux=%v win=%v roots=%v", sawLinux, sawWin, roots)
	}
}

func TestDiscoverFindsJwtWithoutStoringInPath(t *testing.T) {
	dir := t.TempDir()
	writeProductVscdb(t, dir, "Trae CN", nil)
	jwtDir := filepath.Join(dir, ".trae-cn")
	if err := os.MkdirAll(jwtDir, 0o755); err != nil {
		t.Fatal(err)
	}
	secret := "eyJtest-not-a-real-token"
	if err := os.WriteFile(filepath.Join(jwtDir, "trae-jwt-token"), []byte(secret), 0o600); err != nil {
		t.Fatal(err)
	}
	roots := Adapter{}.Discover(testhome.New(dir))
	if len(roots) != 1 {
		t.Fatalf("roots=%v", roots)
	}
	if roots[0].AuthPath == "" {
		t.Fatal("expected AuthPath to jwt file")
	}
	if strings.Contains(roots[0].Path, secret) || strings.Contains(roots[0].AuthPath, secret) {
		t.Fatal("source root leaked jwt contents")
	}
}

func TestDiscoverIgnoresDotTraeSkillsDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".trae", "skills"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, ".trae-cn", "skills"), 0o755); err != nil {
		t.Fatal(err)
	}
	roots := Adapter{}.Discover(testhome.New(dir))
	if len(roots) != 0 {
		t.Fatalf("skills-only dirs must not count as ledgers: %v", roots)
	}
}

func TestParseMissingAuthChineseErrorNoSecret(t *testing.T) {
	dir := t.TempDir()
	db := writeProductVscdb(t, dir, "Trae CN", []kv{
		{key: "memento/icube-ai-agent-storage", value: `{"currentSessionId":"sess-1","list":[{"sessionId":"sess-1","messages":[]}]}`},
	})
	err := (Adapter{}).Parse(adapter.SourceRoot{ID: "trae", Path: db}, func(event.UsageEvent) {}, func(event.TurnEvent) {})
	if err == nil || !strings.Contains(err.Error(), "未找到本机登录态") {
		t.Fatalf("err=%v", err)
	}
	if strings.Contains(strings.ToLower(err.Error()), "bearer") || strings.Contains(err.Error(), "eyJ") || strings.Contains(err.Error(), "Cloud-IDE-JWT") {
		t.Fatal("error must not include credentials")
	}
}

func TestParseEncryptedStorageJSONChineseErrorNoSecret(t *testing.T) {
	dir := t.TempDir()
	db := writeProductVscdb(t, dir, "Trae CN", []kv{
		{key: "memento/icube-ai-agent-storage", value: `{"list":[{"sessionId":"sess-1"}]}`},
	})
	// Trae CN encrypts iCubeAuthInfo; fixture is a format prefix only, not a real token.
	raw := []byte(`{"iCubeAuthInfo://icube.cloudide":"dGMFEAAAfixture-encrypted-blob"}`)
	if err := os.WriteFile(filepath.Join(filepath.Dir(db), "storage.json"), raw, 0o600); err != nil {
		t.Fatal(err)
	}
	err := (Adapter{}).Parse(adapter.SourceRoot{ID: "trae", Path: db}, func(event.UsageEvent) {}, func(event.TurnEvent) {})
	if err == nil || !strings.Contains(err.Error(), "登录态在加密存储中，没有可读的 JWT 文件") {
		t.Fatalf("err=%v", err)
	}
	if strings.Contains(err.Error(), "dGMFEAAA") || strings.Contains(err.Error(), "fixture-encrypted") || strings.Contains(err.Error(), "eyJ") {
		t.Fatalf("error leaked storage blob: %v", err)
	}
}

func TestParseEmptyDirEmitsNothing(t *testing.T) {
	var n int
	err := (Adapter{}).Parse(adapter.SourceRoot{ID: "trae", Path: t.TempDir()}, func(event.UsageEvent) {
		n++
	}, func(event.TurnEvent) {})
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("emitted %d", n)
	}
}

func TestProductionDoesNotLogSecrets(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	dir := filepath.Dir(file)
	for _, name := range []string{"trae.go", "api.go"} {
		raw, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		body := string(raw)
		if strings.Contains(body, "log.") {
			t.Fatalf("%s must not log", name)
		}
		if strings.Contains(body, "fmt.Print") {
			t.Fatalf("%s must not print", name)
		}
	}
}

func TestDiscoverDoesNotUseOwnerHome(t *testing.T) {
	dir := t.TempDir()
	writeProductVscdb(t, dir, "Trae CN", nil)
	roots := Adapter{}.Discover(testhome.New(dir))
	if len(roots) == 0 {
		t.Fatal("expected a root under the fake home")
	}
	for _, r := range roots {
		if !strings.HasPrefix(r.Path, dir) {
			t.Fatalf("path %q not under fake home %q", r.Path, dir)
		}
	}
}

type kv struct{ key, value string }

func writeProductVscdb(t *testing.T, home, product string, rows []kv) string {
	t.Helper()
	dir := filepath.Join(home, "Library", "Application Support", product, "User", "globalStorage")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "state.vscdb")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE ItemTable (key TEXT UNIQUE ON CONFLICT REPLACE, value BLOB)`); err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		if _, err := db.Exec(`INSERT INTO ItemTable (key, value) VALUES (?, ?)`, row.key, row.value); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}
