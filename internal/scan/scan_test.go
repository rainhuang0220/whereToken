package scan

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/rainhuang0220/whereToken/internal/adapter/cursor"
	"github.com/rainhuang0220/whereToken/internal/adapter/testhome"
	"github.com/rainhuang0220/whereToken/internal/adapter/trae"
	"github.com/rainhuang0220/whereToken/internal/event"
	"github.com/rainhuang0220/whereToken/internal/metric"
)

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
