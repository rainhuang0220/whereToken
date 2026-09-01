package scan

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/rainhuang0220/whereToken/internal/adapter/cursor"
	"github.com/rainhuang0220/whereToken/internal/adapter/testhome"
	"github.com/rainhuang0220/whereToken/internal/adapter/trae"
	"github.com/rainhuang0220/whereToken/internal/community"
	"github.com/rainhuang0220/whereToken/internal/event"
	"github.com/rainhuang0220/whereToken/internal/metric"
)

func TestProgressDisplayLabelASCII(t *testing.T) {
	p := Progress{Label: readingLabel("kimi")}
	if !strings.Contains(p.Label, "…") {
		t.Fatalf("label=%q", p.Label)
	}
	if got := p.DisplayLabel(true); strings.Contains(got, "…") || !strings.Contains(got, "...") {
		t.Fatalf("ascii label=%q", got)
	}
}

func TestCloudSkipped(t *testing.T) {
	if CloudSkipped(Adapters(false)) {
		t.Fatal("online adapters")
	}
	if !CloudSkipped(Adapters(true)) {
		t.Fatal("offline adapters should skip cloud")
	}
}

func TestAdaptersOfflineFlagsCloudSources(t *testing.T) {
	off := Adapters(true)
	var cursorOff, traeOff bool
	for _, a := range off {
		if c, ok := a.(cursor.Adapter); ok {
			cursorOff = c.Offline
		}
		if tr, ok := a.(trae.Adapter); ok {
			traeOff = tr.Offline
		}
	}
	if !cursorOff || !traeOff {
		t.Fatalf("cursor=%v trae=%v", cursorOff, traeOff)
	}
	on := Adapters(false)
	for _, a := range on {
		if c, ok := a.(cursor.Adapter); ok && c.Offline {
			t.Fatal("online cursor marked offline")
		}
	}
}

func TestMarshalSummaryMarksOffline(t *testing.T) {
	raw, err := MarshalSummary(Result{Offline: true, Errors: []string{}})
	if err != nil {
		t.Fatal(err)
	}
	var payload struct {
		Offline bool `json:"offline"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	if !payload.Offline {
		t.Fatalf("offline scan JSON must say so: %s", raw)
	}
}

func TestMarshalSummaryMarksScanning(t *testing.T) {
	raw, err := MarshalSummary(Result{Scanning: true, Errors: []string{}})
	if err != nil {
		t.Fatal(err)
	}
	var payload struct {
		Scanning bool `json:"scanning"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	if !payload.Scanning {
		t.Fatalf("in-flight summary must say scanning: %s", raw)
	}
}

func TestMarshalSummaryOmitsOfflineWhenOnline(t *testing.T) {
	raw, err := MarshalSummary(Result{Errors: []string{}})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), `"offline": true`) {
		t.Fatalf("online scan should not claim offline: %s", raw)
	}
}

func TestMarshalSummaryOmitsRawEvents(t *testing.T) {
	t.Setenv("WHERETOKEN_EXTRA_ROOTS", "")
	r := Run(testhome.New(t.TempDir()), AllAdapters())
	raw, err := MarshalSummary(r)
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	if strings.Contains(s, `"events"`) || strings.Contains(s, `"turns"`) {
		t.Fatalf("raw events leaked into observatory JSON")
	}
}

func TestApplyWindowClonesCommunity(t *testing.T) {
	view := community.EmptyView(community.StatusOK, community.DisclaimerEN)
	view.Today.Display = "#37 / 842"
	r := Result{Community: &view, Errors: []string{}}
	win := metric.Window{Today: true}
	got := ApplyWindow(r, win, time.UTC)
	if got.Community == nil || got.Community == r.Community {
		t.Fatal("window must clone community")
	}
	got.Community.Today.Display = "mutated"
	if r.Community.Today.Display != "#37 / 842" {
		t.Fatal("window must not mutate the full-scan standing")
	}
}

func TestWindowedSummaryDoesNotPasteTodayRankOntoInsights(t *testing.T) {
	view := community.EmptyView(community.StatusOK, community.DisclaimerEN)
	view.Today.Rank = 37
	view.Today.Display = "#37 / 842"
	r := Result{
		Errors:    []string{},
		Community: &view,
		Compare:   &metric.Compare{},
	}
	raw, err := MarshalSummary(r)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "社区排名") {
		t.Fatalf("7d/today window must not reuse today's podium:\n%s", raw)
	}
}

func TestAllTimeSummaryAttachesRealRankInsight(t *testing.T) {
	view := community.EmptyView(community.StatusOK, community.DisclaimerEN)
	view.Today.Rank = 1
	view.Today.Display = "#1 / 20"
	view.All.Rank = 37
	view.All.Display = "#37 / 842"
	r := Result{Errors: []string{}, Community: &view}
	raw, err := MarshalSummary(r)
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	if !strings.Contains(s, "社区排名 #37 / 842") || !strings.Contains(s, "不是全球榜") {
		t.Fatalf("%s", s)
	}
	if strings.Contains(s, "社区排名 #1 / 20") {
		t.Fatal("all-time 用量说明 must not paste today's podium")
	}
	if strings.Contains(s, "#0") {
		t.Fatal(s)
	}
}

func TestMarshalSummaryTinyPodiumDoesNotBecomeInsight(t *testing.T) {
	view := community.EmptyView(community.StatusOK, community.DisclaimerEN)
	view.All.Rank = 1
	view.All.Display = "#1 / 3"
	view.All.Participants = 3
	raw, err := MarshalSummary(Result{Community: &view, Errors: []string{}})
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	if strings.Contains(s, "社区排名 #1 / 3") || strings.Contains(s, "#1 / 3") {
		t.Fatalf("all-time 用量说明 must not print a 3-person podium:\n%s", s)
	}
}

func TestMarshalSummaryCommunityInsightNeverZeroRank(t *testing.T) {
	view := community.EmptyView(community.StatusOK, community.DisclaimerEN)
	view.Today.Status = community.StatusOK
	view.Today.Rank = 0
	view.Today.Display = "#0 / 20"
	raw, err := MarshalSummary(Result{Community: &view, Errors: []string{}})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "#0") {
		t.Fatalf("observatory must not print #0: %s", raw)
	}

	view.All.Rank = 37
	view.All.Display = "#37 / 842"
	raw, err = MarshalSummary(Result{Community: &view, Errors: []string{}})
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	if !strings.Contains(s, "#37 / 842") || !strings.Contains(s, "不是全球榜") {
		t.Fatalf("missing real standing insight: %s", s)
	}
	if strings.Contains(s, `"events"`) || strings.Contains(s, "prompt") {
		t.Fatal("raw payload leaked")
	}
}

func TestRunKimiFixture(t *testing.T) {
	t.Setenv("WHERETOKEN_EXTRA_ROOTS", "")
	dir := t.TempDir()
	dstDir := filepath.Join(dir, ".kimi-code", "sessions", "x", "s", "agents", "main")
	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		t.Fatal(err)
	}
	_, file, _, _ := runtime.Caller(0)
	src := filepath.Join(filepath.Dir(file), "..", "..", "testdata", "adapters", "kimi", "session", "agents", "main", "wire.jsonl")
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dstDir, "wire.jsonl"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	r := Run(testhome.New(dir), AllAdapters())
	if r.Summary.All.Total() != 1185 {
		t.Fatalf("all=%d", r.Summary.All.Total())
	}
	if len(r.Events) == 0 {
		t.Fatal("expected events on Result for CLI filters")
	}
	var kimi, moon int64
	for _, s := range r.Summary.BySource {
		if s.ID == "kimi" {
			kimi = s.Total()
		}
	}
	for _, s := range r.Summary.ByVendor {
		if s.ID == "moonshot" {
			moon = s.Total()
		}
	}
	if kimi != r.Summary.All.Total() || moon != r.Summary.All.Total() {
		t.Fatalf("kimi=%d moon=%d all=%d", kimi, moon, r.Summary.All.Total())
	}
	var modelSum int64
	for _, m := range r.Summary.DrillAll.Models {
		modelSum += m.Total()
	}
	if modelSum != r.Summary.All.Total() {
		t.Fatalf("drill models=%d all=%d", modelSum, r.Summary.All.Total())
	}
}

func TestRunGrokFixture(t *testing.T) {
	t.Setenv("WHERETOKEN_EXTRA_ROOTS", "")
	dir := t.TempDir()
	dstDir := filepath.Join(dir, ".grok", "sessions", "%2Ftmp%2Fdemo", "s1")
	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		t.Fatal(err)
	}
	_, file, _, _ := runtime.Caller(0)
	src := filepath.Join(filepath.Dir(file), "..", "..", "testdata", "adapters", "grok", "sessions", "%2Ftmp%2Fdemo", "s1", "updates.jsonl")
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dstDir, "updates.jsonl"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	r := Run(testhome.New(dir), AllAdapters())
	if r.Summary.All.Total() != 185 {
		t.Fatalf("all=%d", r.Summary.All.Total())
	}
	var grokTotal, xai int64
	for _, s := range r.Summary.BySource {
		if s.ID == "grok" {
			grokTotal = s.Total()
			if s.Label != "Grok" {
				t.Fatalf("label=%q", s.Label)
			}
		}
	}
	for _, s := range r.Summary.ByVendor {
		if s.ID == "xai" {
			xai = s.Total()
		}
	}
	if grokTotal != 185 || xai != 185 {
		t.Fatalf("grok=%d xai=%d", grokTotal, xai)
	}
}

func TestRunDoesNotDoubleExtraRootSymlink(t *testing.T) {
	dir := t.TempDir()
	dstDir := filepath.Join(dir, ".kimi-code", "sessions", "x", "s", "agents", "main")
	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		t.Fatal(err)
	}
	_, file, _, _ := runtime.Caller(0)
	src := filepath.Join(filepath.Dir(file), "..", "..", "testdata", "adapters", "kimi", "session", "agents", "main", "wire.jsonl")
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dstDir, "wire.jsonl"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	alias := dir + "-alias"
	if err := os.Symlink(dir, alias); err != nil {
		t.Skipf("symlink: %v", err)
	}
	t.Setenv("WHERETOKEN_EXTRA_ROOTS", alias)
	r := Run(testhome.New(dir), AllAdapters())
	if r.Summary.All.Total() != 1185 {
		t.Fatalf("extra-root symlink double-counted: all=%d", r.Summary.All.Total())
	}
}

func TestRunMarksCursorAbsent(t *testing.T) {
	t.Setenv("WHERETOKEN_EXTRA_ROOTS", "")
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".cursor"), 0o755); err != nil {
		t.Fatal(err)
	}
	r := Run(testhome.New(dir), AllAdapters())
	var cursor *metric.Slice
	for i := range r.Summary.BySource {
		if r.Summary.BySource[i].ID == "cursor" {
			cursor = &r.Summary.BySource[i]
			break
		}
	}
	if cursor == nil {
		t.Fatal("expected cursor row")
	}
	if cursor.Quality != event.QualityAbsent || cursor.Total() != 0 {
		t.Fatalf("cursor %+v", cursor)
	}
	if r.Summary.All.Total() != 0 {
		t.Fatalf("all=%d", r.Summary.All.Total())
	}
}

func TestRunCursorVscdbConservation(t *testing.T) {
	t.Setenv("WHERETOKEN_EXTRA_ROOTS", "")
	dir := t.TempDir()
	gs := filepath.Join(dir, "Library", "Application Support", "Cursor", "User", "globalStorage")
	if err := os.MkdirAll(gs, 0o755); err != nil {
		t.Fatal(err)
	}
	dbPath := filepath.Join(gs, "state.vscdb")
	writeScanCursorDB(t, dbPath)

	r := Run(testhome.New(dir), AllAdapters())
	var cursor, anthropic *metric.Slice
	for i := range r.Summary.BySource {
		if r.Summary.BySource[i].ID == "cursor" {
			cursor = &r.Summary.BySource[i]
		}
	}
	for i := range r.Summary.ByVendor {
		if r.Summary.ByVendor[i].ID == "anthropic" {
			anthropic = &r.Summary.ByVendor[i]
		}
	}
	if cursor == nil || cursor.Quality == event.QualityAbsent {
		t.Fatalf("cursor %+v", cursor)
	}
	if cursor.Requests != 1 || cursor.UserTurns != 1 || cursor.Total() != 55 {
		t.Fatalf("cursor %+v", cursor)
	}
	if anthropic == nil || anthropic.Total() != 55 {
		t.Fatalf("vendor %+v", anthropic)
	}
	if r.Summary.All.Total() != 55 || r.Summary.All.Total() != cursor.Total() {
		t.Fatalf("conservation all=%d cursor=%d", r.Summary.All.Total(), cursor.Total())
	}
	var vendSum int64
	for _, s := range r.Summary.ByVendor {
		vendSum += s.Total()
	}
	if vendSum != r.Summary.All.Total() {
		t.Fatalf("vendor sum=%d all=%d", vendSum, r.Summary.All.Total())
	}
	foundAuthErr := false
	for _, e := range r.Errors {
		if strings.Contains(e, "未找到本机登录态") {
			foundAuthErr = true
		}
		if strings.Contains(strings.ToLower(e), "bearer") {
			t.Fatalf("error leaked auth: %s", e)
		}
	}
	if !foundAuthErr {
		t.Fatalf("errors=%v", r.Errors)
	}
}

func TestRunTraeMissingAuthIsDegradedNotInvented(t *testing.T) {
	t.Setenv("WHERETOKEN_EXTRA_ROOTS", "")
	dir := t.TempDir()
	gs := filepath.Join(dir, "Library", "Application Support", "Trae CN", "User", "globalStorage")
	if err := os.MkdirAll(gs, 0o755); err != nil {
		t.Fatal(err)
	}
	dbPath := filepath.Join(gs, "state.vscdb")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE ItemTable (key TEXT UNIQUE ON CONFLICT REPLACE, value BLOB)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO ItemTable (key, value) VALUES (?, ?)`,
		"memento/icube-ai-agent-storage", `{"list":[{"sessionId":"sess-1"}]}`); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	r := Run(testhome.New(dir), AllAdapters())
	var trae *metric.Slice
	for i := range r.Summary.BySource {
		if r.Summary.BySource[i].ID == "trae" {
			trae = &r.Summary.BySource[i]
			break
		}
	}
	if trae == nil {
		t.Fatal("expected trae row when Trae CN ledger exists")
	}
	if trae.Total() != 0 || trae.Requests != 0 {
		t.Fatalf("must not invent tokens: %+v", trae)
	}
	if trae.Quality != event.QualityDegraded {
		t.Fatalf("quality=%s want degraded when sessions exist but login is missing", trae.Quality)
	}
	found := false
	for _, e := range r.Errors {
		if strings.Contains(e, "未找到本机登录态") {
			found = true
		}
		if strings.Contains(e, "eyJ") || strings.Contains(strings.ToLower(e), "bearer") {
			t.Fatalf("error leaked auth: %s", e)
		}
	}
	if !found {
		t.Fatalf("errors=%v", r.Errors)
	}
}

func TestMarshalSummaryPutsTraeErrorOnSourceRow(t *testing.T) {
	t.Setenv("WHERETOKEN_EXTRA_ROOTS", "")
	dir := t.TempDir()
	gs := filepath.Join(dir, "Library", "Application Support", "Trae CN", "User", "globalStorage")
	if err := os.MkdirAll(gs, 0o755); err != nil {
		t.Fatal(err)
	}
	dbPath := filepath.Join(gs, "state.vscdb")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE ItemTable (key TEXT UNIQUE ON CONFLICT REPLACE, value BLOB)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO ItemTable (key, value) VALUES (?, ?)`,
		"memento/icube-ai-agent-storage", `{"list":[{"sessionId":"sess-1"}]}`); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	r := Run(testhome.New(dir), AllAdapters())
	raw, err := MarshalSummary(r)
	if err != nil {
		t.Fatal(err)
	}
	var payload struct {
		BySource []struct {
			ID      string `json:"id"`
			Quality string `json:"quality"`
			Error   string `json:"error"`
		} `json:"by_source"`
		ByVendor []struct {
			ID    string `json:"id"`
			Error string `json:"error"`
		} `json:"by_vendor"`
		Errors []string `json:"errors"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	var traeErr string
	for _, s := range payload.BySource {
		if s.ID != "trae" {
			continue
		}
		if s.Quality != string(event.QualityDegraded) {
			t.Fatalf("trae quality=%s", s.Quality)
		}
		traeErr = s.Error
	}
	if traeErr != "未找到本机登录态" {
		t.Fatalf("by_source trae error=%q payload.errors=%v", traeErr, payload.Errors)
	}
	if strings.Contains(traeErr, "eyJ") || strings.Contains(strings.ToLower(traeErr), "bearer") {
		t.Fatal("row error leaked auth")
	}
}

func TestMarshalSummaryPutsEncryptedTraeErrorOnSourceRow(t *testing.T) {
	t.Setenv("WHERETOKEN_EXTRA_ROOTS", "")
	dir := t.TempDir()
	gs := filepath.Join(dir, "Library", "Application Support", "Trae CN", "User", "globalStorage")
	if err := os.MkdirAll(gs, 0o755); err != nil {
		t.Fatal(err)
	}
	dbPath := filepath.Join(gs, "state.vscdb")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE ItemTable (key TEXT UNIQUE ON CONFLICT REPLACE, value BLOB)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO ItemTable (key, value) VALUES (?, ?)`,
		"memento/icube-ai-agent-storage", `{"list":[{"sessionId":"sess-1"}]}`); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	rawStorage := []byte(`{"iCubeAuthInfo://icube.cloudide":"dGMFEAAAfixture-encrypted-blob"}`)
	if err := os.WriteFile(filepath.Join(gs, "storage.json"), rawStorage, 0o600); err != nil {
		t.Fatal(err)
	}

	r := Run(testhome.New(dir), AllAdapters())
	raw, err := MarshalSummary(r)
	if err != nil {
		t.Fatal(err)
	}
	var payload struct {
		BySource []struct {
			ID    string `json:"id"`
			Error string `json:"error"`
		} `json:"by_source"`
		Errors []string `json:"errors"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	var traeErr string
	for _, s := range payload.BySource {
		if s.ID == "trae" {
			traeErr = s.Error
		}
	}
	if traeErr != "登录态在加密存储中，没有可读的 JWT 文件" {
		t.Fatalf("by_source trae error=%q errors=%v", traeErr, payload.Errors)
	}
	blob := string(raw)
	if strings.Contains(blob, "dGMFEAAA") || strings.Contains(blob, "fixture-encrypted") || strings.Contains(blob, "eyJ") {
		t.Fatal("summary leaked storage blob")
	}
}

func TestMarshalSummaryRedactsJWTInErrors(t *testing.T) {
	jwt := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0In0.abc"
	raw, err := MarshalSummary(Result{
		Errors: []string{"trae: bearer " + jwt},
		Summary: metric.Summary{
			BySource: []metric.Slice{{ID: "trae", Label: "Trae"}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	if strings.Contains(s, "eyJ") || strings.Contains(s, jwt) {
		t.Fatalf("scan JSON leaked JWT:\n%s", s)
	}
	if !strings.Contains(s, "[redacted]") {
		t.Fatalf("expected redaction:\n%s", s)
	}
}

func writeScanCursorDB(t *testing.T, path string) {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE cursorDiskKV (key TEXT UNIQUE ON CONFLICT REPLACE, value BLOB)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO cursorDiskKV (key, value) VALUES (?, ?)`,
		"composerData:s1", `{"composerId":"s1","createdAt":1700000000000,"modelConfig":{"modelName":"claude-opus-4-6"},"usageData":{}}`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO cursorDiskKV (key, value) VALUES (?, ?)`,
		"bubbleId:s1:u1", `{"type":1,"createdAt":"2026-02-09T14:44:05.860Z","tokenCount":{"inputTokens":0,"outputTokens":0}}`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO cursorDiskKV (key, value) VALUES (?, ?)`,
		"bubbleId:s1:a1", `{"type":2,"createdAt":"2026-02-09T14:44:08.000Z","tokenCount":{"inputTokens":40,"outputTokens":15}}`); err != nil {
		t.Fatal(err)
	}
}

func summaryEvaluationLevel(t *testing.T, r Result) (string, string) {
	t.Helper()
	raw, err := MarshalSummary(r)
	if err != nil {
		t.Fatal(err)
	}
	var payload struct {
		Evaluation struct {
			Level   string `json:"level"`
			Summary string `json:"summary"`
			Reason  string `json:"reason"`
		} `json:"evaluation"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	return payload.Evaluation.Level, payload.Evaluation.Summary + "|" + payload.Evaluation.Reason
}

func TestMarshalSummaryIncludesEvaluation(t *testing.T) {
	now := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)
	r := Result{
		Summary: metric.AggregateAt([]event.UsageEvent{
			{Source: "kimi", Vendor: "moonshot", Model: "k3", Miss: 5_000_000, Timestamp: now},
		}, nil, now, time.UTC),
		Errors: []string{},
	}
	level, detail := summaryEvaluationLevel(t, r)
	if level != "high_usage" || !strings.Contains(detail, "高强度使用") {
		t.Fatalf("level=%s detail=%s", level, detail)
	}
	if !strings.Contains(detail, "5.00 M") {
		t.Fatalf("evaluation must explain itself: %s", detail)
	}
}

func TestMarshalSummaryEmptyEvaluationIsDash(t *testing.T) {
	level, detail := summaryEvaluationLevel(t, Result{Errors: []string{}})
	if level != "none" || !strings.HasPrefix(detail, "—") {
		t.Fatalf("empty window is —, never light: level=%s detail=%s", level, detail)
	}
}

func TestWindowedEvaluationFollowsTheWindow(t *testing.T) {
	now := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)
	evs := []event.UsageEvent{
		{Source: "kimi", Vendor: "moonshot", Model: "k3", Miss: 6_000_000, Timestamp: now},
	}
	for i := 1; i <= 4; i++ {
		evs = append(evs, event.UsageEvent{
			Source: "kimi", Vendor: "moonshot", Model: "k3",
			Miss: 400_000, Timestamp: now.AddDate(0, 0, -i),
		})
	}
	r := Result{
		Summary: metric.AggregateAt(evs, nil, now, time.UTC),
		Events:  evs,
		Errors:  []string{},
	}
	win, err := metric.ParseWindow(true, "", "", "", now, time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	levelToday, _ := summaryEvaluationLevel(t, ApplyWindow(r, win, time.UTC))
	if levelToday != "high_usage" {
		t.Fatalf("today window: %s", levelToday)
	}
	levelAll, _ := summaryEvaluationLevel(t, r)
	if levelAll == "high_usage" {
		t.Fatalf("all-time window must not inherit today's intensity: %s", levelAll)
	}
}
