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

func TestRedactAccessTokenQuery(t *testing.T) {
	in := "https://api.example/x?access_token=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.aaa.bbb&n=1"
	out := Redact(in)
	if strings.Contains(out, "eyJ") || strings.Contains(out, "aaa.bbb") {
		t.Fatalf("leaked: %q", out)
	}
}

func TestRedactAuthorizationHeader(t *testing.T) {
	in := "cursor: Authorization: Bearer abc.def.ghi leaked"
	out := Redact(in)
	if strings.Contains(out, "abc.def.ghi") || strings.Contains(strings.ToLower(out), "bearer abc") {
		t.Fatalf("leaked: %q", out)
	}
}

func TestRedactXAPIKey(t *testing.T) {
	in := "claude: x-api-key: sk-ant-secretvalue999 leaked"
	out := Redact(in)
	if strings.Contains(out, "sk-ant-secretvalue999") {
		t.Fatalf("leaked: %q", out)
	}
}

func TestRedactOpenAIAndAnthropicHeaders(t *testing.T) {
	in := "cursor: openai-api-key: sess-abcdefghijklmnopqrstuvwxyz anthropic-api-key: my-ant-secret-value-here"
	out := Redact(in)
	if strings.Contains(out, "sess-abcdefghijklmnopqrstuvwxyz") || strings.Contains(out, "my-ant-secret-value-here") {
		t.Fatalf("leaked: %q", out)
	}
}

func TestRedactGoogAPIKeyHeader(t *testing.T) {
	in := "error: x-goog-api-key: AIzaSyTESTKEYNOTAREALSECRET000000 leaked"
	out := Redact(in)
	if strings.Contains(out, "AIzaSy") {
		t.Fatalf("leaked: %q", out)
	}
}

func TestRedactLeavesNormalErrors(t *testing.T) {
	in := "trae: 登录态在加密存储中，没有可读的 JWT 文件"
	if got := Redact(in); got != in {
		t.Fatalf("got %q", got)
	}
}
