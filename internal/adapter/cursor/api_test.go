package cursor

import (
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/rainhuang0220/whereToken/internal/adapter"
	"github.com/rainhuang0220/whereToken/internal/event"
	"github.com/rainhuang0220/whereToken/internal/metric"
)

const fakeJWT = "test-token"

func TestParseAccountAPIMapsFilteredEvents(t *testing.T) {
	var sawAuth bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth == "Bearer "+fakeJWT {
			sawAuth = true
		}
		if strings.Contains(strings.ToLower(auth), "authorization") {
			t.Errorf("handler saw nested authorization text")
		}
		switch {
		case strings.Contains(r.URL.Path, "GetFilteredUsageEvents"):
			io.WriteString(w, `{
			  "usageEventsDisplay": [
			    {
			      "timestamp": "1770000000000",
			      "model": "claude-4.6-opus-high-thinking",
			      "conversationId": "sess-a",
			      "tokenUsage": {
			        "inputTokens": "50",
			        "outputTokens": "15",
			        "cacheWriteTokens": "25",
			        "cacheReadTokens": "400"
			      }
			    },
			    {
			      "timestamp": "1770086400000",
			      "model": "gpt-5",
			      "tokenUsage": {
			        "inputTokens": 10,
			        "outputTokens": 5,
			        "cacheWriteTokens": 0,
			        "cacheReadTokens": 80
			      }
			    }
			  ],
			  "totalUsageEventsCount": 2
			}`)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	dir := t.TempDir()
	db := writeVscdb(t, dir, []kv{
		{key: "composerData:sess-a", value: `{"composerId":"sess-a","createdAt":1700000000000,"modelConfig":{"modelName":"claude-opus-4-6"},"usageData":{}}`},
		{key: "bubbleId:sess-a:u1", value: `{"type":1,"createdAt":"2026-02-09T14:44:05.860Z","tokenCount":{"inputTokens":0,"outputTokens":0}}`},
		{key: "bubbleId:sess-a:a1", value: `{"type":2,"createdAt":"2026-02-09T14:44:08.000Z","tokenCount":{"inputTokens":100,"outputTokens":10}}`},
	}, []header{{id: "sess-a", workspace: "/tmp/whereToken"}})
	putItem(t, db, authAccessTokenKey, fakeJWT)

	var evs []event.UsageEvent
	var turns []event.TurnEvent
	a := Adapter{HTTP: srv.Client(), APIBase: srv.URL}
	if err := a.Parse(adapter.SourceRoot{ID: "cursor", Path: db}, func(e event.UsageEvent) {
		evs = append(evs, e)
	}, func(te event.TurnEvent) {
		turns = append(turns, te)
	}); err != nil {
		t.Fatal(err)
	}
	if !sawAuth {
		t.Fatal("expected Bearer test-token on DashboardService call")
	}
	if len(turns) != 1 {
		t.Fatalf("turns=%d", len(turns))
	}

	sum := metric.Aggregate(evs, turns)
	if sum.All.Requests != 1 {
		t.Fatalf("requests=%d want 1 from local bubbles, not API rows", sum.All.Requests)
	}
	if sum.All.UserTurns != 1 {
		t.Fatalf("turns=%d", sum.All.UserTurns)
	}
	if sum.All.Miss != 60 || sum.All.CacheRead != 480 || sum.All.CacheCreate != 25 || sum.All.Output != 20 {
		t.Fatalf("tokens %+v (must come from API, not local 100/10)", sum.All)
	}
	if sum.All.Quality != event.QualityAuthoritative {
		t.Fatalf("quality=%s", sum.All.Quality)
	}

	var anthropic, openai *metric.Slice
	for i := range sum.ByVendor {
		s := &sum.ByVendor[i]
		switch s.ID {
		case "anthropic":
			anthropic = s
		case "openai":
			openai = s
		}
	}
	if anthropic == nil || anthropic.Total() != 50+400+25+15 {
		t.Fatalf("anthropic %+v", anthropic)
	}
	if openai == nil || openai.Total() != 10+80+5 {
		t.Fatalf("openai %+v", openai)
	}

	days := sum.Calendar.BySource["cursor"].Days
	got := map[string]int64{}
	for _, d := range days {
		got[d.Date] = d.Total
	}
	d1 := time.UnixMilli(1770000000000).In(time.Local).Format("2006-01-02")
	d2 := time.UnixMilli(1770086400000).In(time.Local).Format("2006-01-02")
	if got[d1] != 490 || got[d2] != 95 {
		t.Fatalf("calendar days=%v want %s=490 %s=95", got, d1, d2)
	}
}

func TestParseAccountAPIFallsBackToAggregations(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+fakeJWT {
			http.Error(w, "nope", http.StatusUnauthorized)
			return
		}
		switch {
		case strings.Contains(r.URL.Path, "GetFilteredUsageEvents"):
			http.NotFound(w, r)
		case strings.Contains(r.URL.Path, "GetAggregatedUsageEvents"):
			io.WriteString(w, `{
			  "aggregations": [
			    {
			      "modelIntent": "claude-4.6-opus-high-thinking",
			      "inputTokens": "70",
			      "outputTokens": "9",
			      "cacheWriteTokens": "11",
			      "cacheReadTokens": "300"
			    }
			  ],
			  "totalInputTokens": "70",
			  "totalOutputTokens": "9",
			  "totalCacheWriteTokens": "11",
			  "totalCacheReadTokens": "300"
			}`)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	dir := t.TempDir()
	db := writeVscdb(t, dir, []kv{
		{key: "composerData:sess-a", value: `{"composerId":"sess-a","createdAt":1700000000000,"modelConfig":{"modelName":"claude-opus-4-6"},"usageData":{}}`},
		{key: "bubbleId:sess-a:a1", value: `{"type":2,"createdAt":"2026-02-09T14:44:08.000Z","tokenCount":{"inputTokens":0,"outputTokens":0}}`},
	}, nil)
	putItem(t, db, authAccessTokenKey, fakeJWT)

	var evs []event.UsageEvent
	a := Adapter{HTTP: srv.Client(), APIBase: srv.URL, Now: func() time.Time {
		return time.Date(2026, 8, 15, 12, 0, 0, 0, time.UTC)
	}}
	if err := a.Parse(adapter.SourceRoot{ID: "cursor", Path: db}, func(e event.UsageEvent) {
		evs = append(evs, e)
	}, func(event.TurnEvent) {}); err != nil {
		t.Fatal(err)
	}
	sum := metric.Aggregate(evs, nil)
	if sum.All.Miss != 70 || sum.All.CacheRead != 300 || sum.All.CacheCreate != 11 || sum.All.Output != 9 {
		t.Fatalf("agg tokens %+v", sum.All)
	}
	if sum.All.Requests != 1 {
		t.Fatalf("requests=%d", sum.All.Requests)
	}
	if sum.All.Quality != event.QualityAuthoritative {
		t.Fatalf("quality=%s", sum.All.Quality)
	}
}

func TestParseAccountAPIErrorsDoNotLeakToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "upstream boom", http.StatusBadGateway)
	}))
	t.Cleanup(srv.Close)

	dir := t.TempDir()
	db := writeVscdb(t, dir, []kv{
		{key: "composerData:sess-a", value: `{"composerId":"sess-a","createdAt":1700000000000,"usageData":{}}`},
		{key: "bubbleId:sess-a:a1", value: `{"type":2,"createdAt":"2026-02-09T14:44:08.000Z","tokenCount":{"inputTokens":0,"outputTokens":0}}`},
	}, nil)
	putItem(t, db, authAccessTokenKey, fakeJWT)

	err := (Adapter{HTTP: srv.Client(), APIBase: srv.URL}).Parse(adapter.SourceRoot{ID: "cursor", Path: db}, func(event.UsageEvent) {}, func(event.TurnEvent) {})
	if err == nil {
		t.Fatal("expected API error")
	}
	dump, _ := json.Marshal(err.Error())
	if strings.Contains(err.Error(), fakeJWT) || strings.Contains(strings.ToLower(err.Error()), "bearer "+fakeJWT) || strings.Contains(string(dump), fakeJWT) {
		t.Fatal("error leaked token")
	}
}

func TestProductionSQLDoesNotDumpItemTable(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	src, err := os.ReadFile(filepath.Join(filepath.Dir(file), "auth.go"))
	if err != nil {
		t.Fatal(err)
	}
	body := string(src)
	if strings.Contains(body, "SELECT * FROM ItemTable") {
		t.Fatal("must not SELECT * from ItemTable")
	}
	if !strings.Contains(body, "WHERE key = ?") {
		t.Fatal("ItemTable must be keyed")
	}
	if !strings.Contains(body, authAccessTokenKey) {
		t.Fatal("must read cursorAuth/accessToken by name")
	}
}

func putItem(t *testing.T, dbPath, key, value string) {
	t.Helper()
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`INSERT INTO ItemTable (key, value) VALUES (?, ?)`, key, value); err != nil {
		t.Fatal(err)
	}
}

func TestOfflineDoesNotCallAccountAPI(t *testing.T) {
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		hits++
	}))
	t.Cleanup(srv.Close)
	dir := t.TempDir()
	db := writeVscdb(t, dir, []kv{
		{key: "composerData:sess-a", value: `{"composerId":"sess-a","createdAt":1700000000000}`},
		{key: "bubbleId:sess-a:u1", value: `{"type":1,"createdAt":"2026-02-09T14:44:05.860Z"}`},
	}, []header{{id: "sess-a", workspace: "/tmp/whereToken"}})
	putItem(t, db, authAccessTokenKey, fakeJWT)
	a := Adapter{HTTP: srv.Client(), APIBase: srv.URL, Offline: true}
	if err := a.Parse(adapter.SourceRoot{ID: "cursor", Path: db}, func(event.UsageEvent) {}, func(event.TurnEvent) {}); err != nil {
		t.Fatal(err)
	}
	if hits != 0 {
		t.Fatalf("offline still hit API %d times", hits)
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

func TestRestrictRedirectAllowsCursorAPI(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "https://api2.cursor.sh/v1", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := (Adapter{}).restrictRedirect(req, nil); err != nil {
		t.Fatal(err)
	}
}

func TestParseFilteredKeepsSameMsDifferentConversations(t *testing.T) {
	raw := []byte(`{
	  "usageEventsDisplay": [
	    {"timestamp":"1700000000000","model":"grok-4","conversationId":"conv-a","tokenUsage":{"inputTokens":10,"outputTokens":1}},
	    {"timestamp":"1700000000000","model":"grok-4","conversationId":"conv-b","tokenUsage":{"inputTokens":20,"outputTokens":2}}
	  ]
	}`)
	evs, _, err := parseFiltered(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(evs) != 2 {
		t.Fatalf("events=%d", len(evs))
	}
	if evs[0].RequestID == evs[1].RequestID {
		t.Fatalf("same RequestID %q collapsed two conversations", evs[0].RequestID)
	}
	sum := metric.Aggregate(evs, nil)
	if sum.All.Total() != 33 {
		t.Fatalf("same-ms conversations collapsed: total=%d", sum.All.Total())
	}
}

func TestRestrictRedirectHonorsAPIBase(t *testing.T) {
	a := Adapter{APIBase: "https://cursor.test.example"}
	req, err := http.NewRequest(http.MethodGet, "https://cursor.test.example/v1", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.restrictRedirect(req, nil); err != nil {
		t.Fatal(err)
	}
}

func TestAllowedURLRejectsLookalikeHost(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "https://api2.cursor.sh.evil.example/v1", nil)
	if err != nil {
		t.Fatal(err)
	}
	if (Adapter{}).allowedURL(req.URL) {
		t.Fatal("suffix host must not match")
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
