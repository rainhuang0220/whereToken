package trae

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rainhuang0220/whereToken/internal/adapter"
	"github.com/rainhuang0220/whereToken/internal/event"
	"github.com/rainhuang0220/whereToken/internal/metric"
	"github.com/rainhuang0220/whereToken/internal/vendor"
)

const fakeJWT = "test-token"

func TestParseBillingMapsCacheAsWhereTokenColumns(t *testing.T) {
	var sawAuth, sawSession bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if strings.Contains(auth, fakeJWT) {
			sawAuth = true
		}
		if strings.Contains(strings.ToLower(auth), "test-token") && !strings.HasPrefix(auth, "Cloud-IDE-JWT ") && auth != "Bearer "+fakeJWT {
			// allow Cloud-IDE-JWT or Bearer, nothing else that prints the token twice
		}
		if strings.Contains(r.URL.Path, "get_session_usage") {
			body, _ := io.ReadAll(r.Body)
			if strings.Contains(string(body), "sess-1") {
				sawSession = true
			}
			io.WriteString(w, `{
			  "user_usage_group_by_session": {
			    "credits_float": 0.04,
			    "model_name": "DeepSeek-V4-Flash",
			    "session_id": "sess-1",
			    "extra_info": {
			      "input_token": 1000,
			      "output_token": 20,
			      "cache_read_token": 200,
			      "cache_write_token": 50
			    }
			  }
			}`)
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)

	dir := t.TempDir()
	db := writeProductVscdb(t, dir, "Trae CN", []kv{
		{key: "memento/icube-ai-agent-storage", value: `{"currentSessionId":"sess-1","list":[{"sessionId":"sess-1","messages":[]}]}`},
		{key: "icube_session_agent_map", value: `{"sess-1":"solo_agent"}`},
	})
	jwt := filepath.Join(dir, "jwt")
	if err := os.WriteFile(jwt, []byte(fakeJWT), 0o600); err != nil {
		t.Fatal(err)
	}

	var evs []event.UsageEvent
	var turns []event.TurnEvent
	a := Adapter{HTTP: srv.Client(), APIBase: srv.URL}
	if err := a.Parse(adapter.SourceRoot{ID: "trae", Path: db, AuthPath: jwt}, func(e event.UsageEvent) {
		evs = append(evs, e)
	}, func(te event.TurnEvent) {
		turns = append(turns, te)
	}); err != nil {
		t.Fatal(err)
	}
	if !sawAuth {
		t.Fatal("expected Authorization to carry the local session token")
	}
	if !sawSession {
		t.Fatal("expected session_id in get_session_usage body")
	}
	if len(evs) != 1 {
		t.Fatalf("events=%d", len(evs))
	}
	e := evs[0]
	if e.Source != "trae" {
		t.Fatalf("source=%q", e.Source)
	}
	if e.Vendor != "deepseek" {
		t.Fatalf("vendor=%q want deepseek not trae", e.Vendor)
	}
	if e.Miss != 800 || e.CacheRead != 200 || e.CacheCreate != 50 || e.Output != 20 {
		t.Fatalf("tokens %+v", e)
	}
	if e.Quality != event.QualityAuthoritative {
		t.Fatalf("quality=%s", e.Quality)
	}
	sum := metric.Aggregate(evs, turns)
	if sum.All.Total() != 1070 {
		t.Fatalf("total=%d want miss+cache_read+cache_create+output", sum.All.Total())
	}
	if len(turns) != 1 {
		t.Fatalf("user turns=%d want 1 from billing session_id (a real user message id)", len(turns))
	}
}

func TestParseBillingFullCacheHitMissIsZero(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{
		  "user_usage_group_by_session": {
		    "model_name": "Doubao-Seed-2.0-Code",
		    "session_id": "user-message-1",
		    "extra_info": {
		      "input_token": 29234,
		      "output_token": 18,
		      "cache_read_token": 29234,
		      "cache_write_token": 0
		    }
		  }
		}`)
	}))
	t.Cleanup(srv.Close)
	dir := t.TempDir()
	db := writeProductVscdb(t, dir, "Trae CN", []kv{
		{key: "memento/icube-ai-agent-storage", value: `{"list":[{"sessionId":"user-message-1"}]}`},
	})
	jwt := filepath.Join(dir, "jwt")
	if err := os.WriteFile(jwt, []byte(fakeJWT), 0o600); err != nil {
		t.Fatal(err)
	}
	var evs []event.UsageEvent
	a := Adapter{HTTP: srv.Client(), APIBase: srv.URL, Now: func() time.Time { return time.Unix(1_700_000_000, 0).UTC() }}
	if err := a.Parse(adapter.SourceRoot{ID: "trae", Path: db, AuthPath: jwt}, func(e event.UsageEvent) {
		evs = append(evs, e)
	}, func(event.TurnEvent) {}); err != nil {
		t.Fatal(err)
	}
	if len(evs) != 1 {
		t.Fatalf("events=%d", len(evs))
	}
	if evs[0].Miss != 0 || evs[0].CacheRead != 29234 || evs[0].Output != 18 {
		t.Fatalf("%+v", evs[0])
	}
	if evs[0].Vendor != "doubao" {
		t.Fatalf("vendor=%q", evs[0].Vendor)
	}
}

func TestParseNeverLogsOrReturnsJWT(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	t.Cleanup(srv.Close)
	dir := t.TempDir()
	db := writeProductVscdb(t, dir, "Trae CN", []kv{
		{key: "memento/icube-ai-agent-storage", value: `{"list":[{"sessionId":"sess-1"}]}`},
	})
	jwt := filepath.Join(dir, "jwt")
	secret := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.aaa.bbb"
	if err := os.WriteFile(jwt, []byte(secret), 0o600); err != nil {
		t.Fatal(err)
	}
	err := (Adapter{HTTP: srv.Client(), APIBase: srv.URL}).Parse(adapter.SourceRoot{ID: "trae", Path: db, AuthPath: jwt}, func(event.UsageEvent) {}, func(event.TurnEvent) {})
	if err == nil {
		t.Fatal("expected auth error")
	}
	if strings.Contains(err.Error(), secret) || strings.Contains(err.Error(), "eyJ") {
		t.Fatalf("error leaked jwt: %v", err)
	}
}

func TestVendorLookupNotTrae(t *testing.T) {
	if vendor.Lookup("DeepSeek-V4-Flash", "") != "deepseek" {
		t.Fatal(vendor.Lookup("DeepSeek-V4-Flash", ""))
	}
	if vendor.Lookup("Doubao-Seed-2.0-Code", "") != "doubao" {
		t.Fatal(vendor.Lookup("Doubao-Seed-2.0-Code", ""))
	}
	if vendor.Lookup("glm-5.2", "") != "zhipu" {
		t.Fatal(vendor.Lookup("glm-5.2", ""))
	}
	if vendor.Lookup("qwen-3.7-plus", "") != "alibaba" {
		t.Fatal(vendor.Lookup("qwen-3.7-plus", ""))
	}
	if vendor.Lookup("MiniMax-M3", "") == "trae" {
		t.Fatal("minimax must not become trae")
	}
}

func TestOfflineDoesNotCallBillingAPI(t *testing.T) {
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		hits++
	}))
	t.Cleanup(srv.Close)
	dir := t.TempDir()
	db := writeProductVscdb(t, dir, "Trae CN", []kv{
		{key: "memento/icube-ai-agent-storage", value: `{"list":[{"sessionId":"sess-1"}]}`},
	})
	jwt := filepath.Join(dir, "jwt")
	if err := os.WriteFile(jwt, []byte(fakeJWT), 0o600); err != nil {
		t.Fatal(err)
	}
	a := Adapter{HTTP: srv.Client(), APIBase: srv.URL, Offline: true}
	if err := a.Parse(adapter.SourceRoot{ID: "trae", Path: db, AuthPath: jwt}, func(event.UsageEvent) {}, func(event.TurnEvent) {}); err != nil {
		t.Fatal(err)
	}
	if hits != 0 {
		t.Fatalf("offline still hit API %d times", hits)
	}
}

func TestParseSessionUsageDoesNotDoubleParentAndLeaf(t *testing.T) {
	raw := []byte(`{
	  "data": {
	    "input_token": 1000,
	    "output_token": 20,
	    "user_usage_group_by_session": {
	      "session_id": "s1",
	      "model_name": "DeepSeek-V4-Flash",
	      "extra_info": {
	        "input_token": 1000,
	        "output_token": 20,
	        "cache_read_token": 0,
	        "cache_write_token": 0
	      }
	    }
	  }
	}`)
	evs := parseSessionUsage(raw, "/tmp")
	if len(evs) != 1 {
		t.Fatalf("events=%d %+v", len(evs), evs)
	}
	sum := metric.Aggregate(evs, nil)
	if sum.All.Total() != 1020 {
		t.Fatalf("double-counted parent+leaf total=%d", sum.All.Total())
	}
}

func TestParseSessionUsageSumsTwoModelsInOneSession(t *testing.T) {
	raw := []byte(`{
	  "user_usage_group_by_session": [
	    {"session_id":"s1","model_name":"DeepSeek-V4-Flash","extra_info":{"input_token":100,"output_token":10}},
	    {"session_id":"s1","model_name":"glm-5","extra_info":{"input_token":200,"output_token":20}}
	  ]
	}`)
	evs := parseSessionUsage(raw, "/tmp")
	sum := metric.Aggregate(evs, nil)
	if sum.All.Total() != 330 {
		t.Fatalf("same session_id collapsed to max: total=%d events=%d", sum.All.Total(), len(evs))
	}
}

func TestParseSessionUsageLeavesUnknownTimeZero(t *testing.T) {
	raw := []byte(`{"session_id":"s1","model_name":"k3","extra_info":{"input_token":10,"output_token":1}}`)
	evs := parseSessionUsage(raw, "/tmp")
	if len(evs) != 1 {
		t.Fatalf("events=%d", len(evs))
	}
	if !evs[0].Timestamp.IsZero() {
		t.Fatalf("no time in payload must stay zero so the kiln does not dump it on today: %s", evs[0].Timestamp)
	}
}

func TestAPIBaseUsesCNJWTWhenProductDirIsInternational(t *testing.T) {
	got := (Adapter{}).apiBase(
		"/Library/Application Support/Trae/User/globalStorage/state.vscdb",
		"/home/me/.trae-cn/trae-jwt-token",
	)
	if got != defaultAPICN {
		t.Fatalf("CN JWT should pick the CN host, got %s", got)
	}
}

func TestAPIBaseInternationalWithoutCNHint(t *testing.T) {
	got := (Adapter{}).apiBase("/Library/Application Support/Trae/User/globalStorage/state.vscdb")
	if got != defaultAPISG {
		t.Fatalf("got %s", got)
	}
}

func TestDefaultHTTPClientTimeout(t *testing.T) {
	c := Adapter{}.client()
	if c.Timeout != 20*time.Second {
		t.Fatalf("timeout=%s", c.Timeout)
	}
	if c.CheckRedirect == nil {
		t.Fatal("missing redirect lock")
	}
}

func TestRestrictRedirectRejectsOtherHost(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "https://evil.example/steal", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := (Adapter{}).restrictRedirect(req, nil); err == nil {
		t.Fatal("expected reject")
	}
}

func TestRestrictRedirectAllowsTraeAPI(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "https://api.trae.ai/v1", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := (Adapter{}).restrictRedirect(req, nil); err != nil {
		t.Fatal(err)
	}
}

func TestRestrictRedirectHonorsAPIBase(t *testing.T) {
	a := Adapter{APIBase: "https://trae.test.example"}
	req, err := http.NewRequest(http.MethodGet, "https://trae.test.example/v1", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.restrictRedirect(req, nil); err != nil {
		t.Fatal(err)
	}
}

func TestAllowedURLRejectsLoopback(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "http://127.0.0.1/steal", nil)
	if err != nil {
		t.Fatal(err)
	}
	if (Adapter{}).allowedURL(req.URL) {
		t.Fatal("loopback should be rejected unless it is APIBase")
	}
}

func TestAllowedURLRejectsLookalikeHost(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "https://api.trae.ai.evil.example/v1", nil)
	if err != nil {
		t.Fatal(err)
	}
	if (Adapter{}).allowedURL(req.URL) {
		t.Fatal("suffix host must not match")
	}
}
