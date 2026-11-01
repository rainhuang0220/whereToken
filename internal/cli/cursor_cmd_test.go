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
	if !strings.Contains(errb.String(), "操作系统保护的 Cursor 会话") {
		t.Fatalf("connect did not name the protected session:\n%s", errb.String())
	}
	if strings.Contains(out.String(), "eyJ") || strings.Contains(strings.ToLower(out.String()+errb.String()), "eyj") {
		t.Fatalf("status leaked a secret:\n%s", out.String())
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
	if !strings.Contains(out.String(), "cursor connect: off") || !strings.Contains(out.String(), "同意记录已删除") || !strings.Contains(out.String(), "系统里已经记住的允许还在") {
		t.Fatalf("disconnect=%s", out.String())
	}
	if strings.Contains(out.String(), "系统授权已撤销") || strings.Contains(out.String(), "钥匙串授权已删除") {
		t.Fatalf("disconnect claimed the operating system grant was removed:\n%s", out.String())
	}
	if _, err := Parse([]string{"cursor", "nope"}); err == nil {
		t.Fatal("unknown cursor action was accepted")
	}
}
