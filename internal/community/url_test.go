package community

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rainhuang0220/whereToken/internal/event"
)

func TestParseServiceURLRejectsNonHTTP(t *testing.T) {
	ok, err := ParseServiceURL("https://rank.example:8443/base/")
	if err != nil || ok != "https://rank.example:8443/base" {
		t.Fatalf("%q %v", ok, err)
	}
	ipv6, err := ParseServiceURL("http://[::1]:8798")
	if err != nil || ipv6 != "http://[::1]:8798" {
		t.Fatalf("ipv6 %q %v", ipv6, err)
	}
	for _, raw := range []string{
		"",
		"file:///tmp/community",
		"javascript:alert(1)",
		"ftp://rank.example",
		"http://user:pass@rank.example",
		"https://user@rank.example",
		"/v1/community/usage",
		"rank.example",
	} {
		if _, err := ParseServiceURL(raw); err == nil {
			t.Fatalf("accepted %q", raw)
		}
	}
}

func TestEnvURLDropsInvalidSchemes(t *testing.T) {
	if EnvURL(func(string) string { return "file:///etc/passwd" }) != "" {
		t.Fatal("file url")
	}
	if EnvURL(func(string) string { return "http://127.0.0.1:8798/" }) != "http://127.0.0.1:8798" {
		t.Fatal("http")
	}
}

func TestClientRefusesRedirectAndDoesNotReplayBody(t *testing.T) {
	var leaked []byte
	final := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/community/usage" {
			leaked, _ = io.ReadAll(r.Body)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(final.Close)
	redir := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, final.URL+r.URL.Path, http.StatusFound)
	}))
	t.Cleanup(redir.Close)

	id := "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee"
	c := &Client{
		BaseURL:  redir.URL,
		File:     &File{ParticipantID: id, Enabled: true},
		Version:  "0.5.0",
		MinCache: time.Hour,
	}
	now := time.Date(2026, 8, 19, 15, 0, 0, 0, time.UTC)
	events := []event.UsageEvent{{Vendor: "anthropic", Model: "claude-opus-4.6", Miss: 1000, Timestamp: now}}
	v := c.Sync(t.Context(), events, now, time.UTC)
	if v.Today.Status != StatusNetworkError || v.Today.Rank != 0 || v.Today.Display != "" {
		t.Fatalf("redirect must be a network error, not a rank: %+v", v.Today)
	}
	if len(leaked) > 0 {
		t.Fatalf("upload followed redirect: %s", leaked)
	}
	if strings.Contains(string(leaked), id) {
		t.Fatal("participant_id hopped")
	}
}

func TestClientFileURLNeverOpens(t *testing.T) {
	c := &Client{
		BaseURL: "file:///tmp/secret",
		File:    &File{ParticipantID: "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee", Enabled: true},
		Version: "0.5.0",
		HTTP:    &http.Client{Timeout: 50 * time.Millisecond},
	}
	now := time.Date(2026, 8, 19, 15, 0, 0, 0, time.UTC)
	v := c.Sync(t.Context(), []event.UsageEvent{{Vendor: "anthropic", Model: "claude-opus-4.6", Miss: 10, Timestamp: now}}, now, time.UTC)
	if v.Today.Status != StatusServiceUnconfigured || Caption(v.Today) != "—" {
		t.Fatalf("%+v", v.Today)
	}
}

func TestHandlerRateLimitsPerIP(t *testing.T) {
	h := NewHandler(NewStore(1))
	clock := time.Date(2026, 8, 19, 15, 0, 0, 0, time.UTC)
	h.now = func() time.Time { return clock }
	id := "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee"
	mux := h.Mux()
	ok, limited := 0, 0
	for i := 0; i < 61; i++ {
		body := `{"participant_id":"` + id + `","period":"2026-08-19","tokens":10,"client_version":"0.5.0"}`
		req := httptest.NewRequest(http.MethodPost, "/v1/community/usage", strings.NewReader(body))
		req.RemoteAddr = "203.0.113.9:4444"
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		switch rec.Code {
		case http.StatusNoContent:
			ok++
		case http.StatusTooManyRequests:
			limited++
		default:
			t.Fatalf("hit %d status %d", i, rec.Code)
		}
	}
	if ok != 60 || limited != 1 {
		t.Fatalf("ok=%d limited=%d", ok, limited)
	}
}
