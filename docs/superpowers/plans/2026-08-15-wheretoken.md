# whereToken Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship a local `wheretoken` binary that scans Claude Code / Kimi / OpenCode / Codex usage and prints one JSON (and a Vue page) with grand totals plus the same six columns split by tool and by vendor.

**Architecture:** Adapters emit `event.UsageEvent` and `event.TurnEvent`. `vendor.Lookup` fills the manufacturer axis. `metric.Aggregate` produces `Summary{All, BySource, ByVendor, BySourceVendor}` with a conservation invariant. `scan` orchestrates discovery; CLI and HTTP both serialize the same `Summary`. Vue only renders that payload.

**Tech Stack:** Go 1.25, `net/http` stdlib (no chi until a second route family needs it), Vue 3 + Vite + TypeScript + Pinia + ECharts, MIT license. SQLite cache is deferred until a profiler shows rescan > 10s; v1 scans on demand.

## Global Constraints

- Token math is `int64` until display; display divides by `1_000_000` and suffixes `M`.
- `total = miss + cache_read + cache_create + output`.
- `hit_rate = cache_read / (cache_read + miss + cache_create)`; denominator 0 → `—`.
- `sum(BySource.Total) == All.Total == sum(ByVendor.Total)`.
- Tool (source) ≠ vendor. Claude Code + model `MiniMax-M3` → source `claude`, vendor `minimax`.
- User turns attach to the tool axis only.
- Read-only sources. Never open `settings.json`, `auth.json`, or OpenCode `account` / `credential` tables.
- Bind `127.0.0.1` only.
- Git commits must not contain `Co-authored-by`.
- TDD: no production code before a failing test. Watch the fail.
- Spec: `docs/superpowers/specs/2026-08-15-wheretoken-design.md` and `docs/data-sources.md`.

## File map (locked)

| Path | Responsibility |
|------|----------------|
| `LICENSE` | MIT |
| `go.mod` | module `github.com/rainhuang0220/whereToken` |
| `internal/event/event.go` | `UsageEvent`, `TurnEvent`, `Quality` |
| `internal/vendor/vendor.go` | `Lookup`, `Label` |
| `internal/metric/format.go` | `FormatM`, `HitRate` |
| `internal/metric/summary.go` | `Slice`, `Summary`, `Aggregate`, `View` |
| `internal/adapter/adapter.go` | `Adapter`, `SourceRoot`, `Home` |
| `internal/adapter/testhome/home.go` | fake `$HOME` for tests |
| `internal/adapter/kimi/kimi.go` | Kimi `usage.record` |
| `internal/adapter/opencode/opencode.go` | OpenCode sqlite message.tokens |
| `internal/adapter/codex/codex.go` | Codex cumulative deltas |
| `internal/adapter/claude/claude.go` | Claude JSONL max(requestId) |
| `internal/scan/scan.go` | discover + parse + aggregate |
| `cmd/wheretoken/main.go` | `scan` / `serve` / `sources` |
| `internal/httpapi/httpapi.go` | `GET /api/summary`, static web |
| `web/` | Vue dashboard |
| `testdata/adapters/{kimi,codex,claude}/` | anonymized fixtures |
| `scripts/verify-local.sh` | live-disk cross-check, not fixtures |

---

### Task 1: Event types, M formatting, hit rate

**Files:**
- Create: `LICENSE`
- Create: `go.mod`
- Create: `internal/event/event.go`
- Create: `internal/metric/format.go`
- Test: `internal/metric/format_test.go`

**Interfaces:**
- Consumes: nothing
- Produces: `event.UsageEvent`, `event.TurnEvent`, `event.Quality` (`authoritative` \| `degraded` \| `estimated` \| `absent`); `metric.FormatM(tokens int64) string`; `metric.HitRate(miss, cacheRead, cacheCreate int64) (percent float64, ok bool)`

- [ ] **Step 1: Write the failing test**

```go
package metric

import "testing"

func TestFormatM(t *testing.T) {
	cases := []struct {
		in   int64
		want string
	}{
		{0, "0.00 M"},
		{1_000_000, "1.00 M"},
		{360_109_885, "360.11 M"},
		{1_741, "0.0017 M"},
		{9_999, "0.0100 M"},
		{10_000, "0.01 M"},
	}
	for _, c := range cases {
		if got := FormatM(c.in); got != c.want {
			t.Fatalf("FormatM(%d)=%q want %q", c.in, got, c.want)
		}
	}
}

func TestHitRate(t *testing.T) {
	pct, ok := HitRate(97_763_998, 252_128_914, 8_952_459)
	if !ok {
		t.Fatal("expected ok")
	}
	if pct < 70.2 || pct > 70.3 {
		t.Fatalf("pct=%v", pct)
	}
	if _, ok := HitRate(0, 0, 0); ok {
		t.Fatal("zero denom must not be ok")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/metric -count=1`
Expected: FAIL — `go.mod` missing or `FormatM` undefined.

- [ ] **Step 3: Write minimal implementation**

`go.mod`:

```
module github.com/rainhuang0220/whereToken

go 1.25
```

`internal/event/event.go`:

```go
package event

import "time"

type Quality string

const (
	QualityAuthoritative Quality = "authoritative"
	QualityDegraded      Quality = "degraded"
	QualityEstimated     Quality = "estimated"
	QualityAbsent        Quality = "absent"
)

type UsageEvent struct {
	Source, Vendor, SourceRoot string
	SessionID, RequestID       string
	Model, Provider, Workspace string
	Timestamp                  time.Time
	Miss, CacheRead, CacheCreate, Output, Reasoning int64
	Quality                    Quality
}

type TurnEvent struct {
	Source, SessionID, Workspace string
	Timestamp                    time.Time
}
```

`internal/metric/format.go`:

```go
package metric

import "fmt"

func FormatM(tokens int64) string {
	m := float64(tokens) / 1_000_000
	if tokens != 0 && m < 0.01 {
		return fmt.Sprintf("%.4f M", m)
	}
	return fmt.Sprintf("%.2f M", m)
}

func HitRate(miss, cacheRead, cacheCreate int64) (float64, bool) {
	den := cacheRead + miss + cacheCreate
	if den == 0 {
		return 0, false
	}
	return 100 * float64(cacheRead) / float64(den), true
}
```

Copy MIT text into `LICENSE` (copyright `rainhuang0220`).

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/metric -count=1`
Expected: PASS. If `9_999` → `0.0100 M` vs `0.01 M` disagreement, change the test to the rounding you implement and keep the spec rule: below `0.01 M` use 4 decimals; `0.01 M` and above use 2.

- [ ] **Step 5: Commit**

```bash
git add LICENSE go.mod internal/event/event.go internal/metric/format.go internal/metric/format_test.go
git commit -m "$(cat <<'EOF'
feat: add token M formatting and hit-rate math

EOF
)"
```

Do not add `Co-authored-by`.

---

### Task 2: Vendor lookup

**Files:**
- Create: `internal/vendor/vendor.go`
- Test: `internal/vendor/vendor_test.go`

**Interfaces:**
- Consumes: none
- Produces: `vendor.Lookup(model, provider string) string`; `vendor.Label(id string) string`

- [ ] **Step 1: Write the failing test**

```go
package vendor

import "testing"

func TestLookup(t *testing.T) {
	cases := []struct{ model, provider, want string }{
		{"MiniMax-M3", "", "minimax"},
		{"claude-opus-4.6", "", "anthropic"},
		{"kimi-code/k3", "", "moonshot"},
		{"k3", "kimi-for-coding", "moonshot"},
		{"gpt-5", "", "openai"},
		{"o3-mini", "", "openai"},
		{"gemini-2.5-pro", "", "google"},
		{"totally-unknown-model", "", "unknown"},
	}
	for _, c := range cases {
		if got := Lookup(c.model, c.provider); got != c.want {
			t.Fatalf("Lookup(%q,%q)=%q want %q", c.model, c.provider, got, c.want)
		}
	}
}

func TestLabel(t *testing.T) {
	if Label("moonshot") != "Moonshot" {
		t.Fatal(Label("moonshot"))
	}
	if Label("unknown") != "Unknown" {
		t.Fatal(Label("unknown"))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/vendor -count=1`
Expected: FAIL, `Lookup` undefined.

- [ ] **Step 3: Write minimal implementation**

Match spec order: minimax, anthropic, moonshot, openai, google, else unknown. Compare on `strings.ToLower(model + " " + provider)`. Moonshot also matches exact model `k3`. OpenAI matches prefix `o1`/`o3`/`o4` on the model token, not the concatenated string (avoid false positives). `Label` map: `anthropic→Anthropic`, `moonshot→Moonshot`, `openai→OpenAI`, `minimax→MiniMax`, `google→Google`, `unknown→Unknown`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/vendor -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/vendor
git commit -m "$(cat <<'EOF'
feat: map models to manufacturers

EOF
)"
```

---

### Task 3: Aggregate totals, by tool, by vendor

**Files:**
- Create: `internal/metric/summary.go`
- Test: `internal/metric/summary_test.go`

**Interfaces:**
- Consumes: `event.UsageEvent`, `event.TurnEvent`, `vendor.Label`, `metric.FormatM`, `metric.HitRate`
- Produces:
  - `type Slice struct { ID, Label string; Miss, CacheRead, CacheCreate, Output int64; Requests, UserTurns int64; Quality event.Quality }`
  - `func (s Slice) Total() int64`
  - `type SourceVendor struct { Source, Vendor, SourceLabel, VendorLabel string; Miss, CacheRead, CacheCreate, Output, Requests int64 }`
  - `func (s SourceVendor) Total() int64`
  - `type Summary struct { All Slice; BySource []Slice; ByVendor []Slice; BySourceVendor []SourceVendor }`
  - `func Aggregate(events []event.UsageEvent, turns []event.TurnEvent) Summary`
  - `type SliceView struct` with JSON tags `id,label,miss,cache_read,cache_create,output,total,miss_m,cache_read_m,cache_create_m,output_m,total_m,hit_rate,hit_rate_text,requests,user_turns,quality` (`hit_rate` is `*float64`, null when not ok)
  - `func View(s Slice) SliceView`

Source labels: `claude→Claude Code`, `kimi→Kimi Code`, `opencode→OpenCode`, `codex→Codex`.

- [ ] **Step 1: Write the failing test**

```go
package metric

import (
	"testing"

	"github.com/rainhuang0220/whereToken/internal/event"
)

func TestAggregateSplitsToolAndVendor(t *testing.T) {
	events := []event.UsageEvent{
		{Source: "claude", Vendor: "anthropic", RequestID: "a", Miss: 100, CacheRead: 900, Output: 10, Quality: event.QualityDegraded},
		{Source: "claude", Vendor: "minimax", RequestID: "b", Miss: 50, Output: 5, Quality: event.QualityDegraded},
		{Source: "kimi", Vendor: "moonshot", RequestID: "c", Miss: 20, CacheRead: 80, Output: 3, Quality: event.QualityAuthoritative},
	}
	turns := []event.TurnEvent{
		{Source: "claude"},
		{Source: "claude"},
		{Source: "kimi"},
	}
	sum := Aggregate(events, turns)
	if sum.All.Total() != 100+900+10+50+5+20+80+3 {
		t.Fatalf("all=%d", sum.All.Total())
	}
	var src, vend int64
	for _, s := range sum.BySource {
		src += s.Total()
		if s.ID == "claude" && s.UserTurns != 2 {
			t.Fatalf("claude turns=%d", s.UserTurns)
		}
	}
	for _, s := range sum.ByVendor {
		vend += s.Total()
	}
	if src != sum.All.Total() || vend != sum.All.Total() {
		t.Fatalf("conservation src=%d vend=%d all=%d", src, vend, sum.All.Total())
	}
	if sum.All.UserTurns != 3 {
		t.Fatalf("turns=%d", sum.All.UserTurns)
	}
	if len(sum.BySourceVendor) < 3 {
		t.Fatalf("cross=%d", len(sum.BySourceVendor))
	}
}

func TestAggregateDedupesRequestID(t *testing.T) {
	events := []event.UsageEvent{
		{Source: "claude", Vendor: "anthropic", RequestID: "same", Miss: 1, CacheRead: 10, Output: 1},
		{Source: "claude", Vendor: "anthropic", RequestID: "same", Miss: 5, CacheRead: 10, Output: 2},
	}
	sum := Aggregate(events, nil)
	if sum.All.Requests != 1 {
		t.Fatalf("requests=%d", sum.All.Requests)
	}
	if sum.All.Miss != 5 || sum.All.Output != 2 || sum.All.CacheRead != 10 {
		t.Fatalf("max fields %+v", sum.All)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/metric -count=1`
Expected: FAIL, `Aggregate` undefined.

- [ ] **Step 3: Write minimal implementation**

Dedupe key = `source + "\x00" + requestID`; empty requestID does not collapse distinct events (use a per-event unique fallback only when RequestID is empty: do not merge them). When RequestID is non-empty, keep the max of each token field. Then bucket by source, vendor, and source+vendor. Worst quality in a bucket wins, with order `degraded > authoritative > absent`. `All.ID` is `"all"`, `All.Label` is `"合计"`. Sort `BySource`/`ByVendor` by `Total()` descending.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/metric -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/metric/summary.go internal/metric/summary_test.go
git commit -m "$(cat <<'EOF'
feat: aggregate usage by tool and manufacturer

EOF
)"
```

---

### Task 4: Adapter interface and fake home

**Files:**
- Create: `internal/adapter/adapter.go`
- Create: `internal/adapter/testhome/home.go`
- Test: `internal/adapter/testhome/home_test.go`

**Interfaces:**
- Consumes: `event.UsageEvent`, `event.TurnEvent`
- Produces:

```go
package adapter

type SourceRoot struct {
	ID   string
	Path string
}

type Home interface {
	DotDir(name string) string      // $HOME/.<name>
	XDGData(name string) string     // $XDG_DATA_HOME/<name> or $HOME/.local/share/<name>
	AppSupport(name string) string  // macOS Application Support/<name>
}

type Adapter interface {
	ID() string
	Discover(home Home) []SourceRoot
	Parse(root SourceRoot, emit func(event.UsageEvent), emitTurn func(event.TurnEvent)) error
}
```

`testhome.New(root string) adapter.Home` maps those three methods under `root`.

- [x] **Step 1: Write the failing test**

```go
package testhome

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDotDir(t *testing.T) {
	dir := t.TempDir()
	h := New(dir)
	want := filepath.Join(dir, ".kimi-code")
	if h.DotDir("kimi-code") != want {
		t.Fatalf("got %q", h.DotDir("kimi-code"))
	}
	if err := os.MkdirAll(want, 0o755); err != nil {
		t.Fatal(err)
	}
}
```

- [x] **Step 2: Run test to verify it fails**

Run: `go test ./internal/adapter/testhome -count=1`
Expected: FAIL, `New` undefined.

- [x] **Step 3: Write minimal `New` and `adapter.go`**

- [x] **Step 4: Run test to verify it passes**

Run: `go test ./internal/adapter/... -count=1`
Expected: PASS

- [x] **Step 5: Commit**

```bash
git add internal/adapter
git commit -m "$(cat <<'EOF'
feat: define adapter contract and test home

EOF
)"
```

---

### Task 5: Kimi adapter

**Files:**
- Create: `testdata/adapters/kimi/session/agents/main/wire.jsonl`
- Create: `internal/adapter/kimi/kimi.go`
- Test: `internal/adapter/kimi/kimi_test.go`

**Interfaces:**
- Consumes: `adapter.Adapter`, `adapter.Home`, `vendor.Lookup`
- Produces: `kimi.Adapter` with `ID() == "kimi"`; `Discover` returns `~/.kimi-code` if it exists (and `~/.kimi` if that exists); `Parse` walks `sessions/**/agents/**/wire.jsonl`

Fixture `wire.jsonl` (no prompt text):

```json
{"type":"turn.prompt","origin":{"kind":"user"},"time":1786722811961}
{"type":"usage.record","model":"kimi-code/k3","usage":{"inputOther":100,"output":10,"inputCacheRead":900,"inputCacheCreation":0},"usageScope":"turn","time":1786722944364}
{"type":"usage.record","model":"kimi-code/k3","usage":{"inputOther":50,"output":5,"inputCacheRead":100,"inputCacheCreation":20},"usageScope":"turn","time":1786722975161}
{"type":"turn.prompt","origin":{"kind":"tool"},"time":1786722976000}
```

- [x] **Step 1: Write the failing test**

```go
package kimi

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/rainhuang0220/whereToken/internal/adapter"
	"github.com/rainhuang0220/whereToken/internal/event"
)

func TestParseUsageRecord(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(file), "..", "..", "..", "testdata", "adapters", "kimi")
	var evs []event.UsageEvent
	var turns []event.TurnEvent
	a := Adapter{}
	if err := a.Parse(adapter.SourceRoot{ID: "kimi", Path: root}, func(e event.UsageEvent) { evs = append(evs, e) }, func(t event.TurnEvent) { turns = append(turns, t) }); err != nil {
		t.Fatal(err)
	}
	if len(evs) != 2 {
		t.Fatalf("events=%d", len(evs))
	}
	if evs[0].Vendor != "moonshot" || evs[0].Source != "kimi" {
		t.Fatalf("%+v", evs[0])
	}
	if evs[0].Miss != 100 || evs[0].CacheRead != 900 || evs[0].Output != 10 {
		t.Fatalf("%+v", evs[0])
	}
	if len(turns) != 1 {
		t.Fatalf("turns=%d", len(turns))
	}
}
```

Fix the `turn` parameter name so it does not shadow `testing.T`.

- [x] **Step 2: Run test to verify it fails**

Run: `go test ./internal/adapter/kimi -count=1`
Expected: FAIL, `Adapter` undefined.

- [x] **Step 3: Write parser**

Stream the file line by line. On `usage.record`, map `inputOther→Miss`, `inputCacheRead→CacheRead`, `inputCacheCreation→CacheCreate`, `output→Output`, `vendor.Lookup(model, "")`, `QualityAuthoritative`, `RequestID` = `fmt.Sprintf("%s:%d", path, time)`. On `turn.prompt` with `origin.kind==user`, emit a TurnEvent. Ignore telemetry and `state.json`. Do not read `config.toml`.

- [x] **Step 4: Run test to verify it passes**

Run: `go test ./internal/adapter/kimi -count=1`
Expected: PASS

- [x] **Step 5: Commit**

```bash
git add testdata/adapters/kimi internal/adapter/kimi
git commit -m "$(cat <<'EOF'
feat: parse Kimi Code usage.record ledgers

EOF
)"
```

---

### Task 6: OpenCode adapter

**Files:**
- Create: `internal/adapter/opencode/opencode.go`
- Test: `internal/adapter/opencode/opencode_test.go`

**Interfaces:**
- Consumes: `adapter.Adapter`, `vendor.Lookup`
- Produces: `ID()=="opencode"`; Discover `XDGData("opencode")` looking for `opencode.db` then `opencode-stable.db`; Parse uses `file:path?mode=ro`

- [x] **Step 1: Write the failing test** that creates a temp sqlite with:

```sql
CREATE TABLE message (id TEXT, session_id TEXT, time_created INTEGER, time_updated INTEGER, data TEXT);
```

Insert one user-role row without tokens and one assistant-like row:

```json
{"role":"assistant","tokens":{"input":100,"output":10,"reasoning":2,"cache":{"read":50,"write":5}},"modelID":"k3","providerID":"kimi-for-coding"}
```

Assert one event: Miss=100, Output=12 (10+2 reasoning, Reasoning=2 but not added twice), CacheRead=50, CacheCreate=5, Vendor=moonshot, Source=opencode. Query **only** `SELECT data FROM message`. The production SQL string must not contain `account` or `credential`.

Also assert a second test: if `part` table exists with a `step-finish` tokens object, those must **not** be counted when message.data already has tokens.

- [x] **Step 2: Run test to verify it fails**

Run: `go test ./internal/adapter/opencode -count=1`
Expected: FAIL, package missing.

- [x] **Step 3: Write parser using `database/sql` + `modernc.org/sqlite`**

Open DSN `file:%s?mode=ro&immutable=1`. If open fails, retry without `immutable`. Never `SELECT` from `account`, `control_account`, or `credential`.

- [x] **Step 4: Run test to verify it passes**

Run: `go test ./internal/adapter/opencode -count=1`
Expected: PASS

- [x] **Step 5: Commit**

```bash
git add internal/adapter/opencode go.mod go.sum
git commit -m "$(cat <<'EOF'
feat: read OpenCode sqlite message tokens

EOF
)"
```

---

### Task 7: Codex adapter

**Files:**
- Create: `testdata/adapters/codex/sessions/2026/01/01/rollout-dup.jsonl`
- Create: `internal/adapter/codex/codex.go`
- Test: `internal/adapter/codex/codex_test.go`

**Interfaces:**
- Consumes: `adapter.Adapter`, `vendor.Lookup`
- Produces: `ID()=="codex"`; Discover `DotDir("codex")`; Parse `sessions/**/rollout-*.jsonl` and `archived_sessions/**/rollout-*.jsonl` streaming.

Fixture (duplicate snapshot must count once):

```json
{"timestamp":"2026-01-01T00:00:00Z","type":"turn_context","payload":{"model":"gpt-5.2"}}
{"timestamp":"2026-01-01T00:00:01Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":100,"cached_input_tokens":20,"output_tokens":30,"reasoning_output_tokens":5},"last_token_usage":{"input_tokens":100,"cached_input_tokens":20,"output_tokens":30,"reasoning_output_tokens":5}}}}
{"timestamp":"2026-01-01T00:00:02Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":100,"cached_input_tokens":20,"output_tokens":30,"reasoning_output_tokens":5},"last_token_usage":{"input_tokens":100,"cached_input_tokens":20,"output_tokens":30,"reasoning_output_tokens":5}}}}
{"timestamp":"2026-01-01T00:00:03Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":150,"cached_input_tokens":40,"output_tokens":40,"reasoning_output_tokens":8},"last_token_usage":{"input_tokens":50,"cached_input_tokens":20,"output_tokens":10,"reasoning_output_tokens":3}}}}
{"timestamp":"2026-01-01T00:00:04Z","type":"response_item","payload":{"type":"message","role":"user"}}
```

Rules: only emit when `total_token_usage` advances; delta miss = `(input - cached)` of the delta (if input includes cached, miss_delta = input_delta - cached_delta, floor 0); output_delta includes reasoning; vendor from latest turn_context model.

After both advancing events, totals: first event miss=80 cache_read=20 output=35; second miss=30 cache_read=20 output=13. Requests=2. Turns=1.

- [x] **Step 1: Write the failing test** loading that fixture, summing emitted events, asserting Requests via `metric.Aggregate`.

- [x] **Step 2: Run test to verify it fails**

Run: `go test ./internal/adapter/codex -count=1`
Expected: FAIL

- [x] **Step 3: Write streaming parser** using `bufio.Scanner` with a raised buffer (at least 10 MiB) so a long JSON line does not fail.

- [x] **Step 4: Run test to verify it passes**

Run: `go test ./internal/adapter/codex -count=1`
Expected: PASS. A second test with only `last_token_usage` (no total) should count that last usage once.

- [x] **Step 5: Commit**

```bash
git add testdata/adapters/codex internal/adapter/codex
git commit -m "$(cat <<'EOF'
feat: parse Codex token_count as cumulative deltas

EOF
)"
```

---

### Task 8: Claude adapter

**Files:**
- Create: `testdata/adapters/claude/projects/-tmp-demo/s.jsonl`
- Create: `internal/adapter/claude/claude.go`
- Test: `internal/adapter/claude/claude_test.go`

**Interfaces:**
- Consumes: `adapter.Adapter`, `vendor.Lookup`
- Produces: `ID()=="claude"`; Discover `DotDir("claude")/projects` if present; Parse `**/*.jsonl` under that root.

Fixture:

```json
{"type":"user","message":{"role":"user","content":[{"type":"text","text":"hi"}]}}
{"type":"assistant","requestId":"r1","message":{"model":"claude-opus-4.6","usage":{"input_tokens":1,"output_tokens":1,"cache_read_input_tokens":100,"cache_creation_input_tokens":10}}}
{"type":"assistant","requestId":"r1","message":{"model":"claude-opus-4.6","usage":{"input_tokens":8,"output_tokens":2,"cache_read_input_tokens":100,"cache_creation_input_tokens":10}}}
{"type":"assistant","requestId":"r2","message":{"model":"MiniMax-M3","usage":{"input_tokens":20,"output_tokens":4,"cache_read_input_tokens":0,"cache_creation_input_tokens":0}}}
{"type":"user","message":{"role":"user","content":[{"type":"tool_result","content":"ok"}]}}
```

Assert: two usage events after parse (or raw three lines but Aggregate of parse output must have Requests=2, Miss=max(1,8)+20=28, Output=max(1,2)+4=6, one turn only). Event for r2 has Vendor=minimax, Source=claude. Quality=degraded. Parser must never `os.ReadFile` a path whose base is `settings.json`.

- [x] **Step 1: Write the failing test** covering requestId max, MiniMax vendor, tool_result ignored.

- [x] **Step 2: Run test to verify it fails**

Run: `go test ./internal/adapter/claude -count=1`
Expected: FAIL

- [x] **Step 3: Write parser.** Skip `settings.json`. Treat `content` string as a real user turn. `content` list with any `tool_result` is not a user turn.

- [x] **Step 4: Run test to verify it passes**

Run: `go test ./internal/adapter/claude -count=1`
Expected: PASS

- [x] **Step 5: Commit**

```bash
git add testdata/adapters/claude internal/adapter/claude
git commit -m "$(cat <<'EOF'
feat: parse Claude Code JSONL with request dedupe

EOF
)"
```

---

### Task 9: Scan orchestrator and CLI JSON

**Files:**
- Create: `internal/scan/scan.go`
- Create: `internal/scan/oshome.go`
- Test: `internal/scan/scan_test.go`
- Create: `cmd/wheretoken/main.go`

**Interfaces:**
- Consumes: all four adapters, `metric.Aggregate`, `metric.View`
- Produces:
  - `func RealHome() adapter.Home` using `os.UserHomeDir` and `os.Getenv("XDG_DATA_HOME")`
  - `func AllAdapters() []adapter.Adapter`
  - `type Result struct { Summary metric.Summary; Roots []adapter.SourceRoot; Errors []string }`
  - `func Run(home adapter.Home, adapters []adapter.Adapter) Result`
  - CLI: `wheretoken scan --json` writes

```json
{
  "all": { SliceView },
  "by_source": [ SliceView ],
  "by_vendor": [ SliceView ],
  "by_source_vendor": [ SourceVendor ],
  "errors": []
}
```

`func EncodeSummary(w io.Writer, r Result) error` lives in `internal/scan` so HTTP can reuse it. CLI must not recompute totals.

- [x] **Step 1: Write the failing test** using `testhome` populated with the Kimi fixture under `.kimi-code/sessions/x/s/agents/main/wire.jsonl`. `Run` must put Kimi totals on `BySource` id `kimi` and vendor `moonshot`, and `All.Total` equal to that.

- [x] **Step 2: Run test to verify it fails**

Run: `go test ./internal/scan -count=1`
Expected: FAIL

- [x] **Step 3: Implement `Run` and `cmd/wheretoken`**

`main.go` commands: `scan` (default `--json` to stdout), `sources` (print discovered roots as text), `serve` stub that prints `not implemented` and exits 2 until Task 10. `--home` flag overrides the fake/real home for tests via env `WHERETOKEN_HOME` mapped in `RealHome` as: if set, `testhome.New(that)`.

- [x] **Step 4: Run tests and a smoke command**

Run: `go test ./... -count=1`
Run: `go run ./cmd/wheretoken scan --json` against the developer machine.
Expected: JSON has `all`, `by_source`, `by_vendor`. Kimi row present if `~/.kimi-code` exists. Manually confirm `by_source` totals sum to `all.total`.

- [x] **Step 5: Commit**

```bash
git add internal/scan cmd/wheretoken
git commit -m "$(cat <<'EOF'
feat: scan all adapters into one JSON summary

EOF
)"
```

---

### Task 10: HTTP API serving the same summary

**Files:**
- Create: `internal/httpapi/httpapi.go`
- Test: `internal/httpapi/httpapi_test.go`
- Modify: `cmd/wheretoken/main.go` (`serve`)

**Interfaces:**
- Consumes: `scan.Run`, `scan.EncodeSummary`
- Produces: `func Listen(addr string, home adapter.Home) error`; `GET /api/summary` returns the same JSON as `scan --json`; `GET /` serves `web/dist` if present else a one-line `text/plain` `whereToken`. Bind `127.0.0.1`. If port 8787 is taken, try 8788–8797 and print the chosen URL.

- [x] **Step 1: Write the failing test** using `httptest` and testhome+Kimi fixture. `GET /api/summary` must decode `all.total` matching `scan.Run`.

- [x] **Step 2: Run test to verify it fails**

Run: `go test ./internal/httpapi -count=1`
Expected: FAIL

- [x] **Step 3: Implement mux.** No public `0.0.0.0`. `serve` flag `--port` default 8787.

- [x] **Step 4: Run tests**

Run: `go test ./internal/httpapi -count=1`
Expected: PASS

- [x] **Step 5: Commit**

```bash
git add internal/httpapi cmd/wheretoken/main.go
git commit -m "$(cat <<'EOF'
feat: serve summary JSON on localhost

EOF
)"
```

---

### Task 11: Vue dashboard — total, by tool, by vendor

**Files:**
- Create: `web/package.json`, `web/vite.config.ts`, `web/tsconfig.json`, `web/index.html`
- Create: `web/src/main.ts`, `web/src/App.vue`
- Create: `web/src/types.ts` matching `SliceView`
- Create: `web/src/api.ts` (`fetch('/api/summary')`)
- Create: `web/src/components/KpiRow.vue`
- Create: `web/src/components/SliceTable.vue`
- Test: `web/src/format.test.ts` (vitest) asserting it does not re-divide already-formatted `*_m` fields
- Modify: `internal/httpapi` to embed or proxy `web/dist`; in dev, Vite proxy `/api` to `8787`

**Interfaces:**
- Consumes: `/api/summary` JSON from Task 9/10
- Produces: one page with (1) six KPI from `all`, (2) table 「按工具」 from `by_source`, (3) table 「按厂家」 from `by_vendor`, (4) optional details 「工具 × 厂家」 from `by_source_vendor`

Do not recompute totals in the frontend. Display `total_m`, `hit_rate_text`, `miss_m`, `output_m`, `requests`, `user_turns`. Chinese labels. No login.

Read and follow `frontend-design` when writing `App.vue` styles. Keep the page quiet and numeric; no aurora backgrounds.

- [x] **Step 1: Write the failing vitest** that a pure `columnsFrom(view)` returns the six display strings from `SliceView` without calling `/ 1e6`.

- [x] **Step 2: Run `npx vitest run`** in `web/`
Expected: FAIL, module missing.

- [x] **Step 3: Scaffold Vue app and components.** `vite.config.ts` `server.proxy['/api'] = 'http://127.0.0.1:8787'`.

- [x] **Step 4: Run vitest and `npm run build`.** Point `httpapi` at `web/dist`. Manual: `go run ./cmd/wheretoken serve` and `npm run dev` — confirm Claude Code and Kimi are different rows, MiniMax appears under 厂家 if present, and the two tables plus KPI share one payload.

- [x] **Step 5: Commit**

```bash
git add web internal/httpapi
git commit -m "$(cat <<'EOF'
feat: show totals beside per-tool and per-vendor tables

EOF
)"
```

---

### Task 12: Live-disk verifier and README runbook

**Files:**
- Create: `scripts/verify-local.sh`
- Create: `scripts/sum_kimi.py`
- Create: `scripts/sum_opencode.py`
- Modify: `README.md` (dev commands)

**Interfaces:**
- Consumes: `wheretoken scan --json`, the two python scripts
- Produces: exit 0 only if Kimi totals match python exactly and OpenCode totals match python exactly. Claude/Codex only print the JSON slices (no hard-coded 360.11). Scripts must not write home files.

`sum_kimi.py` walks `~/.kimi-code/sessions/**/wire.jsonl`, sums `usage.record` the same way as Task 5.

`sum_opencode.py` opens `~/.local/share/opencode/opencode.db` read-only, sums `message.data` tokens the same way as Task 6.

- [x] **Step 1: Write `scripts/sum_kimi.py` and a test that it agrees with the Task 5 fixture** by pointing it at `testdata/adapters/kimi` via argv.

- [x] **Step 2: Run it on the fixture**
Expected: miss=150 cache_read=1000 cache_create=20 output=15. If the script is missing, it fails.

- [x] **Step 3: Write `verify-local.sh`** that builds `wheretoken`, runs `scan --json`, compares Kimi/OpenCode with python. Skip a source if its directory is absent.

- [x] **Step 4: Run `bash scripts/verify-local.sh`**
Expected: PASS on this developer machine for Kimi and OpenCode.

- [x] **Step 5: Commit**

```bash
git add scripts README.md
git commit -m "$(cat <<'EOF'
test: cross-check Kimi and OpenCode against live disk

EOF
)"
```

---

## Spec coverage

| Spec item | Task |
|-----------|------|
| M units, formulas | 1 |
| Vendor ≠ tool, MiniMax under Claude Code | 2, 3, 8, 11 |
| Conservation all = by_source = by_vendor | 3, 9 |
| Kimi usage.record | 5, 12 |
| OpenCode message.tokens, no credential tables | 6, 12 |
| Codex cumulative deltas | 7 |
| Claude requestId max, tool_result, degraded | 8 |
| scan --json + localhost serve | 9, 10 |
| UI: 合计 + 按工具 + 按厂家 | 11 |
| Read-only, 127.0.0.1, no prompt storage | 5–10 |
| Cursor P1 / Tauri v2 | not in this plan (explicitly later) |
| SQLite cache | deferred; scan on demand until slow |

## Notes for the executor

- If `modernc.org/sqlite` cannot be fetched, fail the task; do not switch to CGO sqlite3 silently.
- Empty `RequestID` events must not collapse together in `Aggregate`.
- Never commit live `~/.claude` transcripts. Fixtures only.
- After Task 9, a human can already see the split in JSON. Do not skip the JSON conservation check to “get to the UI”.
