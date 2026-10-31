package cursor

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rainhuang0220/whereToken/internal/adapter"
	"github.com/rainhuang0220/whereToken/internal/adapter/testhome"
	"github.com/rainhuang0220/whereToken/internal/event"
	"github.com/rainhuang0220/whereToken/internal/metric"
	"github.com/rainhuang0220/whereToken/internal/report"
)

func TestUnreadableSessionNeverCallsTheAccountAPI(t *testing.T) {
	cases := []string{
		"cursor-safe-storage:v1:QUJDRA",
		"cursor-safe-storage:v2:QUJDRA",
		"AQAAANCMnd8BFdERjHoAwE/Cl+sBAAAA",
		"not-a-session",
	}
	for _, token := range cases {
		t.Run(token[:12], func(t *testing.T) {
			hits := 0
			srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				hits++
			}))
			t.Cleanup(srv.Close)
			dir := t.TempDir()
			db := writeVscdb(t, dir, []kv{
				{key: "bubbleId:sess-a:a1", value: `{"type":2,"createdAt":"2026-02-09T14:44:08.000Z","tokenCount":{"inputTokens":40,"outputTokens":2}}`},
			}, nil)
			putItem(t, db, authAccessTokenKey, token)
			putItem(t, db, authRefreshTokenKey, token)

			var evs []event.UsageEvent
			err := (Adapter{HTTP: srv.Client(), APIBase: srv.URL}).Parse(adapter.SourceRoot{ID: "cursor", Path: db}, func(e event.UsageEvent) {
				evs = append(evs, e)
			}, func(event.TurnEvent) {})
			if !strings.Contains(err.Error(), "账号用量不可用") {
				t.Fatalf("err=%v", err)
			}
			if strings.Contains(strings.ToLower(err.Error()), "auth") || strings.Contains(err.Error(), "未找到本机登录态") {
				t.Fatalf("error would be labeled as a missing login: %v", err)
			}
			if strings.Contains(err.Error(), token) || strings.Contains(err.Error(), "eyJ") || strings.Contains(strings.ToLower(err.Error()), "bearer") {
				t.Fatalf("error leaked session material: %v", err)
			}
			if hits != 0 {
				t.Fatalf("unreadable session made %d HTTP calls", hits)
			}
			if len(evs) != 1 || evs[0].Miss != 40 || evs[0].Output != 2 || evs[0].Quality != event.QualityDegraded {
				t.Fatalf("local tokenCount=%+v", evs)
			}
			if evs[0].Derivation == event.DeriveProviderAPI {
				t.Fatal("local row was labeled as an account measurement")
			}
		})
	}
}

func TestSealedOfflineStillReportsAccountUsageUnavailable(t *testing.T) {
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		hits++
	}))
	t.Cleanup(srv.Close)
	dir := t.TempDir()
	db := writeVscdb(t, dir, nil, nil)
	putItem(t, db, authAccessTokenKey, "cursor-safe-storage:v1:QUJDRA")
	err := (Adapter{HTTP: srv.Client(), APIBase: srv.URL, Offline: true}).Parse(adapter.SourceRoot{ID: "cursor", Path: db}, func(event.UsageEvent) {}, func(event.TurnEvent) {})
	if err == nil || !strings.Contains(err.Error(), "账号用量不可用") {
		t.Fatalf("err=%v", err)
	}
	if hits != 0 {
		t.Fatalf("offline sealed session called the API %d times", hits)
	}
}

func TestStorageJSONSealIsTheSameRefusal(t *testing.T) {
	dir := t.TempDir()
	db := writeVscdb(t, dir, nil, nil)
	raw, err := json.Marshal(map[string]string{authAccessTokenKey: "cursor-safe-storage:v1:QUJDRA"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(filepath.Dir(db), "storage.json"), raw, 0o600); err != nil {
		t.Fatal(err)
	}
	err = (Adapter{Offline: true}).Parse(adapter.SourceRoot{ID: "cursor", Path: db}, func(event.UsageEvent) {}, func(event.TurnEvent) {})
	if err == nil || !strings.Contains(err.Error(), "账号用量不可用") {
		t.Fatalf("err=%v", err)
	}
}

func TestQuotedPlaintextSessionStillReadsAccountUsage(t *testing.T) {
	cases := []string{
		`"` + fakeJWT + `"`,
		"\"user_1::" + fakeJWT + "\"\n",
	}
	for _, stored := range cases {
		t.Run(stored[:8], func(t *testing.T) {
			var bearer string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				bearer = r.Header.Get("Authorization")
				if strings.Contains(r.URL.Path, "GetFilteredUsageEvents") {
					io.WriteString(w, `{"totalUsageEventsCount":1,"usageEventsDisplay":[{"timestamp":"1770000000000","model":"gpt-5","tokenUsage":{"inputTokens":4,"outputTokens":1}}]}`)
					return
				}
				http.NotFound(w, r)
			}))
			t.Cleanup(srv.Close)
			dir := t.TempDir()
			db := writeVscdb(t, dir, nil, nil)
			putItem(t, db, authAccessTokenKey, stored)
			var evs []event.UsageEvent
			err := (Adapter{HTTP: srv.Client(), APIBase: srv.URL}).Parse(adapter.SourceRoot{ID: "cursor", Path: db}, func(e event.UsageEvent) {
				evs = append(evs, e)
			}, func(event.TurnEvent) {})
			if err != nil {
				t.Fatal(err)
			}
			if !strings.HasPrefix(bearer, "Bearer ") || strings.Contains(bearer, `"`) || !strings.Contains(bearer, fakeJWT) {
				t.Fatalf("bearer=%q", bearer)
			}
			if metric.Aggregate(evs, nil).All.Total() != 5 {
				t.Fatalf("events=%+v", evs)
			}
		})
	}
}

func TestQuotedSealStaysUnread(t *testing.T) {
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		hits++
	}))
	t.Cleanup(srv.Close)
	dir := t.TempDir()
	db := writeVscdb(t, dir, nil, nil)
	putItem(t, db, authAccessTokenKey, `"cursor-safe-storage:v1:QUJDRA"`)
	err := (Adapter{HTTP: srv.Client(), APIBase: srv.URL}).Parse(adapter.SourceRoot{ID: "cursor", Path: db}, func(event.UsageEvent) {}, func(event.TurnEvent) {})
	if err == nil || !strings.Contains(err.Error(), "账号用量不可用") {
		t.Fatalf("err=%v", err)
	}
	if hits != 0 {
		t.Fatalf("quoted seal made %d HTTP calls", hits)
	}
}

func TestTwoPartSessionIsNotABearer(t *testing.T) {
	if usableSessionToken("eyJhbGciOiJub25lIn0.eyJzdWIiOiJ4In0") {
		t.Fatal("a two-part token was treated as a session JWT")
	}
	if usableSessionToken("e30.eyJzdWIiOiJ4In0.sig") {
		t.Fatal("a non-object header was treated as a session JWT")
	}
	if !usableSessionToken("  user_01::" + fakeJWT + "  ") {
		t.Fatal("a WorkOS-prefixed JWT was rejected")
	}
}

func TestPlaintextSessionStillReadsAccountUsage(t *testing.T) {
	var bearer string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bearer = r.Header.Get("Authorization")
		if strings.Contains(r.URL.Path, "GetFilteredUsageEvents") {
			io.WriteString(w, `{"totalUsageEventsCount":1,"usageEventsDisplay":[{"timestamp":"1770000000000","model":"gpt-5","tokenUsage":{"inputTokens":9,"outputTokens":1}}]}`)
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)
	dir := t.TempDir()
	db := writeVscdb(t, dir, []kv{
		{key: "bubbleId:sess-a:a1", value: `{"type":2,"createdAt":"2026-02-09T14:44:08.000Z","tokenCount":{"inputTokens":100,"outputTokens":10}}`},
	}, nil)
	session := "user_1::" + fakeJWT
	putItem(t, db, authAccessTokenKey, session)

	var evs []event.UsageEvent
	err := (Adapter{HTTP: srv.Client(), APIBase: srv.URL}).Parse(adapter.SourceRoot{ID: "cursor", Path: db}, func(e event.UsageEvent) {
		evs = append(evs, e)
	}, func(event.TurnEvent) {})
	if err != nil {
		t.Fatal(err)
	}
	if bearer != "Bearer "+session {
		t.Fatalf("bearer=%q", bearer)
	}
	sum := metric.Aggregate(evs, nil)
	if sum.All.Miss != 9 || sum.All.Output != 1 {
		t.Fatalf("account tokens were replaced or doubled: %+v", sum.All)
	}
}

func TestSameJWTIsNotPostedAsRefresh(t *testing.T) {
	oauth := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "oauth/token") {
			oauth++
			io.WriteString(w, `{"access_token":"`+fakeJWT+`"}`)
			return
		}
		http.Error(w, "no", http.StatusUnauthorized)
	}))
	t.Cleanup(srv.Close)
	dir := t.TempDir()
	db := writeVscdb(t, dir, nil, nil)
	putItem(t, db, authAccessTokenKey, fakeJWT)
	putItem(t, db, authRefreshTokenKey, fakeJWT)
	err := (Adapter{HTTP: srv.Client(), APIBase: srv.URL}).Parse(adapter.SourceRoot{ID: "cursor", Path: db}, func(event.UsageEvent) {}, func(event.TurnEvent) {})
	if err == nil || !strings.Contains(err.Error(), "已失效") {
		t.Fatalf("err=%v", err)
	}
	if oauth != 0 {
		t.Fatalf("identical access and refresh were posted to oauth %d times", oauth)
	}
}

func TestDistinctSessionJWTIsNotPosted(t *testing.T) {
	session := testJWT(t, `{"sub":"other","iss":"https://authentication.cursor.sh","aud":"https://cursor.com","type":"session"}`)
	plain := testJWT(t, `{"sub":"other"}`)
	for _, refresh := range []string{session, "user_01::" + session, plain} {
		oauth := 0
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.Contains(r.URL.Path, "oauth/token") {
				oauth++
			}
			http.Error(w, "no", http.StatusUnauthorized)
		}))
		dir := t.TempDir()
		db := writeVscdb(t, dir, nil, nil)
		putItem(t, db, authAccessTokenKey, fakeJWT)
		putItem(t, db, authRefreshTokenKey, refresh)
		err := (Adapter{HTTP: srv.Client(), APIBase: srv.URL}).Parse(adapter.SourceRoot{ID: "cursor", Path: db}, func(event.UsageEvent) {}, func(event.TurnEvent) {})
		srv.Close()
		if err == nil || !strings.Contains(err.Error(), "已失效") {
			t.Fatalf("refresh kind err=%v", err)
		}
		if oauth != 0 {
			t.Fatalf("a non-refresh JWT was posted %d times", oauth)
		}
	}
}

func TestDeclaredRefreshTokenIsPosted(t *testing.T) {
	refreshJWT := "user_01::" + testJWT(t, `{"sub":"refresh","type":"refresh_token"}`)
	var posted string
	filtered := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "oauth/token"):
			var body map[string]string
			_ = json.NewDecoder(r.Body).Decode(&body)
			posted = body["grant_type"] + " " + body["refresh_token"]
			io.WriteString(w, `{"access_token":"`+fakeJWT+`"}`)
		case strings.Contains(r.URL.Path, "GetFilteredUsageEvents"):
			filtered++
			if filtered == 1 {
				http.Error(w, "no", http.StatusUnauthorized)
				return
			}
			io.WriteString(w, `{"totalUsageEventsCount":1,"usageEventsDisplay":[{"timestamp":"1770000000000","model":"gpt-5","tokenUsage":{"inputTokens":3,"outputTokens":1}}]}`)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	dir := t.TempDir()
	db := writeVscdb(t, dir, nil, nil)
	putItem(t, db, authAccessTokenKey, fakeJWT)
	putItem(t, db, authRefreshTokenKey, refreshJWT)
	var evs []event.UsageEvent
	err := (Adapter{HTTP: srv.Client(), APIBase: srv.URL}).Parse(adapter.SourceRoot{ID: "cursor", Path: db}, func(e event.UsageEvent) {
		evs = append(evs, e)
	}, func(event.TurnEvent) {})
	if err != nil {
		t.Fatal(err)
	}
	if posted != "refresh_token "+refreshJWT {
		t.Fatal("declared refresh token was not posted with the refresh grant")
	}
	if metric.Aggregate(evs, nil).All.Total() != 4 {
		t.Fatalf("events=%+v", evs)
	}
}

func testJWT(t *testing.T, payload string) string {
	t.Helper()
	return "eyJhbGciOiJub25lIn0." + base64.RawURLEncoding.EncodeToString([]byte(payload)) + ".sig"
}

func TestSealedRefreshIsNotPosted(t *testing.T) {
	oauth := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "oauth/token") {
			oauth++
		}
		http.Error(w, "no", http.StatusUnauthorized)
	}))
	t.Cleanup(srv.Close)
	dir := t.TempDir()
	db := writeVscdb(t, dir, nil, nil)
	putItem(t, db, authAccessTokenKey, fakeJWT)
	putItem(t, db, authRefreshTokenKey, "cursor-safe-storage:v1:QUJDRA")
	err := (Adapter{HTTP: srv.Client(), APIBase: srv.URL}).Parse(adapter.SourceRoot{ID: "cursor", Path: db}, func(event.UsageEvent) {}, func(event.TurnEvent) {})
	if err == nil {
		t.Fatal("expected unauthorized")
	}
	if oauth != 0 {
		t.Fatalf("sealed refresh was posted %d times", oauth)
	}
}

func TestAccountSubjectIgnoresSealedSession(t *testing.T) {
	dir := t.TempDir()
	db := writeVscdb(t, dir, nil, nil)
	putItem(t, db, authAccessTokenKey, "cursor-safe-storage:v1:QUJDRA")
	if got := (Adapter{}).AccountSubject(testhome.New(dir)); got != "" {
		t.Fatalf("subject=%q", got)
	}
	putItem(t, db, authAccessTokenKey, fakeJWT)
	if got := (Adapter{}).AccountSubject(testhome.New(dir)); got != "test-user" {
		t.Fatalf("subject=%q", got)
	}
}

func TestMissingTokenCountRendersUnavailable(t *testing.T) {
	dir := t.TempDir()
	db := writeVscdb(t, dir, []kv{
		{key: "bubbleId:sess-a:u1", value: `{"type":1,"createdAt":"2026-10-31T12:00:00Z","text":"PROMPT_MUST_NOT_APPEAR"}`},
		{key: "bubbleId:sess-a:a1", value: `{"type":2,"createdAt":"2026-10-31T12:00:01Z","text":"ASSISTANT_MUST_NOT_APPEAR"}`},
	}, nil)
	putItem(t, db, authAccessTokenKey, "cursor-safe-storage:v1:QUJDRA")
	var evs []event.UsageEvent
	var turns []event.TurnEvent
	err := (Adapter{}).Parse(adapter.SourceRoot{ID: "cursor", Path: db}, func(e event.UsageEvent) {
		evs = append(evs, e)
	}, func(te event.TurnEvent) {
		turns = append(turns, te)
	})
	if err == nil {
		t.Fatal("expected unavailable")
	}
	if len(evs) != 1 || evs[0].Miss != 0 || evs[0].Output != 0 || evs[0].Derivation == event.DeriveProviderAPI {
		t.Fatalf("events=%+v", evs)
	}
	now := time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)
	snap, berr := report.Build(evs, turns, []string{"cursor: " + err.Error()}, report.Filter{Tool: "cursor"}, now, time.UTC)
	if berr != nil {
		t.Fatal(berr)
	}
	out := report.Render(snap, report.Options{ASCII: true, Width: 80})
	for _, want := range []string{"账号用量不可用", "不可用", "没有可读数字"} {
		if !strings.Contains(out, want) {
			t.Fatalf("report missing %q:\n%s", want, out)
		}
	}
	for _, bad := range []string{"0.00 M", "钥匙串", "keychain", "PROMPT_MUST_NOT_APPEAR", "ASSISTANT_MUST_NOT_APPEAR", "QUJDRA", "eyJ", "需要已登录"} {
		if strings.Contains(out, bad) {
			t.Fatalf("report contains %q:\n%s", bad, out)
		}
	}
}

func TestShortFilteredPageIsNotAMeasuredTotal(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Page int `json:"page"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if strings.Contains(r.URL.Path, "GetAggregatedUsageEvents") {
			http.Error(w, "down", http.StatusBadGateway)
			return
		}
		if body.Page <= 1 {
			io.WriteString(w, `{"totalUsageEventsCount":5,"usageEventsDisplay":[{"timestamp":"1770000000000","model":"gpt-5","tokenUsage":{"inputTokens":100}}]}`)
			return
		}
		io.WriteString(w, `{"totalUsageEventsCount":5,"usageEventsDisplay":[]}`)
	}))
	t.Cleanup(srv.Close)
	evs, err := (Adapter{HTTP: srv.Client(), APIBase: srv.URL}).fetchUsageWithToken("root", fakeJWT)
	if err == nil || len(evs) != 0 {
		t.Fatalf("err=%v events=%d", err, len(evs))
	}
}

func TestMissingDeclaredTotalIsNotAMeasuredTotal(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "GetAggregatedUsageEvents") {
			io.WriteString(w, `{"aggregations":[]}`)
			return
		}
		io.WriteString(w, `{"usageEventsDisplay":[{"timestamp":"1770000000000","model":"gpt-5","tokenUsage":{"inputTokens":100}}]}`)
	}))
	t.Cleanup(srv.Close)
	evs, err := (Adapter{HTTP: srv.Client(), APIBase: srv.URL}).fetchUsageWithToken("root", fakeJWT)
	if err == nil || len(evs) != 0 {
		t.Fatalf("err=%v events=%+v", err, evs)
	}
	for _, e := range evs {
		if e.RequestID == "cursor-api:measured-zero" || e.Derivation == event.DeriveProviderAPI {
			t.Fatalf("incomplete page became an account row: %+v", e)
		}
	}
}

func TestRateLimitPublishesNothing(t *testing.T) {
	agg := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "GetAggregatedUsageEvents") {
			agg++
			io.WriteString(w, `{"aggregations":[{"modelIntent":"gpt-5","inputTokens":9,"outputTokens":1}]}`)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		http.Error(w, `{"secret":"nope"}`, http.StatusTooManyRequests)
	}))
	t.Cleanup(srv.Close)
	evs, err := (Adapter{HTTP: srv.Client(), APIBase: srv.URL}).fetchUsageWithToken("root", fakeJWT)
	if err == nil || len(evs) != 0 || agg != 0 {
		t.Fatalf("err=%v events=%d agg=%d", err, len(evs), agg)
	}
	if strings.Contains(err.Error(), "secret") || strings.Contains(err.Error(), "nope") {
		t.Fatalf("body leaked: %v", err)
	}
}

func TestDuplicateRowsInsideDeclaredTotalStayComplete(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{
		  "totalUsageEventsCount": 2,
		  "usageEventsDisplay": [
		    {"timestamp":"1770000000000","model":"gpt-5","conversationId":"c","tokenUsage":{"inputTokens":10,"outputTokens":1}},
		    {"timestamp":"1770000000000","model":"gpt-5","conversationId":"c","tokenUsage":{"inputTokens":10,"outputTokens":1}}
		  ]
		}`)
	}))
	t.Cleanup(srv.Close)
	evs, err := (Adapter{HTTP: srv.Client(), APIBase: srv.URL}).fetchFiltered(fakeJWT, 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(evs) != 2 || metric.Aggregate(evs, nil).All.Total() != 22 {
		t.Fatalf("events=%+v", evs)
	}
}

func TestRowsBeyondDeclaredTotalAreRejected(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{
		  "totalUsageEventsCount": 1,
		  "usageEventsDisplay": [
		    {"timestamp":"1770000000000","model":"gpt-5","tokenUsage":{"inputTokens":10}},
		    {"timestamp":"1770000000001","model":"gpt-5","tokenUsage":{"inputTokens":10}}
		  ]
		}`)
	}))
	t.Cleanup(srv.Close)
	evs, err := (Adapter{HTTP: srv.Client(), APIBase: srv.URL}).fetchFiltered(fakeJWT, 1, 2)
	if err == nil || len(evs) != 0 {
		t.Fatalf("err=%v events=%d", err, len(evs))
	}
}

func TestAdapterSourceDoesNotTouchOSSecretStores(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	banned := []string{
		"find-generic-password",
		"set-key-partition-list",
		"SecItemCopyMatching",
		"CryptUnprotectData",
		"secret-tool",
		"DPAPI",
		"PlainList",
		"cursor-bridge",
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		body, err := os.ReadFile(e.Name())
		if err != nil {
			t.Fatal(err)
		}
		for _, bad := range banned {
			if strings.Contains(string(body), bad) {
				t.Fatalf("%s contains %s", e.Name(), bad)
			}
		}
	}
}

// TestReleaseSurfaceLeavesCursorSecretsOut checks the files a release
// packages. internal/credstore is the product login store for service
// whereToken. It is outside this scan, and it must not name the Cursor item.
func TestReleaseSurfaceLeavesCursorSecretsOut(t *testing.T) {
	root := moduleRoot(t)
	banned := []string{
		"cursor-bridge",
		"PlainList",
		"find-generic-password",
		"set-key-partition-list",
		"SecItemCopyMatching",
		"CryptUnprotectData",
		"secret-tool",
		"Cursor Safe Storage",
		"/Users/rainhuang",
		"/private/tmp",
		"/tmp/wt-",
	}
	var files []string
	walk := func(rel string) {
		t.Helper()
		base := filepath.Join(root, rel)
		err := filepath.WalkDir(base, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				if os.IsNotExist(err) {
					return nil
				}
				return err
			}
			if d.IsDir() {
				switch d.Name() {
				case "credstore", "testdata", "node_modules":
					return filepath.SkipDir
				}
				return nil
			}
			name := d.Name()
			if strings.HasSuffix(name, "_test.go") || strings.HasSuffix(name, ".test.js") {
				return nil
			}
			files = append(files, path)
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	for _, rel := range []string{"cmd", "internal", "completions", "npm", "Formula"} {
		walk(rel)
	}
	for _, rel := range []string{
		"scripts/install.sh",
		"scripts/install.ps1",
		"scripts/install.cmd",
		"scripts/package-release.sh",
		".goreleaser.yaml",
	} {
		files = append(files, filepath.Join(root, rel))
	}
	if len(files) == 0 {
		t.Fatal("release surface was empty")
	}
	for _, path := range files {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		text := string(body)
		for _, bad := range banned {
			if strings.Contains(text, bad) {
				t.Fatalf("%s contains %s", path, bad)
			}
		}
	}
	cred, err := os.ReadFile(filepath.Join(root, "internal", "credstore", "keychain.go"))
	if err != nil {
		t.Fatal(err)
	}
	credText := string(cred)
	if !strings.Contains(credText, "whereToken") {
		t.Fatal("credstore is no longer the product login store")
	}
	for _, bad := range []string{"Cursor Safe Storage", "cursor-bridge", "PlainList"} {
		if strings.Contains(credText, bad) {
			t.Fatalf("credstore contains %s", bad)
		}
	}
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		next := filepath.Dir(dir)
		if next == dir {
			t.Fatal("go.mod not found")
		}
		dir = next
	}
}
