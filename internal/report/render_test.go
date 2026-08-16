package report

import (
	"strings"
	"testing"

	"github.com/rainhuang0220/whereToken/internal/event"
	"github.com/rainhuang0220/whereToken/internal/table"
)

func TestRenderP0LabelsAndValues(t *testing.T) {
	loc := shanghai()
	now := ts(loc, 2026, 8, 16, 15)
	events, turns := fixture(loc)
	snap, err := Build(events, turns, nil, Filter{}, now, loc)
	if err != nil {
		t.Fatal(err)
	}
	out := Render(snap, Options{})
	for _, want := range []string{
		"whereToken", "有账本以来",
		"总用量", "命中率", "最长连烧", "当前连烧", "请求", "用户回合",
		"11.68 M", "85.2%", "2 天",
		"Claude Code", "Kimi Code", "Anthropic", "MiniMax", "Moonshot",
		"命中率不含输出", "占比", "近7日",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in\n%s", want, out)
		}
	}
	if strings.Contains(out, "消耗") {
		t.Fatal("must not watermark 消耗")
	}
	if strings.Contains(out, "eyJ") {
		t.Fatal("jwt leaked")
	}
}

func TestRenderTodayHidesStreaksShowsBreakdown(t *testing.T) {
	loc := shanghai()
	now := ts(loc, 2026, 8, 16, 15)
	events, turns := fixture(loc)
	snap, err := Build(events, turns, nil, Filter{Today: true}, now, loc)
	if err != nil {
		t.Fatal(err)
	}
	out := Render(snap, Options{})
	if strings.Contains(out, "最长连烧") || strings.Contains(out, "当前连烧") {
		t.Fatalf("streaks on today view:\n%s", out)
	}
	if !strings.Contains(out, "今天 2026-08-16") {
		t.Fatalf("%s", out)
	}
	if !strings.Contains(out, "模型") {
		t.Fatalf("today should list models\n%s", out)
	}
}

func TestRenderASCIIHasNoBoxDrawing(t *testing.T) {
	loc := shanghai()
	now := ts(loc, 2026, 8, 16, 15)
	snap, err := Build(nil, nil, nil, Filter{}, now, loc)
	if err != nil {
		t.Fatal(err)
	}
	out := Render(snap, Options{ASCII: true})
	if strings.ContainsAny(out, "┌┐└┘├┤┬┴┼─│") {
		t.Fatalf("unicode:\n%s", out)
	}
}

func TestRenderTruncatesLongModelNames(t *testing.T) {
	loc := shanghai()
	now := ts(loc, 2026, 8, 16, 15)
	events, turns := fixture(loc)
	events[2].Model = "this-is-an-extremely-long-model-identifier-that-should-not-blow-the-table"
	snap, err := Build(events, turns, nil, Filter{Today: true}, now, loc)
	if err != nil {
		t.Fatal(err)
	}
	out := Render(snap, Options{})
	if strings.Contains(out, "this-is-an-extremely-long-model-identifier-that-should-not-blow-the-table") {
		t.Fatalf("untruncated:\n%s", out)
	}
	if !strings.Contains(out, "…") {
		t.Fatalf("expected ellipsis\n%s", out)
	}
}

func TestRenderBoxLinesSameWidth(t *testing.T) {
	loc := shanghai()
	now := ts(loc, 2026, 8, 16, 15)
	events, turns := fixture(loc)
	snap, err := Build(events, turns, nil, Filter{}, now, loc)
	if err != nil {
		t.Fatal(err)
	}
	out := Render(snap, Options{})
	var box []string
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "┌") || strings.HasPrefix(line, "│") || strings.HasPrefix(line, "├") || strings.HasPrefix(line, "└") {
			box = append(box, line)
		}
	}
	if len(box) != 7 {
		t.Fatalf("box lines=%d\n%s", len(box), out)
	}
	w := table.DisplayWidth(box[0])
	for i, line := range box {
		if table.DisplayWidth(line) != w {
			t.Fatalf("line %d width %d want %d\n%q", i, table.DisplayWidth(line), w, line)
		}
	}
}

func TestRenderFootnoteContinuationIsIndented(t *testing.T) {
	loc := shanghai()
	now := ts(loc, 2026, 8, 16, 15)
	snap, err := Build(nil, nil, []string{"Unknown 厂家 · 账本没写模型名（Cursor 账号用量常这样）"}, Filter{}, now, loc)
	if err != nil {
		t.Fatal(err)
	}
	out := Render(snap, Options{Width: 40, ASCII: true})
	var noteLines []string
	capture := false
	for _, line := range strings.Split(out, "\n") {
		if line == "注" {
			capture = true
			continue
		}
		if capture && line != "" {
			noteLines = append(noteLines, line)
		}
	}
	if len(noteLines) < 2 {
		t.Fatalf("expected wrapped footnote:\n%s", out)
	}
	if !strings.HasPrefix(noteLines[0], "  ·") {
		t.Fatalf("first note: %q\n%s", noteLines[0], out)
	}
	for _, line := range noteLines[1:] {
		if strings.HasPrefix(strings.TrimSpace(line), "·") {
			t.Fatalf("fake bullet: %q\n%s", line, out)
		}
		if !strings.HasPrefix(line, "  ") {
			t.Fatalf("missing hanging indent: %q\n%s", line, out)
		}
	}
}

func TestRenderFitsNarrowWidth(t *testing.T) {
	loc := shanghai()
	now := ts(loc, 2026, 8, 16, 15)
	snap, err := Build(nil, nil, []string{
		"offline · 只用本机账本，没有请求 Cursor/Trae 云端",
		"trae: 登录态在加密存储中，没有可读的 JWT 文件",
	}, Filter{}, now, loc)
	if err != nil {
		t.Fatal(err)
	}
	out := Render(snap, Options{Width: 40, ASCII: true})
	for i, line := range strings.Split(out, "\n") {
		if line == "" {
			continue
		}
		if w := table.DisplayWidth(line); w > 40 {
			t.Fatalf("line %d width %d > 40: %q", i, w, line)
		}
	}
	if !strings.Contains(out, "命中率不含输出") || !strings.Contains(out, "offline") {
		t.Fatalf("lost copy:\n%s", out)
	}
}

func TestRenderZeroData(t *testing.T) {
	loc := shanghai()
	now := ts(loc, 2026, 8, 16, 15)
	snap, err := Build(nil, nil, nil, Filter{}, now, loc)
	if err != nil {
		t.Fatal(err)
	}
	out := Render(snap, Options{})
	if !strings.Contains(out, "0.00 M") || !strings.Contains(out, "—") {
		t.Fatalf("%s", out)
	}
	if !strings.Contains(out, "本机没有找到账本") {
		t.Fatalf("empty home should explain itself:\n%s", out)
	}
}

func TestRenderEmptyHomeFitsNarrowWidth(t *testing.T) {
	loc := shanghai()
	now := ts(loc, 2026, 8, 16, 15)
	snap, err := Build(nil, nil, nil, Filter{}, now, loc)
	if err != nil {
		t.Fatal(err)
	}
	out := Render(snap, Options{Width: 40, ASCII: true})
	for i, line := range strings.Split(out, "\n") {
		if line == "" {
			continue
		}
		if w := table.DisplayWidth(line); w > 40 {
			t.Fatalf("line %d width %d > 40: %q", i, w, line)
		}
	}
	if !strings.Contains(out, "本机没有找到账本") {
		t.Fatalf("%s", out)
	}
}

func TestRenderDimsNotesOnlyWhenColor(t *testing.T) {
	loc := shanghai()
	now := ts(loc, 2026, 8, 16, 15)
	snap, err := Build(nil, nil, []string{"trae: 登录态在加密存储中，没有可读的 JWT 文件"}, Filter{}, now, loc)
	if err != nil {
		t.Fatal(err)
	}
	plain := Render(snap, Options{})
	if strings.Contains(plain, "\x1b") {
		t.Fatal("plain had ANSI")
	}
	color := Render(snap, Options{Color: true})
	if !strings.Contains(color, "\x1b[2m") {
		t.Fatalf("expected dim notes:\n%s", color)
	}
	if strings.Contains(color, "\x1b[2m总用量") {
		t.Fatal("must not dim KPI labels")
	}
}

func TestRenderRedactsNotes(t *testing.T) {
	loc := shanghai()
	now := ts(loc, 2026, 8, 16, 15)
	jwt := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0In0.abc"
	snap, err := Build(nil, nil, []string{"trae: bearer " + jwt}, Filter{}, now, loc)
	if err != nil {
		t.Fatal(err)
	}
	out := Render(snap, Options{})
	if strings.Contains(out, "eyJ") || strings.Contains(out, jwt) {
		t.Fatalf("jwt in output:\n%s", out)
	}
	if !strings.Contains(out, "Trae") {
		t.Fatalf("%s", out)
	}
}

func TestRenderOfflineBannerAfterTitle(t *testing.T) {
	loc := shanghai()
	now := ts(loc, 2026, 8, 16, 15)
	events, turns := fixture(loc)
	snap, err := Build(events, turns, []string{"offline · 只用本机账本，没有请求 Cursor/Trae 云端"}, Filter{}, now, loc)
	if err != nil {
		t.Fatal(err)
	}
	out := Render(snap, Options{})
	lines := strings.Split(out, "\n")
	if len(lines) < 2 {
		t.Fatalf("too short:\n%s", out)
	}
	if !strings.HasPrefix(lines[0], "whereToken") {
		t.Fatalf("title: %q", lines[0])
	}
	if !strings.HasPrefix(lines[1], "offline ·") {
		t.Fatalf("banner should sit under the title, got %q\n%s", lines[1], out)
	}
	if strings.Contains(out, "注\n  · offline") {
		t.Fatalf("offline duplicated in footnotes:\n%s", out)
	}
}

func TestRenderHidesZeroTokenModelTable(t *testing.T) {
	loc := shanghai()
	now := ts(loc, 2026, 8, 16, 15)
	events := []event.UsageEvent{{
		Source: "cursor", Vendor: "unknown", Model: "composer-2.5", RequestID: "a",
		Timestamp: now, Quality: event.QualityDegraded,
	}}
	snap, err := Build(events, nil, nil, Filter{Tool: "cursor"}, now, loc)
	if err != nil {
		t.Fatal(err)
	}
	out := Render(snap, Options{})
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "模型") {
			t.Fatalf("all-zero model table is noise:\n%s", out)
		}
	}
}
