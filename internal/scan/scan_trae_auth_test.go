package scan

import (
	"database/sql"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/rainhuang0220/whereToken/internal/adapter"
	"github.com/rainhuang0220/whereToken/internal/adapter/testhome"
	"github.com/rainhuang0220/whereToken/internal/adapter/trae"
	"github.com/rainhuang0220/whereToken/internal/event"
)

func TestRunReReadsTraeAuthFromDiskEachScan(t *testing.T) {
	t.Setenv("WHERETOKEN_EXTRA_ROOTS", "")
	dir := t.TempDir()
	writeScanTraeSessionDB(t, dir, "sess-rotate")
	jwtPath := filepath.Join(dir, ".trae-cn", "trae-jwt-token")
	if err := os.MkdirAll(filepath.Dir(jwtPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(jwtPath, []byte("expired-session"), 0o600); err != nil {
		t.Fatal(err)
	}

	var sawExpired, sawFresh int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		switch {
		case strings.Contains(auth, "expired-session"):
			sawExpired++
			w.WriteHeader(http.StatusUnauthorized)
		case strings.Contains(auth, "fresh-session"):
			sawFresh++
			io.WriteString(w, `{
			  "user_usage_group_by_session": {
			    "model_name": "DeepSeek-V4-Flash",
			    "session_id": "sess-rotate",
			    "extra_info": {
			      "input_token": 1000,
			      "output_token": 20,
			      "cache_read_token": 200,
			      "cache_write_token": 50
			    }
			  }
			}`)
		default:
			w.WriteHeader(http.StatusUnauthorized)
		}
	}))
	t.Cleanup(srv.Close)

	adapters := []adapter.Adapter{trae.Adapter{HTTP: srv.Client(), APIBase: srv.URL}}
	home := testhome.New(dir)

	first := Run(home, adapters)
	var traeFirst *event.Quality
	var firstTotal int64
	for i := range first.Summary.BySource {
		if first.Summary.BySource[i].ID == "trae" {
			q := first.Summary.BySource[i].Quality
			traeFirst = &q
			firstTotal = first.Summary.BySource[i].Total()
		}
	}
	if traeFirst == nil {
		t.Fatal("expected trae row after expired login")
	}
	if *traeFirst != event.QualityDegraded {
		t.Fatalf("first quality=%s want degraded", *traeFirst)
	}
	if firstTotal != 0 {
		t.Fatalf("first total=%d want 0", firstTotal)
	}
	foundExpired := false
	for _, e := range first.Errors {
		if strings.Contains(e, "本机登录态已失效") {
			foundExpired = true
		}
		if strings.Contains(e, "expired-session") || strings.Contains(e, "fresh-session") || strings.Contains(e, "eyJ") {
			t.Fatalf("error leaked auth: %s", e)
		}
	}
	if !foundExpired {
		t.Fatalf("first errors=%v", first.Errors)
	}
	if sawExpired == 0 {
		t.Fatal("first scan must send the on-disk expired session")
	}

	if err := os.WriteFile(jwtPath, []byte("fresh-session"), 0o600); err != nil {
		t.Fatal(err)
	}

	second := Run(home, adapters)
	var traeSecond QualityView
	for i := range second.Summary.BySource {
		if second.Summary.BySource[i].ID == "trae" {
			traeSecond.Quality = second.Summary.BySource[i].Quality
			traeSecond.Total = second.Summary.BySource[i].Total()
		}
	}
	if traeSecond.Quality == event.QualityDegraded {
		t.Fatalf("second scan must recover after JWT file rotation, quality=%s errors=%v", traeSecond.Quality, second.Errors)
	}
	if traeSecond.Quality != event.QualityAuthoritative {
		t.Fatalf("second quality=%s want authoritative", traeSecond.Quality)
	}
	if traeSecond.Total != 1070 {
		t.Fatalf("second total=%d want 1070", traeSecond.Total)
	}
	if sawFresh == 0 {
		t.Fatal("second scan must re-read the rotated file, not reuse the expired session")
	}
	for _, e := range second.Errors {
		if strings.Contains(e, "expired-session") || strings.Contains(e, "fresh-session") || strings.Contains(e, "eyJ") {
			t.Fatalf("error leaked auth: %s", e)
		}
	}
}

type QualityView struct {
	Quality event.Quality
	Total   int64
}

func writeScanTraeSessionDB(t *testing.T, home, sessionID string) {
	t.Helper()
	dir := filepath.Join(home, "Library", "Application Support", "Trae CN", "User", "globalStorage")
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
	value := `{"list":[{"sessionId":"` + sessionID + `"}]}`
	if _, err := db.Exec(`INSERT INTO ItemTable (key, value) VALUES (?, ?)`, "memento/icube-ai-agent-storage", value); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
}
