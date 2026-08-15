package cli

import (
	"strings"
	"testing"
)

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
	_, err = Parse([]string{"--cursor", "--tool=kimi"})
	if err == nil || !IsUsage(err) {
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

func TestParseUnknownVendorIsUsage(t *testing.T) {
	_, err := Parse([]string{"--vendor=acme"})
	if err == nil || !IsUsage(err) {
		t.Fatalf("err=%v", err)
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
