package syncagg

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/rainhuang0220/whereToken/internal/event"
	"github.com/rainhuang0220/whereToken/internal/price"
)

func shanghai(t *testing.T) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	return loc
}

func testBuildInput(t *testing.T) BuildInput {
	t.Helper()
	loc := shanghai(t)
	ts := func(day, hour int) time.Time {
		return time.Date(2026, 9, day, hour, 0, 0, 0, loc)
	}
	return BuildInput{
		DeviceID:            "dev-1",
		Timezone:            "Asia/Shanghai",
		PriceCatalogVersion: price.CardVersion,
		ClientVersion:       "0.7.0",
		GeneratedAt:         time.Date(2026, 9, 4, 12, 0, 0, 0, loc),
		Loc:                 loc,
		UserHashKey:         testKey(t, 3),
		AccountStableID:     map[string]string{"cursor": "cursor-sub-xyz"},
		Revision:            7,
		Events: []event.UsageEvent{
			{
				Source: "claude", Vendor: "anthropic", Model: "claude-opus-4.6",
				SourceRoot: "/Users/rainhuang/.claude/projects/secret",
				SessionID:  "sess-leak", RequestID: "req-leak",
				Workspace: "/Users/rainhuang/Desktop/secret-repo",
				Timestamp: ts(3, 10),
				Miss:      1_000_000, CacheRead: 2_000_000, Output: 100_000,
				Quality: event.QualityAuthoritative, Derivation: event.DeriveRaw,
			},
			{
				Source: "cursor", Vendor: "anthropic", Model: "claude-4.6-opus-high-thinking",
				SourceRoot: "/Users/rainhuang/Library/Application Support/Cursor",
				SessionID:  "conv-1", RequestID: "cursor-api:conv-1",
				Timestamp: ts(3, 11),
				Miss:      50_000_000, Output: 5_000_000,
				Quality: event.QualityAuthoritative, Derivation: event.DeriveProviderAPI,
				SkipRequest: true,
			},
			{
				Source: "cursor", Vendor: "anthropic", Model: "claude-opus-4.6",
				SourceRoot: "/Users/rainhuang/Library/Application Support/Cursor",
				SessionID:  "bubble-1", Workspace: "/Users/rainhuang/proj",
				Timestamp: ts(3, 12),
				Quality: event.QualityAuthoritative, Derivation: event.DeriveRaw,
			},
		},
		Turns: []event.TurnEvent{
			{Source: "claude", SessionID: "sess-leak", Workspace: "/Users/rainhuang/proj", Timestamp: ts(3, 10)},
			{Source: "cursor", SessionID: "bubble-1", Workspace: "/Users/rainhuang/proj", Timestamp: ts(3, 12)},
		},
		Sources: []SourceInput{
			{Tool: "claude", Detected: true, Status: "ok", Quality: "authoritative"},
			{Tool: "cursor", Detected: true, Status: "ok", Quality: "authoritative"},
			{Tool: "trae", Detected: true, Status: "auth_expired", Quality: "degraded"},
		},
	}
}

func TestBuildAggregatesDailyModelAndSplitsCursorScope(t *testing.T) {
	t.Parallel()
	b, err := Build(testBuildInput(t))
	if err != nil {
		t.Fatal(err)
	}
	if b.SchemaVersion != SchemaVersion {
		t.Fatalf("schema %d", b.SchemaVersion)
	}
	if b.DeviceID != "dev-1" || b.Timezone != "Asia/Shanghai" {
		t.Fatalf("envelope %+v", b)
	}
	if b.PriceCatalogVersion != price.CardVersion {
		t.Fatalf("price catalog %q", b.PriceCatalogVersion)
	}

	var claude, cursorAPI, cursorLocal *DailyModel
	for i := range b.DailyModelUsage {
		row := &b.DailyModelUsage[i]
		switch {
		case row.Tool == "claude":
			claude = row
		case row.Tool == "cursor" && row.SourceScope == ScopeAccountGlobal:
			cursorAPI = row
		case row.Tool == "cursor" && row.SourceScope == ScopeDeviceLocal:
			cursorLocal = row
		}
	}
	if claude == nil || cursorAPI == nil || cursorLocal == nil {
		t.Fatalf("rows=%+v", b.DailyModelUsage)
	}
	if claude.Date != "2026-09-03" || claude.Miss != 1_000_000 || claude.Requests != 1 || claude.UserTurns != 1 {
		t.Fatalf("claude %+v", claude)
	}
	if cursorAPI.Miss != 50_000_000 || cursorAPI.Requests != 0 {
		t.Fatalf("cursor API must SkipRequest: %+v", cursorAPI)
	}
	if cursorLocal.Requests != 1 || cursorLocal.UserTurns != 1 || cursorLocal.Miss != 0 {
		t.Fatalf("cursor local requests/turns only: %+v", cursorLocal)
	}
	if claude.SourceKeyHash == cursorAPI.SourceKeyHash {
		t.Fatal("claude device hash collided with cursor account hash")
	}
}

func TestBuildSerializedPayloadOmitsSensitiveMatter(t *testing.T) {
	t.Parallel()
	b, err := Build(testBuildInput(t))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(b)
	if err != nil {
		t.Fatal(err)
	}
	s := strings.ToLower(string(raw))
	for _, bad := range []string{
		"prompt", "content", "transcript",
		"workspace", "source_root", "sourceroot",
		"session", "session_id", "sessionid",
		"path", "filename", "/users/rainhuang",
		"credential", "authorization", "api_key", "apikey",
		"jwt", "cookie", "home",
		"req-leak", "sess-leak", "secret-repo",
	} {
		if strings.Contains(s, bad) {
			t.Fatalf("payload contains %q: %s", bad, raw)
		}
	}
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		t.Fatal(err)
	}
	walkJSONForbid(t, obj, "", []string{
		"prompt", "content", "workspace", "source_root", "session",
		"session_id", "path", "credential", "authorization", "api_key", "home",
	})
}

func walkJSONForbid(t *testing.T, v any, prefix string, forbid []string) {
	t.Helper()
	switch x := v.(type) {
	case map[string]any:
		for k, child := range x {
			lk := strings.ToLower(k)
			for _, bad := range forbid {
				if lk == bad {
					t.Fatalf("JSON key %s%s", prefix, k)
				}
			}
			walkJSONForbid(t, child, prefix+k+".", forbid)
		}
	case []any:
		for i, child := range x {
			walkJSONForbid(t, child, prefix+"[]"+string(rune('0'+i)), forbid)
		}
	}
}

func TestDecodeBatchRejectsUnknownScopeAndFields(t *testing.T) {
	t.Parallel()
	b, err := Build(testBuildInput(t))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(b)
	if err != nil {
		t.Fatal(err)
	}
	got, err := DecodeBatch(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.DailyModelUsage) != len(b.DailyModelUsage) {
		t.Fatalf("roundtrip rows %d vs %d", len(got.DailyModelUsage), len(b.DailyModelUsage))
	}

	badScope := bytes.Replace(raw, []byte(`"device_local"`), []byte(`"global"`), 1)
	if _, err := DecodeBatch(badScope); err == nil {
		t.Fatal("accepted illegal source_scope")
	}
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		t.Fatal(err)
	}
	obj["prompt"] = "hello"
	leaky, err := json.Marshal(obj)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeBatch(leaky); err == nil {
		t.Fatal("accepted unknown/forbidden field prompt")
	}
}

func TestBuildRequiresHashKey(t *testing.T) {
	t.Parallel()
	in := testBuildInput(t)
	in.UserHashKey = nil
	if _, err := Build(in); err == nil {
		t.Fatal("Build accepted missing HMAC key")
	}
}
