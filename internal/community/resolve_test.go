package community

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/rainhuang0220/whereToken/internal/adapter/testhome"
	"github.com/rainhuang0220/whereToken/internal/event"
)

func TestResolveDoesNotCreateFileWithoutURL(t *testing.T) {
	home := testhome.New(t.TempDir())
	path := ConfigPath(home)
	now := time.Date(2026, 8, 19, 15, 0, 0, 0, time.UTC)
	evs := []event.UsageEvent{{
		Vendor: "anthropic", Model: "claude-opus-4.6", Miss: 1000, Timestamp: now,
	}}
	v := Resolve(Request{
		Home:   home,
		Getenv: func(string) string { return "" },
		Now:    now,
		Loc:    time.UTC,
	}, evs)
	if v.Today.Status != StatusServiceUnconfigured || v.Today.Rank != 0 || v.Today.Display != "" || v.Enabled {
		t.Fatalf("%+v enabled=%v", v.Today, v.Enabled)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("must not create %s: %v", path, err)
	}
}

func TestResolveOfflineAndOptOutSkipUpload(t *testing.T) {
	home := testhome.New(t.TempDir())
	now := time.Date(2026, 8, 19, 15, 0, 0, 0, time.UTC)
	getenv := func(k string) string {
		if k == "WHERETOKEN_COMMUNITY_URL" {
			return "http://127.0.0.1:1"
		}
		return ""
	}
	off := Resolve(Request{Home: home, Getenv: getenv, Offline: true, Now: now, Loc: time.UTC}, nil)
	if off.Today.Status != StatusOffline || off.Today.Rank != 0 {
		t.Fatalf("offline %+v", off)
	}
	out := Resolve(Request{Home: home, Getenv: getenv, OptOut: true, Now: now, Loc: time.UTC}, nil)
	if out.Enabled || out.Today.Status != StatusOptedOut {
		t.Fatalf("opt-out %+v", out)
	}
}

func TestResolveEnvOffSkipsUploadAndFile(t *testing.T) {
	home := testhome.New(t.TempDir())
	now := time.Date(2026, 8, 19, 15, 0, 0, 0, time.UTC)
	getenv := func(k string) string {
		switch k {
		case "WHERETOKEN_COMMUNITY_URL":
			return "http://127.0.0.1:1"
		case "WHERETOKEN_COMMUNITY":
			return "off"
		}
		return ""
	}
	v := Resolve(Request{
		Home: home, Getenv: getenv, Now: now, Loc: time.UTC,
		Version: "0.5.0",
	}, []event.UsageEvent{{Vendor: "anthropic", Model: "claude-opus-4.6", Miss: 1000, Timestamp: now}})
	if v.Enabled || v.Today.Status != StatusOptedOut {
		t.Fatalf("%+v", v)
	}
	if _, err := os.Stat(ConfigPath(home)); !os.IsNotExist(err) {
		t.Fatal("env off must not mint community.json")
	}
}

func TestConfigPathStaysOutOfIndex(t *testing.T) {
	t.Setenv("WHERETOKEN_COMMUNITY_FILE", "")
	home := testhome.New(t.TempDir())
	p := ConfigPath(home)
	if filepath.Ext(p) != ".json" {
		t.Fatalf("%s", p)
	}
	if filepath.Base(filepath.Dir(p)) == "cache" {
		t.Fatal("must not live in the usage cache dir")
	}
}

func TestConfigPathUnixVsWindowsLayout(t *testing.T) {
	t.Setenv("WHERETOKEN_COMMUNITY_FILE", "")
	home := testhome.New(t.TempDir())
	unix := filepath.Join(home.XDGConfig("wheretoken"), "community.json")
	win := filepath.Join(home.AppData("whereToken"), "community.json")
	if unix == win {
		t.Fatal("testhome XDGConfig and AppData community.json paths must differ")
	}
	got := ConfigPath(home)
	want := unix
	if runtime.GOOS == "windows" {
		want = win
	}
	if got != want {
		t.Fatalf("ConfigPath=%q want %q (unix XDGConfig=%q windows AppData=%q)", got, want, unix, win)
	}
}

func TestConfigPathHonorsCommunityFileEnv(t *testing.T) {
	home := testhome.New(t.TempDir())
	custom := filepath.Join(t.TempDir(), "override.json")
	t.Setenv("WHERETOKEN_COMMUNITY_FILE", custom)
	if got := ConfigPath(home); got != custom {
		t.Fatalf("ConfigPath=%q want override %q", got, custom)
	}
}
