package cli

import (
	"strings"
	"testing"
)

func TestParseQuiet(t *testing.T) {
	f, err := Parse([]string{"--quiet"})
	if err != nil {
		t.Fatal(err)
	}
	if !f.Quiet {
		t.Fatal("quiet")
	}
	f, err = Parse([]string{"-q"})
	if err != nil || !f.Quiet {
		t.Fatalf("%+v %v", f, err)
	}
}

func TestParseHomeOverride(t *testing.T) {
	f, err := Parse([]string{"--home", "/tmp/fake-home"})
	if err != nil {
		t.Fatal(err)
	}
	if f.Home != "/tmp/fake-home" {
		t.Fatalf("home=%q", f.Home)
	}
}

func TestParseDefaultIsReport(t *testing.T) {
	f, err := Parse(nil)
	if err != nil {
		t.Fatal(err)
	}
	if f.Command != CommandReport {
		t.Fatalf("cmd=%q", f.Command)
	}
}

func TestParseServeKeepsDashboard(t *testing.T) {
	f, err := Parse([]string{"serve", "--port", "8790"})
	if err != nil {
		t.Fatal(err)
	}
	if f.Command != CommandServe || f.Port != 8790 {
		t.Fatalf("%+v", f)
	}
}

func TestParseScanJSONStillAvailable(t *testing.T) {
	f, err := Parse([]string{"scan", "--json"})
	if err != nil {
		t.Fatal(err)
	}
	if f.Command != CommandScan || !f.JSON {
		t.Fatalf("%+v", f)
	}
}

func TestParseHelpAndVersion(t *testing.T) {
	for _, args := range [][]string{{"-h"}, {"--help"}, {"help"}} {
		f, err := Parse(args)
		if err != nil {
			t.Fatalf("%v: %v", args, err)
		}
		if !f.Help {
			t.Fatalf("%v help=%v", args, f.Help)
		}
	}
	for _, args := range [][]string{{"--version"}, {"-V"}, {"version"}} {
		f, err := Parse(args)
		if err != nil {
			t.Fatalf("%v: %v", args, err)
		}
		if !f.Version {
			t.Fatalf("%v version=%v", args, f.Version)
		}
	}
}

func TestParseToolAliasesAndGeneric(t *testing.T) {
	f, err := Parse([]string{"--cursor"})
	if err != nil {
		t.Fatal(err)
	}
	if f.Tool != "cursor" {
		t.Fatalf("tool=%q", f.Tool)
	}
	f, err = Parse([]string{"--tool=kimi"})
	if err != nil {
		t.Fatal(err)
	}
	if f.Tool != "kimi" {
		t.Fatalf("tool=%q", f.Tool)
	}
	f, err = Parse([]string{"--tool", "Claude Code"})
	if err != nil {
		t.Fatal(err)
	}
	if f.Tool != "claude" {
		t.Fatalf("tool=%q", f.Tool)
	}
	f, err = Parse([]string{"--cursor", "--tool=cursor"})
	if err != nil {
		t.Fatal(err)
	}
	if f.Tool != "cursor" {
		t.Fatalf("agree=%q", f.Tool)
	}
}

func TestParseConflictingToolsIsUsage(t *testing.T) {
	_, err := Parse([]string{"--cursor", "--kimi"})
	if err == nil || !IsUsage(err) {
		t.Fatalf("err=%v", err)
	}
	if !strings.Contains(err.Error(), "conflicting") {
		t.Fatalf("err=%v", err)
	}
	_, err = Parse([]string{"--cursor", "--tool=kimi"})
	if err == nil || !IsUsage(err) {
		t.Fatalf("err=%v", err)
	}
	_, err = Parse([]string{"--claude", "--kimi"})
	if err == nil || !IsUsage(err) || !strings.Contains(err.Error(), "conflicting") {
		t.Fatalf("err=%v", err)
	}
}

func TestParseUnknownToolSuggestsClaude(t *testing.T) {
	_, err := Parse([]string{"--tool=claud"})
	if err == nil || !IsUsage(err) {
		t.Fatalf("err=%v", err)
	}
	if !strings.Contains(err.Error(), "claude") {
		t.Fatalf("err=%v", err)
	}
}

func TestParseUnknownToolIsUsage(t *testing.T) {
	_, err := Parse([]string{"--tool=windsurf"})
	if err == nil || !IsUsage(err) {
		t.Fatalf("err=%v", err)
	}
	if err == nil || !strings.Contains(err.Error(), "windsurf") {
		t.Fatalf("err=%v", err)
	}
}

func TestParseUnknownVendorSuggestsAnthropic(t *testing.T) {
	_, err := Parse([]string{"--vendor=anthropc"})
	if err == nil || !IsUsage(err) {
		t.Fatalf("err=%v", err)
	}
	if !strings.Contains(err.Error(), "anthropic") {
		t.Fatalf("err=%v", err)
	}
}

func TestParseUnknownVendorIsUsage(t *testing.T) {
	_, err := Parse([]string{"--vendor=acme"})
	if err == nil || !IsUsage(err) {
		t.Fatalf("err=%v", err)
	}
}

func TestParseScanRejectsTableFilters(t *testing.T) {
	_, err := Parse([]string{"scan", "--today"})
	if err == nil || !IsUsage(err) {
		t.Fatalf("err=%v", err)
	}
	if !strings.Contains(err.Error(), "observatory") {
		t.Fatalf("err=%v", err)
	}
	_, err = Parse([]string{"--tool=claude", "scan"})
	if err == nil || !IsUsage(err) {
		t.Fatalf("err=%v", err)
	}
}

func TestParseVendorXAI(t *testing.T) {
	f, err := Parse([]string{"--vendor=xAI"})
	if err != nil {
		t.Fatal(err)
	}
	if f.Vendor != "xai" {
		t.Fatalf("vendor=%q", f.Vendor)
	}
}

func TestParseTodayJSONVendorModel(t *testing.T) {
	f, err := Parse([]string{"--today", "--json", "--vendor=MiniMax", "--model=k3", "--ascii"})
	if err != nil {
		t.Fatal(err)
	}
	if !f.Today || !f.JSON || f.Vendor != "minimax" || f.Model != "k3" || !f.ASCII {
		t.Fatalf("%+v", f)
	}
}

func TestParseUnknownCommandIsUsage(t *testing.T) {
	_, err := Parse([]string{"explode"})
	if err == nil || !IsUsage(err) {
		t.Fatalf("err=%v", err)
	}
}

func TestParseUnknownFlagIsUsage(t *testing.T) {
	_, err := Parse([]string{"--nope"})
	if err == nil || !IsUsage(err) {
		t.Fatalf("err=%v", err)
	}
	msg := err.Error()
	if strings.Contains(msg, "flag provided but not defined") {
		t.Fatalf("Go flag jargon: %q", msg)
	}
	if !strings.Contains(msg, "unknown flag") || !strings.Contains(msg, "nope") {
		t.Fatalf("err=%v", err)
	}
	if !strings.Contains(msg, "wheretoken --help") {
		t.Fatalf("should point at help: %v", err)
	}
}

func TestParseMissingToolValueIsUsageWithoutGoJargon(t *testing.T) {
	_, err := Parse([]string{"--tool"})
	if err == nil || !IsUsage(err) {
		t.Fatalf("err=%v", err)
	}
	msg := err.Error()
	if strings.Contains(msg, "flag needs an argument") {
		t.Fatalf("Go flag jargon: %q", msg)
	}
	if !strings.Contains(msg, "--tool") || !strings.Contains(msg, "value") {
		t.Fatalf("err=%v", err)
	}
}

func TestParseInvalidPortIsUsageWithoutGoJargon(t *testing.T) {
	_, err := Parse([]string{"--port", "nope"})
	if err == nil || !IsUsage(err) {
		t.Fatalf("err=%v", err)
	}
	msg := err.Error()
	if strings.Contains(msg, "invalid value") && strings.Contains(msg, "parse error") {
		t.Fatalf("Go flag jargon: %q", msg)
	}
	if !strings.Contains(msg, "--port") || !strings.Contains(msg, "nope") {
		t.Fatalf("err=%v", err)
	}
}

func TestParseBrandFlags(t *testing.T) {
	for _, c := range []struct {
		flag, want string
	}{
		{"--claude", "claude"},
		{"--kimi", "kimi"},
		{"--codex", "codex"},
		{"--opencode", "opencode"},
		{"--trae", "trae"},
	} {
		f, err := Parse([]string{c.flag})
		if err != nil {
			t.Fatal(err)
		}
		if f.Tool != c.want {
			t.Fatalf("%s tool=%q", c.flag, f.Tool)
		}
	}
}

func TestParseOffline(t *testing.T) {
	f, err := Parse([]string{"--offline"})
	if err != nil || !f.Offline {
		t.Fatalf("%+v %v", f, err)
	}
}

func TestParseWidth(t *testing.T) {
	f, err := Parse([]string{"--width", "80"})
	if err != nil {
		t.Fatal(err)
	}
	if f.Width != 80 {
		t.Fatalf("width=%d", f.Width)
	}
}

func TestParseBadPortIsUsage(t *testing.T) {
	_, err := Parse([]string{"serve", "--port", "nope"})
	if err == nil || !IsUsage(err) {
		t.Fatalf("err=%v", err)
	}
}

func TestParseNegativeWidthIsUsage(t *testing.T) {
	_, err := Parse([]string{"--width", "-3"})
	if err == nil || !IsUsage(err) {
		t.Fatalf("err=%v", err)
	}
}

func TestParseTodayOfflineCursor(t *testing.T) {
	f, err := Parse([]string{"--today", "--offline", "--cursor", "--quiet"})
	if err != nil {
		t.Fatal(err)
	}
	if !f.Today || !f.Offline || f.Tool != "cursor" || !f.Quiet {
		t.Fatalf("%+v", f)
	}
}

func TestUsageAlias(t *testing.T) {
	err := usageError{msg: "nope"}
	if !IsUsage(err) || !Usage(err) {
		t.Fatal(err)
	}
	if IsUsage(nil) || Usage(nil) {
		t.Fatal("nil")
	}
}

func TestParseFlagsBeforeServe(t *testing.T) {
	f, err := Parse([]string{"--port", "8791", "serve"})
	if err != nil {
		t.Fatal(err)
	}
	if f.Command != CommandServe || f.Port != 8791 {
		t.Fatalf("%+v", f)
	}
}

func TestParseFlagsAroundServe(t *testing.T) {
	f, err := Parse([]string{"--offline", "--quiet", "serve", "--port", "8799"})
	if err != nil {
		t.Fatal(err)
	}
	if f.Command != CommandServe || !f.Offline || !f.Quiet || f.Port != 8799 {
		t.Fatalf("%+v", f)
	}
}

func TestParseFlagsBeforeSources(t *testing.T) {
	f, err := Parse([]string{"--home", "/tmp/fake-home", "--quiet", "sources"})
	if err != nil {
		t.Fatal(err)
	}
	if f.Command != CommandSources {
		t.Fatalf("cmd=%q", f.Command)
	}
	if f.Home != "/tmp/fake-home" || !f.Quiet {
		t.Fatalf("%+v", f)
	}
}

func TestParseFlagsBeforeCompletion(t *testing.T) {
	f, err := Parse([]string{"--quiet", "completion", "zsh"})
	if err != nil {
		t.Fatal(err)
	}
	if f.Command != CommandCompletion || f.CompletionShell != "zsh" || !f.Quiet {
		t.Fatalf("%+v", f)
	}
}

func TestParseCompletionShellThenFlags(t *testing.T) {
	f, err := Parse([]string{"completion", "zsh", "--quiet"})
	if err != nil {
		t.Fatal(err)
	}
	if f.Command != CommandCompletion || f.CompletionShell != "zsh" || !f.Quiet {
		t.Fatalf("%+v", f)
	}
	f, err = Parse([]string{"--quiet", "completion", "zsh", "--offline"})
	if err != nil {
		t.Fatal(err)
	}
	if !f.Quiet || !f.Offline || f.CompletionShell != "zsh" {
		t.Fatalf("%+v", f)
	}
}

func TestParseFlagsBeforeUnknownCommandIsUsage(t *testing.T) {
	_, err := Parse([]string{"--quiet", "plan"})
	if err == nil || !IsUsage(err) {
		t.Fatalf("err=%v", err)
	}
	if !strings.Contains(err.Error(), "unknown command") {
		t.Fatalf("err=%v", err)
	}
}
