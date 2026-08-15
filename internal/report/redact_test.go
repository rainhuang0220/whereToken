package report

import (
	"strings"
	"testing"
)

func TestRedactNeverPrintsJWT(t *testing.T) {
	jwt := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0In0.abc"
	in := "trae: bearer " + jwt + " from storage"
	out := Redact(in)
	if strings.Contains(out, "eyJ") || strings.Contains(strings.ToLower(out), "bearer eyj") {
		t.Fatalf("leaked: %q", out)
	}
	if strings.Contains(out, jwt) {
		t.Fatalf("jwt still present: %q", out)
	}
}

func TestRedactAPIKeyAndHex(t *testing.T) {
	in := "cursor: sk-abcdefghijklmnopqrstuvwxyz1234 hex=0123456789abcdef0123456789abcdef0123456789"
	out := Redact(in)
	if strings.Contains(out, "sk-abcdefghijklmnopqrst") || strings.Contains(out, "0123456789abcdef0123456789abcdef") {
		t.Fatalf("leaked: %q", out)
	}
}

func TestRedactLeavesNormalErrors(t *testing.T) {
	in := "trae: 登录态在加密存储中，没有可读的 JWT 文件"
	if got := Redact(in); got != in {
		t.Fatalf("got %q", got)
	}
}
