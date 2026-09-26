package cli

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// TestProfileRefreshLaunchdRoundTrip bootstraps a user agent with the
// production plist writer and the production launchctl commands. It skips
// when launchctl is not on this machine. That skip is an environment
// absence. On darwin, when launchctl exists, the test must run.
func TestProfileRefreshLaunchdRoundTrip(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("launchctl is absent on this OS")
	}
	if _, err := exec.LookPath("launchctl"); err != nil {
		t.Skip("launchctl absent")
	}
	if os.Geteuid() == 0 {
		t.Skip("refusing to bootstrap a user agent as root")
	}
	home := t.TempDir()
	bin := filepath.Join(t.TempDir(), "fixture")
	script := "#!/bin/sh\nexec /bin/sleep 600\n"
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	app := &App{
		GOOS:     "darwin",
		UserHome: func() (string, error) { return home, nil },
	}
	plist := filepath.Join(home, "Library", "LaunchAgents", profileRefreshLabel+".plist")
	service := fmt.Sprintf("gui/%d/%s", os.Getuid(), profileRefreshLabel)
	t.Cleanup(func() {
		app.stopProfileRefreshAgent()
		_ = exec.Command("launchctl", "bootout", fmt.Sprintf("gui/%d", os.Getuid()), plist).Run()
	})
	if err := app.installProfileRefreshAgent(bin); err != nil {
		t.Fatal(err)
	}
	if err := app.installProfileRefreshAgent(bin); err != nil {
		t.Fatalf("second on: %v", err)
	}
	out, err := exec.Command("launchctl", "print", service).CombinedOutput()
	if err != nil {
		t.Fatalf("print: %v %s", err, out)
	}
	if !launchctlHasPID(string(out)) {
		t.Fatalf("print has no pid:\n%s", out)
	}
	body, err := os.ReadFile(plist)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(body, []byte(bin)) || bytes.Contains(body, []byte("StartInterval")) || bytes.Contains(body, []byte("EnvironmentVariables")) {
		t.Fatalf("plist was not the production agent:\n%s", body)
	}
	matches, err := filepath.Glob(filepath.Join(home, "Library", "LaunchAgents", "*.plist"))
	if err != nil || len(matches) != 1 {
		t.Fatalf("jobs %v %v", matches, err)
	}
	app.stopProfileRefreshAgent()
	if _, err := os.Stat(plist); !os.IsNotExist(err) {
		t.Fatal("off left the plist")
	}
	out, err = exec.Command("launchctl", "print", service).CombinedOutput()
	if err == nil && launchctlHasPID(string(out)) {
		t.Fatalf("job still loaded:\n%s", out)
	}
}
