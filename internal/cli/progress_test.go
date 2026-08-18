package cli

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/rainhuang0220/whereToken/internal/scan"
	"github.com/rainhuang0220/whereToken/internal/table"
)

func TestScanHUDDrawsMarkAndCaption(t *testing.T) {
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
	if !strings.Contains(got, "▄██████▄") && !strings.Contains(got, "+------+") {
		t.Fatalf("missing clawd slab:\n%s", got)
	}
	if !strings.Contains(got, "挠头中") && !strings.Contains(got, "scratching") {
		t.Fatalf("missing gerund:\n%s", got)
	}
	if strings.Count(strings.Trim(got, "\n"), "\n") > 2 {
		t.Fatalf("HUD should be one status line:\n%s", got)
	}
	h.Clear()
	if h.lines != 0 {
		t.Fatalf("clear left lines=%d", h.lines)
	}
	if h.ticking {
		t.Fatal("ticker still running")
	}
}

func TestScanHUDASCIIKeepsMark(t *testing.T) {
	var buf bytes.Buffer
	h := &scanHUD{w: &buf, ascii: true, now: func() time.Time { return time.Unix(0, 0) }}
	h.Show(scan.Progress{Label: "reading Codex...", Index: 1, Total: 3, Status: scan.ProgressReading})
	got := buf.String()
	if strings.ContainsAny(got, "▄█▀▌") {
		t.Fatalf("ascii leaked unicode:\n%s", got)
	}
	if !strings.ContainsAny(got, "#=[]<>%*+") {
		t.Fatalf("ascii lost mark:\n%s", got)
	}
	h.Clear()
}

func TestScanHUDColorIsLemon(t *testing.T) {
	var buf bytes.Buffer
	h := &scanHUD{w: &buf, color: true, now: func() time.Time { return time.Unix(0, 0) }}
	h.Show(scan.Progress{Label: "hi", Status: scan.ProgressReading})
	got := buf.String()
	if !strings.Contains(got, "38;2;255;215;0") {
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

func TestScanHUDTickerAdvancesMood(t *testing.T) {
	var buf bytes.Buffer
	start := time.Date(2026, 8, 18, 2, 0, 0, 0, time.UTC)
	clock := start
	h := &scanHUD{w: &buf, now: func() time.Time { return clock }}
	h.Show(scan.Progress{Label: "正在读 Kimi…", Index: 1, Total: 6, Status: scan.ProgressReading})
	first := table.SpriteMood(table.SpriteMoodTick(0), false)
	if !strings.Contains(buf.String(), first) {
		t.Fatalf("want mood %q in\n%s", first, buf.String())
	}
	clock = start.Add(500 * time.Millisecond)
	h.mu.Lock()
	h.paintLocked()
	h.mu.Unlock()
	second := table.SpriteMood(table.SpriteMoodTick(500*time.Millisecond), false)
	if first == second {
		t.Fatal("clock should change mood")
	}
	if !strings.Contains(buf.String(), second) {
		t.Fatalf("want later mood %q in\n%s", second, buf.String())
	}
	h.Clear()
}

func TestScanHUDShowsChargeBar(t *testing.T) {
	var buf bytes.Buffer
	h := &scanHUD{w: &buf, now: func() time.Time { return time.Unix(0, 0) }}
	h.Show(scan.Progress{Label: "正在读 Codex…", Index: 3, Total: 6, Status: scan.ProgressReading})
	if !strings.Contains(buf.String(), "▰") {
		t.Fatalf("missing bar:\n%s", buf.String())
	}
	if !strings.Contains(buf.String(), "挠头中") {
		t.Fatalf("missing gerund:\n%s", buf.String())
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
