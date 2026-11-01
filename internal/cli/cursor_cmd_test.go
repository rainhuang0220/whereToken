package cli

import (
	"strings"
	"testing"
)

func TestCursorConnectStaysInsideHome(t *testing.T) {
	dir := t.TempDir()
	app, out, errb := testApp([]string{"--home", dir, "cursor", "connect"})
	if code := app.Run(); code != ExitOK {
		t.Fatalf("code=%d %s", code, errb.String())
	}
	if !strings.Contains(out.String(), "cursor connect: on") || !strings.Contains(out.String(), "session: missing") {
		t.Fatalf("out=%s err=%s", out.String(), errb.String())
	}
	if strings.Contains(out.String(), "eyJ") || strings.Contains(strings.ToLower(out.String()), "keychain") || strings.Contains(out.String(), "钥匙串") {
		t.Fatalf("status leaked a secret or a keychain instruction:\n%s", out.String())
	}
	app, out, errb = testApp([]string{"--home", dir, "cursor"})
	if code := app.Run(); code != ExitOK {
		t.Fatalf("status code=%d %s", code, errb.String())
	}
	if !strings.Contains(out.String(), "cursor connect: on") {
		t.Fatalf("status=%s", out.String())
	}
	app, out, errb = testApp([]string{"--home", dir, "cursor", "disconnect"})
	if code := app.Run(); code != ExitOK {
		t.Fatalf("disconnect code=%d %s", code, errb.String())
	}
	if !strings.Contains(out.String(), "cursor connect: off") {
		t.Fatalf("disconnect=%s", out.String())
	}
	if _, err := Parse([]string{"cursor", "nope"}); err == nil {
		t.Fatal("unknown cursor action was accepted")
	}
}
