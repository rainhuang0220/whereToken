package cli

import (
	"os"
	"strings"
	"testing"

	"github.com/rainhuang0220/whereToken/internal/adapter"
	"github.com/rainhuang0220/whereToken/internal/adapter/testhome"
	"github.com/rainhuang0220/whereToken/internal/community"
)

func TestFormatCommunityDoctorStates(t *testing.T) {
	home := testhome.New(t.TempDir())
	off := FormatCommunityDoctor(home, func(string) string { return "" }, true)
	if !strings.Contains(off, "Off this run") {
		t.Fatalf("%s", off)
	}
	none := FormatCommunityDoctor(home, func(string) string { return "" }, false)
	if !strings.Contains(none, "Not configured") {
		t.Fatalf("%s", none)
	}
	url := func(k string) string {
		if k == "WHERETOKEN_COMMUNITY_URL" {
			return "http://127.0.0.1:8798"
		}
		return ""
	}
	ready := FormatCommunityDoctor(home, url, false)
	if !strings.Contains(ready, "no local participant file") {
		t.Fatalf("%s", ready)
	}
}

func TestRunCommunityStatusAndOptOut(t *testing.T) {
	dir := t.TempDir()
	app, out, errb := testApp([]string{"--home", dir, "community", "status"})
	if code := app.Run(); code != ExitOK {
		t.Fatalf("status code=%d %s", code, errb.String())
	}
	s := out.String()
	if !strings.Contains(s, "community rank: off") || !strings.Contains(s, "participant: —") {
		t.Fatalf("status:\n%s", s)
	}
	if strings.Contains(s, "#0") {
		t.Fatal("status printed #0")
	}
	cfg := community.ConfigPath(testhome.New(dir))
	if _, err := os.Stat(cfg); !os.IsNotExist(err) {
		t.Fatalf("status must not create community.json: %v", err)
	}

	app, out, errb = testApp([]string{"--home", dir, "community", "on"})
	if code := app.Run(); code != ExitOK {
		t.Fatalf("on code=%d %s", code, errb.String())
	}
	if !strings.Contains(out.String(), "community rank on") {
		t.Fatalf("on:\n%s", out.String())
	}
	raw, err := os.ReadFile(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"enabled": true`) {
		t.Fatalf("file=%s", raw)
	}

	app, out, errb = testApp([]string{"--home", dir, "community", "off"})
	if code := app.Run(); code != ExitOK {
		t.Fatalf("off code=%d %s", code, errb.String())
	}
	if !strings.Contains(out.String(), "community rank off") {
		t.Fatalf("off:\n%s", out.String())
	}
}

func TestRunReportFifthKPINeverZeroRank(t *testing.T) {
	dir := t.TempDir()
	app, out, errb := testApp([]string{"--home", dir, "--quiet"})
	if code := app.Run(); code != ExitOK {
		t.Fatalf("code=%d %s", code, errb.String())
	}
	s := out.String()
	if !strings.Contains(s, "估价") || !strings.Contains(s, "排名") {
		t.Fatalf("missing fifth column:\n%s", s)
	}
	if strings.Contains(s, "#0") || strings.Contains(s, "Rank 0") {
		t.Fatalf("claimed rank zero:\n%s", s)
	}
	if !strings.Contains(s, "$12.0000") {
		t.Fatalf("priced fixture should print estimate:\n%s", s)
	}
}

func TestRunNoCommunityAndOfflineRank(t *testing.T) {
	dir := t.TempDir()
	app, out, errb := testApp([]string{"--home", dir, "--no-community", "--quiet"})
	if code := app.Run(); code != ExitOK {
		t.Fatalf("code=%d %s", code, errb.String())
	}
	s := out.String()
	if strings.Contains(s, "#0") {
		t.Fatal(s)
	}
	if !strings.Contains(s, "社区排名已关闭") {
		t.Fatalf("opt-out note:\n%s", s)
	}

	app, out, errb = testApp([]string{"--home", dir, "--offline", "--quiet"})
	if code := app.Run(); code != ExitOK {
		t.Fatalf("offline code=%d %s", code, errb.String())
	}
	s = out.String()
	if !strings.Contains(s, "社区排名未上传") {
		t.Fatalf("offline note:\n%s", s)
	}
	if strings.Contains(s, "#0") {
		t.Fatal(s)
	}
}

func TestRunCommunityServeUsesHook(t *testing.T) {
	app, _, errb := testApp([]string{"community", "serve"})
	app.Serve = func(string, adapter.Home, bool) error { return nil }
	if code := app.Run(); code != ExitOK {
		t.Fatalf("code=%d %s", code, errb.String())
	}
	if !strings.Contains(errb.String(), "community rank http://127.0.0.1:8798") {
		t.Fatalf("stderr=%s", errb.String())
	}
}

func TestCaptionHelperNeverZero(t *testing.T) {
	if community.Caption(community.Standing{Status: community.StatusOK, Rank: 0, Display: "#0 / 3"}) != "—" {
		t.Fatal("zero podium")
	}
}

func TestCommunityOffFailsWhenRemoteLeaveFails(t *testing.T) {
	dir := t.TempDir()
	app, _, errb := testApp([]string{"--home", dir, "community", "on"})
	if code := app.Run(); code != ExitOK {
		t.Fatalf("on code=%d %s", code, errb.String())
	}
	cfg := community.ConfigPath(testhome.New(dir))

	app, out, errb := testApp([]string{"--home", dir, "community", "off"})
	app.LookupEnv = func(k string) string {
		if k == "WHERETOKEN_COMMUNITY_URL" {
			return "http://127.0.0.1:1"
		}
		return ""
	}
	if code := app.Run(); code == ExitOK {
		t.Fatalf("off must not claim success when leave cannot reach the service:\n%s\n%s", out.String(), errb.String())
	}
	raw, err := os.ReadFile(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"enabled": true`) {
		t.Fatalf("must keep participation on after failed leave: %s", raw)
	}
}

func TestHelpDocumentsUnixAndWindowsCommunityFile(t *testing.T) {
	h := HelpText()
	if !strings.Contains(h, "~/.config/wheretoken") {
		t.Fatal("help must name the Unix community.json directory")
	}
	if !strings.Contains(h, `%APPDATA%`) {
		t.Fatal("help must name the Windows community.json directory")
	}
}
