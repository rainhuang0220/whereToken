package hosted

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHealthJSONOmitsSecrets(t *testing.T) {
	h := NewMux(MuxOptions{Version: "0.7.0"})
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)

	res, err := http.Get(srv.URL + "/api/health")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d", res.StatusCode)
	}
	raw, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	var body map[string]any
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatal(err)
	}
	if body["status"] != "ok" {
		t.Fatalf("body %s", raw)
	}
	if body["version"] != "0.7.0" {
		t.Fatalf("version %s", raw)
	}
	s := strings.ToLower(string(raw))
	for _, bad := range []string{"password", "dsn", "secret", "mysql", "/home/", "github_client"} {
		if strings.Contains(s, bad) {
			t.Fatalf("health leaked %q: %s", bad, raw)
		}
	}
}

func TestDashboardRequiresSession(t *testing.T) {
	h := NewMux(MuxOptions{Version: "dev"})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard/summary", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestMuxHasNoLocalScanEndpoint(t *testing.T) {
	h := NewMux(MuxOptions{Version: "dev"})
	req := httptest.NewRequest(http.MethodPost, "/api/scan", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("hosted must not expose local /api/scan, got %d", rec.Code)
	}
}

func TestSecurityHeaders(t *testing.T) {
	h := NewMux(MuxOptions{Version: "dev"})
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("missing nosniff")
	}
	if rec.Header().Get("X-Frame-Options") != "DENY" {
		t.Fatal("missing frame deny")
	}
	if rec.Header().Get("Referrer-Policy") == "" {
		t.Fatal("missing referrer policy")
	}
}
