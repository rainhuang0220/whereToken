package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/rainhuang0220/whereToken/internal/adapter/testhome"
	"github.com/rainhuang0220/whereToken/internal/publicprofile"
)

func useLaunchd(t *testing.T, app *App, home string) (string, *[]string) {
	t.Helper()
	if home == "" {
		home = t.TempDir()
	}
	app.GOOS = "darwin"
	app.UserHome = func() (string, error) { return home, nil }
	app.Executable = func() (string, error) { return filepath.Join(home, "wheretoken"), nil }
	var calls []string
	app.RunCmdOutput = func(name string, args ...string) (string, error) {
		calls = append(calls, name+" "+strings.Join(args, " "))
		if name != "launchctl" {
			t.Fatalf("unexpected command %s", name)
		}
		switch args[0] {
		case "print":
			return "pid = 4242\nstate = running\n", nil
		case "bootout":
			return "", errors.New("not loaded")
		case "bootstrap":
			return "", nil
		default:
			t.Fatalf("unexpected launchctl %v", args)
		}
		return "", nil
	}
	return home, &calls
}

func TestProfileRefreshPlistIsQuietAndHasNoInterval(t *testing.T) {
	body, err := profileRefreshPlist("/opt/homebrew/bin/wheretoken", "/Users/ada", "/Users/ada/Library/Logs/wheretoken/profile-refresh.log")
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	for _, want := range []string{
		"<key>Label</key>",
		"<string>" + profileRefreshLabel + "</string>",
		"<string>/opt/homebrew/bin/wheretoken</string>",
		"<string>profile</string>",
		"<string>refresh</string>",
		"<string>watch</string>",
		"<string>--quiet</string>",
		"<key>RunAtLoad</key>",
		"<true/>",
		"<key>SuccessfulExit</key>",
		"<false/>",
		"<key>ThrottleInterval</key>",
		"<integer>60</integer>",
		"<key>ProcessType</key>",
		"<string>Background</string>",
		"<key>WorkingDirectory</key>",
		"<string>/Users/ada</string>",
		"profile-refresh.log",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("plist missing %q\n%s", want, text)
		}
	}
	for _, bad := range []string{"StartInterval", "EnvironmentVariables", "--home", "wtd_", "Bearer"} {
		if strings.Contains(text, bad) {
			t.Fatalf("plist contains %q\n%s", bad, text)
		}
	}
	got, err := plistProgram(writePlist(t, body))
	if err != nil || got != "/opt/homebrew/bin/wheretoken" {
		t.Fatalf("program %q %v", got, err)
	}
}

func writePlist(t *testing.T, body []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), profileRefreshLabel+".plist")
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestChooseRefreshProgramSkipsCellarInode(t *testing.T) {
	home := t.TempDir()
	link := filepath.Join(home, ".local", "bin", "wheretoken")
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(link, []byte("bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	cellar := "/opt/homebrew/Cellar/wheretoken/0.7.5/bin/wheretoken"
	got, err := chooseRefreshProgram(cellar, []string{link, "/opt/homebrew/bin/wheretoken", "/usr/local/bin/wheretoken"})
	if err != nil || got != link {
		t.Fatalf("got %q %v", got, err)
	}
	if _, err := chooseRefreshProgram(cellar, []string{"/opt/homebrew/bin/wheretoken"}); err == nil || strings.Contains(err.Error(), "Cellar") {
		t.Fatalf("err %v", err)
	}
	plain := "/usr/local/bin/wheretoken"
	got, err = chooseRefreshProgram(plain, nil)
	if err != nil || got != plain {
		t.Fatalf("non-cellar %q %v", got, err)
	}
}

func TestProfileRefreshOnBootstrapsOnceAndOffStops(t *testing.T) {
	cfg := testhome.New(t.TempDir())
	app, out, errb := testApp([]string{"profile", "refresh", "on"})
	app.Home = cfg
	puts := 0
	app.HTTPDo = func(*http.Request) (*http.Response, error) {
		puts++
		t.Fatal("on uploaded")
		return jsonRes(500, nil)
	}
	home, calls := useLaunchd(t, app, "")
	switchPath := publicprofile.RefreshConfigPath(cfg)
	record := app.RunCmdOutput
	app.RunCmdOutput = func(name string, args ...string) (string, error) {
		on, err := publicprofile.LoadRefreshSwitch(switchPath)
		if err != nil || !on {
			t.Fatalf("%s %v ran before the switch file contained enabled=true (err=%v on=%v)", name, args, err, on)
		}
		return record(name, args...)
	}
	if code := app.Run(); code != ExitOK {
		t.Fatalf("on %d %s", code, errb.String())
	}
	if puts != 0 {
		t.Fatal("on put")
	}
	if !strings.Contains(out.String(), "public profile refresh on (sanitized snapshot only — prompts and paths stay local)") {
		t.Fatalf("on text %s", out.String())
	}
	if len(*calls) != 2 || !strings.HasPrefix((*calls)[0], "launchctl bootout ") || !strings.HasPrefix((*calls)[1], "launchctl bootstrap ") {
		t.Fatalf("first on calls %v", *calls)
	}
	on, err := publicprofile.LoadRefreshSwitch(switchPath)
	if err != nil || !on {
		t.Fatalf("first on switch %v %v", on, err)
	}
	app, out, errb = testApp([]string{"profile", "refresh", "on"})
	app.Home = cfg
	app.HTTPDo = func(*http.Request) (*http.Response, error) {
		t.Fatal("second on uploaded")
		return jsonRes(500, nil)
	}
	_, calls = useLaunchd(t, app, home)
	if code := app.Run(); code != ExitOK {
		t.Fatalf("second on %d %s", code, errb.String())
	}
	if len(*calls) != 2 || !strings.HasPrefix((*calls)[0], "launchctl bootout ") || !strings.HasPrefix((*calls)[1], "launchctl bootstrap ") {
		t.Fatalf("calls %v", *calls)
	}
	boot := strings.TrimPrefix((*calls)[1], "launchctl bootstrap ")
	fields := strings.Fields(boot)
	if len(fields) != 2 || !strings.HasPrefix(fields[0], "gui/") {
		t.Fatalf("bootstrap %q", boot)
	}
	plist := filepath.Join(home, "Library", "LaunchAgents", profileRefreshLabel+".plist")
	if fields[1] != plist {
		t.Fatalf("plist %s want %s", fields[1], plist)
	}
	body, err := os.ReadFile(plist)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), "Cellar") || strings.Contains(string(body), "--home") || strings.Contains(string(body), "StartInterval") {
		t.Fatalf("plist %s", body)
	}
	matches, err := filepath.Glob(filepath.Join(home, "Library", "LaunchAgents", "*.plist"))
	if err != nil || len(matches) != 1 {
		t.Fatalf("plists %v %v", matches, err)
	}

	app, out, errb = testApp([]string{"profile", "refresh", "status"})
	app.Home = cfg
	useLaunchd(t, app, home)
	if code := app.Run(); code != ExitOK {
		t.Fatalf("status %d %s", code, errb.String())
	}
	if !strings.HasPrefix(out.String(), "phase=idle 等待下一次\n") || !strings.Contains(out.String(), "scheduler=running\n") || !strings.Contains(out.String(), "last=\n") {
		t.Fatalf("status %q", out.String())
	}

	app, out, errb = testApp([]string{"profile", "refresh", "off"})
	app.Home = cfg
	useLaunchd(t, app, home)
	if code := app.Run(); code != ExitOK {
		t.Fatalf("off %d %s", code, errb.String())
	}
	if !strings.HasPrefix(out.String(), "public profile refresh off (GitHub profile stays as last published)\n") || !strings.Contains(out.String(), "scheduler=stopped\n") {
		t.Fatalf("off %q", out.String())
	}
	if _, err := os.Stat(plist); !os.IsNotExist(err) {
		t.Fatal("off left the plist")
	}
}

func TestProfileRefreshOnWritesSwitchBeforeLaunchctl(t *testing.T) {
	cfg := testhome.New(t.TempDir())
	app, _, errb := testApp([]string{"profile", "refresh", "on"})
	app.Home = cfg
	home, calls := useLaunchd(t, app, "")
	switchPath := publicprofile.RefreshConfigPath(cfg)
	record := app.RunCmdOutput
	app.RunCmdOutput = func(name string, args ...string) (string, error) {
		if name != "launchctl" {
			t.Fatalf("unexpected command %s", name)
		}
		on, err := publicprofile.LoadRefreshSwitch(switchPath)
		if err != nil || !on {
			t.Fatalf("launchctl %v before enabled=true (err=%v on=%v)", args, err, on)
		}
		return record(name, args...)
	}
	if code := app.Run(); code != ExitOK {
		t.Fatalf("on %d %s", code, errb.String())
	}
	if len(*calls) != 2 || (*calls)[0] == "" || !strings.HasPrefix((*calls)[0], "launchctl bootout ") || !strings.HasPrefix((*calls)[1], "launchctl bootstrap ") {
		t.Fatalf("calls %v", *calls)
	}
	if strings.Contains(errb.String(), home) {
		t.Fatalf("error included a home path: %s", errb.String())
	}
}

func TestProfileRefreshOnKeepsSwitchWhenBootstrapFails(t *testing.T) {
	cfg := testhome.New(t.TempDir())
	app, out, errb := testApp([]string{"profile", "refresh", "on"})
	app.Home = cfg
	home, _ := useLaunchd(t, app, "")
	app.RunCmdOutput = func(name string, args ...string) (string, error) {
		if name != "launchctl" || len(args) == 0 {
			t.Fatalf("unexpected %s %v", name, args)
		}
		on, err := publicprofile.LoadRefreshSwitch(publicprofile.RefreshConfigPath(cfg))
		if err != nil || !on {
			t.Fatalf("launchctl %v before enabled=true (err=%v on=%v)", args, err, on)
		}
		switch args[0] {
		case "bootout":
			return "", errors.New("not loaded")
		case "bootstrap":
			return "", errors.New("bootstrap failed")
		default:
			t.Fatalf("unexpected launchctl %v", args)
		}
		return "", nil
	}
	if code := app.Run(); code == ExitOK {
		t.Fatal("bootstrap failure returned success")
	}
	if errb.String() != "public profile refresh failed; will retry\n" {
		t.Fatalf("stderr %q", errb.String())
	}
	if strings.Contains(errb.String(), home) {
		t.Fatalf("error included a home path: %s", errb.String())
	}
	on, err := publicprofile.LoadRefreshSwitch(publicprofile.RefreshConfigPath(cfg))
	if err != nil || !on {
		t.Fatal("failed bootstrap cleared the switch")
	}

	app, out, errb = testApp([]string{"profile", "refresh", "status"})
	app.Home = cfg
	useLaunchd(t, app, home)
	app.RunCmdOutput = func(name string, args ...string) (string, error) {
		if name == "launchctl" && len(args) > 0 && args[0] == "print" {
			return "", errors.New("not loaded")
		}
		t.Fatalf("status ran %s %v", name, args)
		return "", nil
	}
	if code := app.Run(); code != ExitOK {
		t.Fatalf("status %d %s", code, errb.String())
	}
	if !strings.Contains(out.String(), "scheduler=dead\n") || !strings.HasPrefix(out.String(), "phase=idle 等待下一次\n") {
		t.Fatalf("status %q", out.String())
	}

	app, out, errb = testApp([]string{"profile", "refresh", "on"})
	app.Home = cfg
	_, calls := useLaunchd(t, app, home)
	if code := app.Run(); code != ExitOK {
		t.Fatalf("later on %d %s", code, errb.String())
	}
	if len(*calls) != 2 || !strings.HasPrefix((*calls)[0], "launchctl bootout ") || !strings.HasPrefix((*calls)[1], "launchctl bootstrap ") {
		t.Fatalf("later on calls %v", *calls)
	}
	if !strings.Contains(out.String(), "public profile refresh on (sanitized snapshot only — prompts and paths stay local)") {
		t.Fatalf("later on text %s", out.String())
	}
}

func TestProfileRefreshOnRefusesRoot(t *testing.T) {
	app, _, errb := testApp([]string{"profile", "refresh", "on"})
	app.Home = testhome.New(t.TempDir())
	app.Euid = func() int { return 0 }
	home, _ := useLaunchd(t, app, "")
	app.RunCmdOutput = func(name string, args ...string) (string, error) {
		t.Fatalf("root ran %s %v", name, args)
		return "", nil
	}
	if code := app.Run(); code == ExitOK {
		t.Fatal("root on succeeded")
	}
	if strings.Contains(errb.String(), home) {
		t.Fatalf("error included a home path: %s", errb.String())
	}
	if _, err := os.Stat(publicprofile.RefreshConfigPath(app.Home)); !os.IsNotExist(err) {
		t.Fatal("root on wrote the switch")
	}
}

func TestProfileRefreshOnUnsupportedOnLinuxAndWindows(t *testing.T) {
	for _, goos := range []string{"linux", "windows"} {
		app, out, errb := testApp([]string{"profile", "refresh", "on"})
		app.GOOS = goos
		app.Home = testhome.New(t.TempDir())
		app.RunCmdOutput = func(name string, args ...string) (string, error) {
			t.Fatalf("%s on ran %s %v", goos, name, args)
			return "", nil
		}
		if code := app.Run(); code != ExitOK {
			t.Fatalf("%s on %d %s", goos, code, errb.String())
		}
		want := "public profile refresh on (sanitized snapshot only — prompts and paths stay local)\nscheduler=unsupported\nThis version does not install a background agent on " + goos + ". Public auto-refresh is macOS-only. Foreground loop: wheretoken profile refresh watch\n"
		if out.String() != want {
			t.Fatalf("%s stdout %q", goos, out.String())
		}
	}
}

func TestProfileRefreshStatusAndOffAreUnsupportedOffDarwin(t *testing.T) {
	for _, goos := range []string{"linux", "windows"} {
		cfg := testhome.New(t.TempDir())
		if err := publicprofile.SaveRefreshSwitch(publicprofile.RefreshConfigPath(cfg), true); err != nil {
			t.Fatal(err)
		}
		app, out, errb := testApp([]string{"profile", "refresh", "status"})
		app.GOOS = goos
		app.Home = cfg
		if code := app.Run(); code != ExitOK {
			t.Fatalf("status %d %s", code, errb.String())
		}
		if !strings.HasPrefix(out.String(), "phase=idle 等待下一次\n") || !strings.Contains(out.String(), "scheduler=unsupported\n") {
			t.Fatalf("%s status %q", goos, out.String())
		}
		app, out, errb = testApp([]string{"profile", "refresh", "off"})
		app.GOOS = goos
		app.Home = cfg
		if code := app.Run(); code != ExitOK {
			t.Fatalf("off %d %s", code, errb.String())
		}
		if !strings.Contains(out.String(), "scheduler=unsupported\n") || !strings.HasPrefix(out.String(), "public profile refresh off (GitHub profile stays as last published)\n") {
			t.Fatalf("%s off %q", goos, out.String())
		}
	}
}

func TestProfileRefreshStatusDeadWhenPrintHasNoPID(t *testing.T) {
	cfg := testhome.New(t.TempDir())
	if err := publicprofile.SaveRefreshSwitch(publicprofile.RefreshConfigPath(cfg), true); err != nil {
		t.Fatal(err)
	}
	app, out, errb := testApp([]string{"profile", "refresh", "status"})
	app.Home = cfg
	useLaunchd(t, app, "")
	app.RunCmdOutput = func(name string, args ...string) (string, error) {
		if name == "launchctl" && len(args) > 0 && args[0] == "print" {
			return "", errors.New("not loaded")
		}
		t.Fatalf("status ran %s %v", name, args)
		return "", nil
	}
	if code := app.Run(); code != ExitOK {
		t.Fatalf("status %d %s", code, errb.String())
	}
	if !strings.Contains(out.String(), "scheduler=dead\n") {
		t.Fatalf("%q", out.String())
	}
	app, out, errb = testApp([]string{"profile", "refresh", "status"})
	app.Home = testhome.New(t.TempDir())
	app.GOOS = "darwin"
	called := false
	app.RunCmdOutput = func(string, ...string) (string, error) {
		called = true
		return "", nil
	}
	if code := app.Run(); code != ExitOK {
		t.Fatalf("off status %d %s", code, errb.String())
	}
	if called || !strings.Contains(out.String(), "scheduler=off\n") || !strings.HasPrefix(out.String(), "phase=idle 未开启\n") {
		t.Fatalf("called=%v %q", called, out.String())
	}
}

func TestProfileRefreshWatchReexecsWhenBinaryChanges(t *testing.T) {
	dir := t.TempDir()
	self := filepath.Join(dir, "old")
	next := filepath.Join(dir, "new")
	if err := os.WriteFile(self, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(next, []byte("new"), 0o755); err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	body, err := profileRefreshPlist(next, home, filepath.Join(home, "profile-refresh.log"))
	if err != nil {
		t.Fatal(err)
	}
	plist := filepath.Join(home, "Library", "LaunchAgents", profileRefreshLabel+".plist")
	if err := os.MkdirAll(filepath.Dir(plist), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(plist, body, 0o644); err != nil {
		t.Fatal(err)
	}
	var argv []string
	app := &App{
		GOOS: "darwin",
		Args: []string{"profile", "refresh", "watch", "--quiet"},
		UserHome: func() (string, error) {
			return home, nil
		},
		Executable: func() (string, error) { return self, nil },
		Reexec: func(got []string) {
			argv = append([]string(nil), got...)
		},
	}
	app.maybeReexecProfileWatch()
	if len(argv) != 5 || argv[0] != next || argv[1] != "profile" || argv[4] != "--quiet" {
		t.Fatalf("argv %v", argv)
	}
	app.Executable = func() (string, error) { return next, nil }
	argv = nil
	app.maybeReexecProfileWatch()
	if argv != nil {
		t.Fatalf("same inode reexeced %v", argv)
	}
}

func TestUpdateRebootstrapsProfileRefreshWhenEnabled(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "wheretoken")
	if err := os.WriteFile(dest, []byte("old-bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	payload := []byte("new-bin-contents")
	archive := tarGzNamed(t, "wheretoken", payload)
	sum := sha256.Sum256(archive)
	sums := hex.EncodeToString(sum[:]) + "  wheretoken_darwin_arm64.tar.gz\n"
	cfg := testhome.New(t.TempDir())
	if err := publicprofile.SaveRefreshSwitch(publicprofile.RefreshConfigPath(cfg), true); err != nil {
		t.Fatal(err)
	}
	app, _, errb := testApp([]string{"update"})
	app.Home = cfg
	app.GOOS = "darwin"
	app.GOARCH = "arm64"
	app.Executable = func() (string, error) { return dest, nil }
	app.HTTPGet = func(url string) ([]byte, error) {
		switch {
		case strings.HasSuffix(url, "checksums.txt"):
			return []byte(sums), nil
		case strings.HasSuffix(url, "wheretoken_darwin_arm64.tar.gz"):
			return archive, nil
		default:
			return nil, errors.New("unexpected url")
		}
	}
	launchHome := t.TempDir()
	app.UserHome = func() (string, error) { return launchHome, nil }
	var calls []string
	app.RunCmdOutput = func(name string, args ...string) (string, error) {
		calls = append(calls, name+" "+strings.Join(args, " "))
		if len(args) > 0 && args[0] == "bootout" {
			return "", errors.New("not loaded")
		}
		return "", nil
	}
	if code := app.Run(); code != ExitOK {
		t.Fatalf("update %d %s", code, errb.String())
	}
	if len(calls) != 2 || !strings.HasPrefix(calls[0], "launchctl bootout ") || !strings.HasPrefix(calls[1], "launchctl bootstrap ") {
		t.Fatalf("calls %v", calls)
	}
	if strings.Contains(strings.Join(calls, "\n"), "Cellar") {
		t.Fatalf("cellar path in launchctl: %v", calls)
	}
}

func TestInstallScriptsDoNotEnableProfileRefresh(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	root := filepath.Join(filepath.Dir(file), "..", "..")
	for _, rel := range []string{"scripts/install.sh", "scripts/install.ps1", "scripts/install.cmd"} {
		body, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatal(err)
		}
		text := string(body)
		if strings.Contains(text, "profile refresh") || strings.Contains(text, "launchctl") {
			t.Fatalf("%s enables public refresh", rel)
		}
	}
}
