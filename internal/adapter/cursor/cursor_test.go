package cursor

import (
	"database/sql"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/rainhuang0220/whereToken/internal/adapter"
	"github.com/rainhuang0220/whereToken/internal/adapter/testhome"
	"github.com/rainhuang0220/whereToken/internal/event"
)

func TestDiscoverCursorDotDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".cursor"), 0o755); err != nil {
		t.Fatal(err)
	}
	roots := Adapter{}.Discover(testhome.New(dir))
	if len(roots) != 1 || roots[0].ID != "cursor" {
		t.Fatalf("roots=%v", roots)
	}
}

func TestDiscoverLinuxXDGConfigAndWindowsAppData(t *testing.T) {
	dir := t.TempDir()
	linux := filepath.Join(dir, ".config", "Cursor", "User", "globalStorage")
	if err := os.MkdirAll(linux, 0o755); err != nil {
		t.Fatal(err)
	}
	linuxDB := filepath.Join(linux, "state.vscdb")
	if err := os.WriteFile(linuxDB, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	roots := Adapter{}.Discover(testhome.New(dir))
	if len(roots) != 1 || roots[0].Path != linuxDB {
		t.Fatalf("linux roots=%v", roots)
	}

	dir2 := t.TempDir()
	win := filepath.Join(dir2, "AppData", "Roaming", "Cursor", "User", "globalStorage")
	if err := os.MkdirAll(win, 0o755); err != nil {
		t.Fatal(err)
	}
	winDB := filepath.Join(win, "state.vscdb")
	if err := os.WriteFile(winDB, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	roots = Adapter{}.Discover(testhome.New(dir2))
	if len(roots) != 1 || roots[0].Path != winDB {
		t.Fatalf("windows roots=%v", roots)
	}
}

func TestDiscoverPrefersStateVscdb(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".cursor"), 0o755); err != nil {
		t.Fatal(err)
	}
	db := writeVscdb(t, dir, []kv{
		{key: "composerData:sess-a", value: `{"composerId":"sess-a","createdAt":1700000000000,"modelConfig":{"modelName":"claude-opus-4-6"},"usageData":{}}`},
	}, nil)
	roots := Adapter{}.Discover(testhome.New(dir))
	if len(roots) != 1 || roots[0].ID != "cursor" {
		t.Fatalf("roots=%v", roots)
	}
	if roots[0].Path != db {
		t.Fatalf("path=%q want %q", roots[0].Path, db)
	}
}

func TestParseMissingAuthReturnsChineseError(t *testing.T) {
	dir := t.TempDir()
	db := writeVscdb(t, dir, []kv{
		{key: "composerData:sess-a", value: `{"composerId":"sess-a","createdAt":1700000000000,"modelConfig":{"modelName":"claude-opus-4-6"},"usageData":{}}`},
		{key: "bubbleId:sess-a:u1", value: `{"type":1,"createdAt":"2026-02-09T14:44:05.860Z","tokenCount":{"inputTokens":0,"outputTokens":0}}`},
		{key: "bubbleId:sess-a:a1", value: `{"type":2,"createdAt":"2026-02-09T14:44:08.000Z","tokenCount":{"inputTokens":0,"outputTokens":0}}`},
	}, nil)

	err := (Adapter{}).Parse(adapter.SourceRoot{ID: "cursor", Path: db}, func(event.UsageEvent) {}, func(event.TurnEvent) {})
	if err == nil || !strings.Contains(err.Error(), "未找到本机登录态") {
		t.Fatalf("err=%v", err)
	}
	if strings.Contains(strings.ToLower(err.Error()), "bearer") || strings.Contains(err.Error(), "eyJ") {
		t.Fatal("error must not include credentials")
	}
}

func TestParseEmptyDirEmitsNothing(t *testing.T) {
	var n int
	err := (Adapter{}).Parse(adapter.SourceRoot{ID: "cursor", Path: t.TempDir()}, func(event.UsageEvent) {
		n++
	}, func(event.TurnEvent) {})
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("emitted %d events", n)
	}
}

func TestParseBubbleTokensAndTurns(t *testing.T) {
	dir := t.TempDir()
	db := writeVscdb(t, dir, []kv{
		{key: "composerData:sess-a", value: `{
			"composerId":"sess-a",
			"createdAt":1700000000000,
			"modelConfig":{"modelName":"claude-opus-4-6"},
			"usageData":{},
			"promptTokenBreakdown":{"totalUsedTokens":88888,"maxTokens":200000}
		}`},
		{key: "bubbleId:sess-a:u1", value: `{
			"type":1,
			"createdAt":"2026-02-09T14:44:05.860Z",
			"modelInfo":{"modelName":"MiniMax-M2.7"},
			"tokenCount":{"inputTokens":0,"outputTokens":0},
			"text":"PROMPT_BODY_MUST_NOT_BE_READ"
		}`},
		{key: "bubbleId:sess-a:think", value: `{
			"type":2,
			"capabilityType":30,
			"createdAt":"2026-02-09T14:44:06.000Z",
			"tokenCount":{"inputTokens":0,"outputTokens":0},
			"thinking":{"text":"REASONING_MUST_NOT_BE_READ"}
		}`},
		{key: "bubbleId:sess-a:tool", value: `{
			"type":2,
			"capabilityType":15,
			"createdAt":"2026-02-09T14:44:07.000Z",
			"tokenCount":{"inputTokens":0,"outputTokens":0},
			"toolFormerData":{"name":"read_file_v2","result":"TOOL_RESULT_MUST_NOT_BE_READ"}
		}`},
		{key: "bubbleId:sess-a:asst", value: `{
			"type":2,
			"createdAt":"2026-02-09T14:44:08.000Z",
			"tokenCount":{"inputTokens":100,"outputTokens":10},
			"text":"ASSISTANT_BODY_MUST_NOT_BE_READ"
		}`},
		{key: "composerData:sub-1", value: `{
			"composerId":"sub-1",
			"createdAt":1700000001000,
			"modelConfig":{"modelName":"gpt-5"},
			"usageData":{}
		}`},
		{key: "bubbleId:sub-1:u-sub", value: `{
			"type":1,
			"createdAt":"2026-02-09T15:00:00.000Z",
			"tokenCount":{"inputTokens":0,"outputTokens":0},
			"text":"SUBAGENT_PROMPT"
		}`},
		{key: "bubbleId:sub-1:a-sub", value: `{
			"type":2,
			"createdAt":"2026-02-09T15:00:01.000Z",
			"tokenCount":{"inputTokens":0,"outputTokens":0}
		}`},
	}, []header{
		{id: "sess-a", sub: 0, workspace: "/tmp/whereToken"},
		{id: "sub-1", sub: 1, workspace: "/tmp/whereToken"},
	})

	var evs []event.UsageEvent
	var turns []event.TurnEvent
	if err := (Adapter{}).Parse(adapter.SourceRoot{ID: "cursor", Path: db}, func(e event.UsageEvent) {
		evs = append(evs, e)
	}, func(te event.TurnEvent) {
		turns = append(turns, te)
	}); err != nil && !strings.Contains(err.Error(), "未找到本机登录态") {
		t.Fatal(err)
	}
	if len(turns) != 1 {
		t.Fatalf("user turns=%d want 1 (human only, not subagent)", len(turns))
	}
	if turns[0].Source != "cursor" || turns[0].SessionID != "sess-a" {
		t.Fatalf("turn %+v", turns[0])
	}
	if len(evs) != 3 {
		t.Fatalf("requests=%d want 3 (tool+asst+subagent asst; not thinking)", len(evs))
	}

	var asst, tool, sub *event.UsageEvent
	for i := range evs {
		e := &evs[i]
		if e.Source != "cursor" {
			t.Fatalf("source=%q", e.Source)
		}
		switch e.RequestID {
		case "sess-a:asst":
			asst = e
		case "sess-a:tool":
			tool = e
		case "sub-1:a-sub":
			sub = e
		default:
			t.Fatalf("unexpected request %q", e.RequestID)
		}
	}
	if asst == nil || tool == nil || sub == nil {
		t.Fatalf("missing events %+v", evs)
	}
	if asst.Miss != 100 || asst.Output != 10 || asst.CacheRead != 0 {
		t.Fatalf("asst tokens %+v", asst)
	}
	if asst.Quality != event.QualityAuthoritative {
		t.Fatalf("asst quality=%s", asst.Quality)
	}
	if asst.Vendor != "minimax" || asst.Model != "MiniMax-M2.7" {
		t.Fatalf("asst vendor/model %+v", asst)
	}
	if asst.Workspace != "/tmp/whereToken" {
		t.Fatalf("workspace=%q", asst.Workspace)
	}
	if asst.Timestamp.UTC().Format(time.RFC3339) != "2026-02-09T14:44:08Z" {
		t.Fatalf("asst ts=%s", asst.Timestamp.UTC())
	}
	if tool.Miss != 0 || tool.Output != 0 || tool.Quality != event.QualityDegraded {
		t.Fatalf("zero tokenCount must stay 0 and degraded: %+v", tool)
	}
	if tool.Vendor != "minimax" {
		t.Fatalf("tool follows user model: %+v", tool)
	}
	if sub.Vendor != "openai" || sub.Model != "gpt-5" {
		t.Fatalf("subagent composer model %+v", sub)
	}
	if sub.Quality != event.QualityDegraded {
		t.Fatalf("sub quality=%s", sub.Quality)
	}
}

func TestParseDoesNotCreditContextMeter(t *testing.T) {
	dir := t.TempDir()
	db := writeVscdb(t, dir, []kv{
		{key: "composerData:sess-b", value: `{
			"composerId":"sess-b",
			"createdAt":1700000000000,
			"modelConfig":{"modelName":"grok-4.5"},
			"contextTokensUsed":235611,
			"promptTokenBreakdown":{"totalUsedTokens":235611,"maxTokens":300000}
		}`},
		{key: "bubbleId:sess-b:u1", value: `{
			"type":1,
			"createdAt":"2026-08-15T12:00:00.000Z",
			"tokenCount":{"inputTokens":0,"outputTokens":0}
		}`},
		{key: "bubbleId:sess-b:a1", value: `{
			"type":2,
			"createdAt":"2026-08-15T12:00:01.000Z",
			"tokenCount":{"inputTokens":0,"outputTokens":0},
			"contextWindowStatusAtCreation":{"tokensUsed":180000,"tokenLimit":200000}
		}`},
	}, nil)

	var evs []event.UsageEvent
	if err := (Adapter{}).Parse(adapter.SourceRoot{ID: "cursor", Path: db}, func(e event.UsageEvent) {
		evs = append(evs, e)
	}, func(event.TurnEvent) {}); err != nil && !strings.Contains(err.Error(), "未找到本机登录态") {
		t.Fatal(err)
	}
	if len(evs) != 1 {
		t.Fatalf("events=%d", len(evs))
	}
	if evs[0].Miss != 0 || evs[0].Output != 0 || evs[0].CacheRead != 0 {
		t.Fatalf("must not treat context snapshots as billed tokens: %+v", evs[0])
	}
	if evs[0].Vendor != "unknown" {
		t.Fatalf("grok stays unknown vendor: %q", evs[0].Vendor)
	}
}

func TestProductionSQLIsPrefixOnly(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	src, err := os.ReadFile(filepath.Join(filepath.Dir(file), "cursor.go"))
	if err != nil {
		t.Fatal(err)
	}
	body := string(src)
	if strings.Contains(body, "SELECT * FROM cursorDiskKV") || strings.Contains(body, "select * from cursorDiskKV") {
		t.Fatal("must not SELECT * from cursorDiskKV")
	}
	if strings.Contains(body, "$.text") || strings.Contains(body, "`text`") {
		t.Fatal("must not extract bubble/composer text")
	}
	if !strings.Contains(body, "key LIKE 'composerData:%'") {
		t.Fatal("composerData must be prefix-queried")
	}
	if !strings.Contains(body, "key LIKE 'bubbleId:%'") {
		t.Fatal("bubbleId must be prefix-queried")
	}
	if strings.Contains(body, "SELECT value FROM cursorDiskKV") || strings.Contains(body, "SELECT key, value FROM cursorDiskKV") {
		t.Fatal("must not load cursorDiskKV value blobs")
	}
	if strings.Contains(body, "FROM cursorDiskKV") && !strings.Contains(body, "LIKE") {
		t.Fatal("cursorDiskKV query without LIKE")
	}
	apiSrc, err := os.ReadFile(filepath.Join(filepath.Dir(file), "api.go"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(apiSrc), "log.") {
		t.Fatal("must not log API traffic")
	}
}

type kv struct {
	key, value string
}

type header struct {
	id, workspace string
	sub           int
}

func writeVscdb(t *testing.T, home string, rows []kv, headers []header) string {
	t.Helper()
	dir := filepath.Join(home, "Library", "Application Support", "Cursor", "User", "globalStorage")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "state.vscdb")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE cursorDiskKV (key TEXT UNIQUE ON CONFLICT REPLACE, value BLOB)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE composerHeaders (
		composerId TEXT PRIMARY KEY, workspaceId TEXT, createdAt INTEGER, lastUpdatedAt INTEGER,
		isArchived INTEGER, isSubagent INTEGER, recency INTEGER, checkpointAt INTEGER, value TEXT
	)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE ItemTable (key TEXT UNIQUE ON CONFLICT REPLACE, value BLOB)`); err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		if _, err := db.Exec(`INSERT INTO cursorDiskKV (key, value) VALUES (?, ?)`, row.key, row.value); err != nil {
			t.Fatal(err)
		}
	}
	for _, h := range headers {
		val := `{"workspaceIdentifier":{"uri":{"fsPath":"` + h.workspace + `"}}}`
		if _, err := db.Exec(`INSERT INTO composerHeaders (composerId, isSubagent, value) VALUES (?, ?, ?)`, h.id, h.sub, val); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}
