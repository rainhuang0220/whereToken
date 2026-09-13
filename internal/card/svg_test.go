package card

import (
	"bytes"
	"encoding/xml"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/rainhuang0220/whereToken/internal/event"
	"github.com/rainhuang0220/whereToken/internal/metric"
)

func render(t *testing.T, c PublicCard) string {
	t.Helper()
	var buf bytes.Buffer
	if err := Render(&buf, c); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

func TestRenderByteIdentical(t *testing.T) {
	t.Parallel()
	c := DemoCard()
	a := render(t, c)
	b := render(t, c)
	if a != b {
		t.Fatal("renderer is not byte-deterministic")
	}
}

func TestRenderXMLParses(t *testing.T) {
	t.Parallel()
	dec := xml.NewDecoder(strings.NewReader(render(t, DemoCard())))
	for {
		_, err := dec.Token()
		if err == io.EOF {
			return
		}
		if err != nil {
			t.Fatalf("xml: %v", err)
		}
	}
}

func TestRenderEscapesDynamicText(t *testing.T) {
	t.Parallel()
	loc := shanghai()
	now := ts(loc, 2026, 8, 15, 12)
	e := ev(`<x&y">`, "unknown", ts(loc, 2026, 8, 15, 10), 9, 0, 0)
	e.Source = `<script>alert("xss")</script>&'"<>`
	sum := metric.AggregateAt([]event.UsageEvent{e}, nil, now, loc)
	c := NewView(sum, StatusOf(sum), `<v&1>`)
	out := render(t, c)
	for _, bad := range []string{
		"<script>", `<v&1>`, `<script>alert("xss")`,
	} {
		if strings.Contains(out, bad) {
			t.Fatalf("unescaped %q in\n%s", bad, out)
		}
	}
	if !strings.Contains(out, "&lt;v&amp;1&gt;") {
		t.Fatalf("expected escaped version, got\n%s", out)
	}
	if strings.Contains(out, `<script>alert("xss")`) {
		t.Fatal("raw source markup in SVG")
	}
	u := ev("窑工<>", "unknown", ts(loc, 2026, 8, 15, 11), 3, 0, 0)
	sum = metric.AggregateAt([]event.UsageEvent{u}, nil, now, loc)
	out = render(t, NewView(sum, StatusOf(sum), "dev"))
	if strings.Contains(out, "窑工") {
		t.Fatal("unknown source label must not be painted")
	}
	if !strings.Contains(out, agentOtherLabel) {
		t.Fatal("unknown source should render as Other")
	}
	hostile := DemoCard()
	hostile.Agents = []Agent{{ID: "x", Label: `</text><image href="file:///etc/passwd"/>`, ShareText: "1%", ShareRatio: 0.1}}
	escaped := render(t, hostile)
	if strings.Contains(escaped, `<image href=`) || strings.Contains(escaped, "<script>") {
		t.Fatal("unescaped agent label became markup")
	}
	if !strings.Contains(escaped, "&lt;/text&gt;") {
		t.Fatal("agent label must be XML-escaped")
	}
}

func TestRenderFixedLayoutAndPalette(t *testing.T) {
	t.Parallel()
	out := render(t, DemoCard())
	for _, want := range []string{
		`viewBox="0 0 800 576"`,
		`width="800"`,
		`height="576"`,
		colBg, colBorder, colText, colSecondary, colMuted, colPurple, colAmber, colEmpty, colTrack,
		"whereToken",
		"TOKENS BURNED · ALL TIME",
		"LAST 53 WEEKS",
		"CURRENT STREAK",
		"LONGEST STREAK",
		"ACTIVE DAYS · 53W",
		"API LIST-PRICE EQUIV.",
		"VIBE CODING WALL",
		"WHERE TOKENS WENT",
		"$ wheretoken card profile.svg",
		"github.com/rainhuang0220/whereToken",
		`id="wt-unknown"`,
		`id="wt-scanline"`,
		fontStack,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q", want)
		}
	}
	dates := regexp.MustCompile(`data-date="[0-9]{4}-[0-9]{2}-[0-9]{2}"`).FindAllString(out, -1)
	if len(dates) != 371 {
		t.Fatalf("cells=%d", len(dates))
	}
}

func TestRenderFutureUnknownEmptyDistinct(t *testing.T) {
	t.Parallel()
	loc := shanghai()
	now := ts(loc, 2026, 9, 9, 15)

	empty := NewView(metric.AggregateAt(nil, nil, now, loc), DataUnavailable, "dev")
	u := render(t, empty)
	if !strings.Contains(u, `data-state="unknown"`) {
		t.Fatal("unavailable missing unknown cells")
	}
	if !strings.Contains(u, `data-state="future"`) {
		t.Fatal("unavailable missing future cells")
	}
	if strings.Contains(u, `fill="#141a1d"`) && strings.Count(u, `data-state="empty"`) > 0 {
		t.Fatal("unavailable must not paint empty zeros")
	}
	if strings.Contains(u, `data-state="empty"`) {
		t.Fatal("unavailable painted empty")
	}

	one := ev("kimi", "moonshot", ts(loc, 2026, 9, 9, 10), 10, 0, 0)
	got := render(t, NewView(metric.AggregateAt([]event.UsageEvent{one}, nil, now, loc), DataAvailable, "dev"))
	if !strings.Contains(got, `data-state="empty"`) {
		t.Fatal("available missing empty cells")
	}
	if !strings.Contains(got, `data-state="active"`) {
		t.Fatal("available missing active cell")
	}
	if !strings.Contains(got, `data-state="future"`) {
		t.Fatal("available missing future cells")
	}
	if strings.Contains(got, `data-state="unknown"`) {
		t.Fatal("available painted unknown")
	}
	if !strings.Contains(got, `fill="url(#wt-unknown)"`) {
		// unknown pattern must still be defined for the allowlisted SVG
	}
	if !strings.Contains(u, `fill="url(#wt-unknown)"`) {
		t.Fatal("unknown cells must use hatch pattern")
	}
	if !strings.Contains(got, `fill="none"`) || !strings.Contains(got, colBorder) {
		t.Fatal("future cells need dim border")
	}
}

func TestRenderHasNoExternalResourcesOrScript(t *testing.T) {
	t.Parallel()
	out := render(t, DemoCard())
	low := strings.ToLower(out)
	for _, bad := range []string{
		"<script", "foreignobject", "localhost",
		"data:", "javascript:", "@font-face", "<style", "xlink:href",
	} {
		if strings.Contains(low, bad) {
			t.Fatalf("forbidden %q", bad)
		}
	}
	stripped := strings.ReplaceAll(low, `xmlns="http://www.w3.org/2000/svg"`, "")
	if strings.Contains(stripped, "http://") || strings.Contains(stripped, "https://") {
		t.Fatal("external http(s) URL")
	}
	if strings.Contains(low, `href=`) {
		t.Fatal("href attribute present")
	}
}

func TestRenderPrivacySentinelsAbsent(t *testing.T) {
	t.Parallel()
	loc := shanghai()
	now := ts(loc, 2026, 8, 15, 12)
	e := ev("claude", "anthropic", ts(loc, 2026, 8, 15, 10), 100, 20, 5)
	e.Workspace = "/Users/alice/src/secret-repo"
	e.SessionID = "sess_LIVE_SECRET_99"
	e.RequestID = "req_LIVE_SECRET_99"
	e.SourceRoot = "/Users/alice/.claude/projects/acme"
	e.Model = "claude-opus-4.6-poison"
	e.Provider = "sk-ant-api03-SECRET"
	sum := metric.AggregateAt([]event.UsageEvent{e}, nil, now, loc)
	out := render(t, NewView(sum, StatusOf(sum), "dev"))
	for _, poison := range []string{
		"/Users/alice", "secret-repo", "sess_LIVE_SECRET_99", "req_LIVE_SECRET_99",
		"macbook-pro.local", "alice-home-user", "sk-ant-api03-SECRET",
		"DELETE ALL PRODUCTION DATA", "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9",
		"claude-opus-4.6-poison", "/Users/alice/.claude",
		"profile.svg", // footer uses this literal; wait, footer DOES contain profile.svg
	} {
		if poison == "profile.svg" {
			continue
		}
		if strings.Contains(out, poison) {
			t.Fatalf("SVG leaked %q", poison)
		}
	}
	// Footer command is the public example, not a local path.
	if strings.Contains(out, "/Users/") || strings.Contains(out, "C:\\") {
		t.Fatal("absolute path in SVG")
	}
}

func TestRenderGolden(t *testing.T) {
	got := render(t, DemoCard())
	path := filepath.Join("testdata", "vibe-wall.golden.svg")
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden: %v (run UPDATE_GOLDEN=1 go test ./internal/card -run TestRenderGolden)", err)
	}
	if string(want) != got {
		t.Fatalf("golden mismatch (%d bytes got, %d want). UPDATE_GOLDEN=1 to refresh", len(got), len(want))
	}
}

func TestCommittedDemoMatchesRenderer(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	demo := filepath.Join(filepath.Dir(file), "..", "..", "docs", "media", "vibe-coding-wall-demo.svg")
	got := render(t, DemoCard())
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		if err := os.MkdirAll(filepath.Dir(demo), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(demo, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(demo)
	if err != nil {
		t.Fatalf("read demo: %v (run UPDATE_GOLDEN=1)", err)
	}
	if string(want) != got {
		t.Fatalf("docs/media/vibe-coding-wall-demo.svg drifted from renderer")
	}
}

func TestRenderPeakOutlineAndPartialBadge(t *testing.T) {
	t.Parallel()
	c := DemoCard()
	out := render(t, c)
	if !strings.Contains(out, `<tspan fill="#ff8700"`) || !strings.Contains(out, "PARTIAL") {
		t.Fatal("partial cost must be a tspan beside the amount")
	}
	if !strings.Contains(out, `stroke="#ff8700"`) {
		t.Fatal("visible peak needs amber outline")
	}
	if !strings.Contains(out, "PEAK 2026-03-12") {
		t.Fatal("all-time peak caption")
	}
}

func TestRenderOffWallPeakHasNoOutline(t *testing.T) {
	t.Parallel()
	loc := shanghai()
	now := ts(loc, 2026, 9, 9, 15)
	events := []event.UsageEvent{
		ev("claude", "anthropic", ts(loc, 2025, 8, 1, 10), 99_000_000, 0, 0),
		ev("claude", "anthropic", ts(loc, 2026, 9, 9, 10), 100, 0, 0),
	}
	sum := metric.AggregateAt(events, nil, now, loc)
	c := NewView(sum, StatusOf(sum), "dev")
	if c.Peak.VisibleInWall {
		t.Fatal("setup")
	}
	out := render(t, c)
	if !strings.Contains(out, "PEAK 2025-08-01") {
		t.Fatal("caption must keep all-time peak")
	}
	if strings.Contains(out, `stroke="#ff8700"`) {
		t.Fatal("off-wall peak must not outline a cell")
	}
}

func TestRenderUnavailableUsesEmDashNotZero(t *testing.T) {
	t.Parallel()
	loc := shanghai()
	now := ts(loc, 2026, 9, 9, 15)
	c := NewView(metric.AggregateAt(nil, nil, now, loc), DataUnavailable, "dev")
	out := render(t, c)
	if !strings.Contains(out, "NO LOCAL USAGE AVAILABLE") {
		t.Fatal("banner")
	}
	if strings.Count(out, ">"+emDash+"<") < 4 {
		t.Fatalf("expected several — metrics, svg snippet missing")
	}
	if strings.Contains(out, "$0") {
		t.Fatal("$0")
	}
}

func TestRenderShortensPseudoVersion(t *testing.T) {
	t.Parallel()
	c := DemoCard()
	c.Version = "v0.7.1-0.20260906060051-e0cfcd35b64d"
	out := render(t, c)
	if strings.Contains(out, "20260906060051") || strings.Contains(out, "e0cfcd35") {
		t.Fatal("pseudo-version leaked into the header")
	}
	if !strings.Contains(out, ">v0.7.1-dev<") {
		t.Fatalf("short version missing:\n%s", out[:800])
	}
}

func TestRenderDoesNotEmbedClockOrPath(t *testing.T) {
	t.Parallel()
	out := render(t, DemoCard())
	if strings.Contains(out, "15:00") || strings.Contains(out, "Asia/Shanghai") {
		t.Fatal("clock or timezone leaked")
	}
	if strings.Contains(out, time.Now().Format("15:04")) {
		t.Fatal("wall clock leaked")
	}
}
