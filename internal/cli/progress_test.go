package cli

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/rainhuang0220/whereToken/internal/scan"
	"github.com/rainhuang0220/whereToken/internal/table"
)

func TestScanHUDDrawsKidAndCaption(t *testing.T) {
	var buf bytes.Buffer
	now := time.Date(2026, 8, 18, 2, 0, 0, 0, time.UTC)
	h := &scanHUD{w: &buf, now: func() time.Time { return now }}
	h.Show(scan.Progress{
		Source: "codex",
		Label:  "正在读 Codex…",
		Index:  2,
		Total:  6,
		Status: scan.ProgressReading,
	})
	got := buf.String()
	if !strings.Contains(got, "正在读 Codex") {
		t.Fatalf("missing caption:\n%s", got)
	}
	if !strings.Contains(got, "2/6") {
		t.Fatalf("missing progress:\n%s", got)
	}
	if !strings.Contains(got, "(•ᴗ•)") && !strings.Contains(got, "(•-•)") && !strings.Contains(got, "(✧ᴗ✧)") {
		t.Fatalf("missing face:\n%s", got)
	}
	if !strings.Contains(got, "∩∩") {
		t.Fatalf("missing tuft:\n%s", got)
	}
	h.Clear()
	if h.lines != 0 {
		t.Fatalf("clear left lines=%d", h.lines)
	}
	if h.ticking {
		t.Fatal("ticker still running")
	}
}

func TestScanHUDASCIIKeepsFace(t *testing.T) {
	var buf bytes.Buffer
	h := &scanHUD{w: &buf, ascii: true, now: func() time.Time { return time.Unix(0, 0) }}
	h.Show(scan.Progress{Label: "reading Codex...", Index: 1, Total: 3, Status: scan.ProgressReading})
	got := buf.String()
	if strings.ContainsAny(got, "•ᴗ✧≡∩∪") {
		t.Fatalf("ascii leaked unicode:\n%s", got)
	}
	if !strings.Contains(got, "(o_o)") && !strings.Contains(got, "(^_^)") {
		t.Fatalf("ascii lost face:\n%s", got)
	}
	h.Clear()
}

func TestScanHUDColorIsLemon(t *testing.T) {
	var buf bytes.Buffer
	h := &scanHUD{w: &buf, color: true, now: func() time.Time { return time.Unix(0, 0) }}
	h.Show(scan.Progress{Label: "hi", Status: scan.ProgressReading})
	got := buf.String()
	if !strings.Contains(got, "38;5;228") {
		t.Fatalf("want lemon:\n%q", got)
	}
	if strings.Contains(got, "38;5;208") {
		t.Fatal("claude orange")
	}
	h.Clear()
}

func TestScanHUDIgnoresNonReading(t *testing.T) {
	var buf bytes.Buffer
	h := &scanHUD{w: &buf}
	h.Show(scan.Progress{Status: scan.ProgressDone, Label: "done"})
	if buf.Len() != 0 {
		t.Fatalf("wrote %q", buf.String())
	}
}

func TestScanHUDTickerAdvancesPose(t *testing.T) {
	var buf bytes.Buffer
	start := time.Date(2026, 8, 18, 2, 0, 0, 0, time.UTC)
	clock := start
	h := &scanHUD{w: &buf, now: func() time.Time { return clock }}
	h.Show(scan.Progress{Label: "正在读 Kimi…", Index: 1, Total: 6, Status: scan.ProgressReading})
	first := table.SpriteMood(table.SpriteTick(0), false)
	if !strings.Contains(buf.String(), first) {
		t.Fatalf("want mood %q in\n%s", first, buf.String())
	}
	// let the real ticker fire, but drive pose via clock
	clock = start.Add(400 * time.Millisecond)
	h.mu.Lock()
	h.paintLocked()
	h.mu.Unlock()
	second := table.SpriteMood(table.SpriteTick(400*time.Millisecond), false)
	if first == second {
		t.Fatal("clock should change pose")
	}
	if !strings.Contains(buf.String(), second) {
		t.Fatalf("want later mood %q in\n%s", second, buf.String())
	}
	h.Clear()
}

func TestKilnTipRotates(t *testing.T) {
	a := kilnTip(0, false)
	b := kilnTip(2*time.Second, false)
	if a == "" || a == b {
		t.Fatalf("tips should rotate: %q %q", a, b)
	}
	if kilnTip(0, true) == a {
		t.Fatal("ascii tips should differ")
	}
}

func TestScanHUDShowsChargeBar(t *testing.T) {
	var buf bytes.Buffer
	h := &scanHUD{w: &buf, now: func() time.Time { return time.Unix(0, 0) }}
	h.Show(scan.Progress{Label: "正在读 Codex…", Index: 3, Total: 6, Status: scan.ProgressReading})
	if !strings.Contains(buf.String(), "▰") {
		t.Fatalf("missing bar:\n%s", buf.String())
	}
	if !strings.Contains(buf.String(), "煤要一块一块加") {
		t.Fatalf("missing tip:\n%s", buf.String())
	}
	h.Clear()
}

func TestScanHUDClearIsIdempotent(t *testing.T) {
	var buf bytes.Buffer
	h := &scanHUD{w: &buf}
	h.Clear()
	h.Clear()
	h.Show(scan.Progress{Label: "x", Status: scan.ProgressReading})
	h.Clear()
	h.Clear()
	if h.ticking || h.lines != 0 {
		t.Fatalf("ticking=%v lines=%d", h.ticking, h.lines)
	}
}
