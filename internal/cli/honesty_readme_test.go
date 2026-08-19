package cli

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

func englishREADME(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	raw, err := os.ReadFile(filepath.Join(filepath.Dir(file), "..", "..", "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func TestEnglishREADMEHonesty(t *testing.T) {
	rs := englishREADME(t)

	if !strings.Contains(rs, "not on the npm registry") && !strings.Contains(rs, "npm not on registry") {
		t.Fatal(`English README must keep "npm not on registry"`)
	}
	if !strings.Contains(rs, "curl") || !strings.Contains(rs, "| bash") {
		t.Fatal("English README must keep the curl|bash install")
	}
	if !strings.Contains(strings.ToLower(rs), "irm") || !strings.Contains(rs, "| iex") {
		t.Fatal("English README must keep the irm|iex install")
	}
	if !strings.Contains(rs, "brew tap rainhuang0220/wheretoken") {
		t.Fatal("English README must keep brew tap rainhuang0220/wheretoken")
	}
	if !strings.Contains(rs, "unsigned") {
		t.Fatal("English README must say release binaries are unsigned")
	}
	if !strings.Contains(rs, "signed in") {
		t.Fatal("English README must say some integrations need the app signed in")
	}

	if strings.Contains(rs, "does not estimate cost") {
		t.Fatal("English README must not claim the product does not estimate cost")
	}
	if !strings.Contains(rs, "does not upload") {
		t.Fatal("English README must say Community Rank does not upload prompts, paths, and the rest")
	}
	for _, want := range []string{
		"worldwide",
		"all-AI-users",
		"remote deploy blocker",
		"never sent as $0",
		"not a global",
		"DO_NOT_TRACK",
		"days this client uploaded",
	} {
		if !strings.Contains(rs, want) {
			t.Errorf("English README missing honesty %q", want)
		}
	}
	if regexp.MustCompile(`WHERETOKEN_COMMUNITY_URL\s*=\s*https?://`).MatchString(rs) {
		t.Fatal("English README must not ship a public WHERETOKEN_COMMUNITY_URL")
	}
	for _, bad := range []string{"rank.wheretoken.", "community.wheretoken.", "https://wheretoken.com"} {
		if strings.Contains(rs, bad) {
			t.Fatalf("English README invented a public rank URL %q", bad)
		}
	}
}

func TestChineseREADMEHonesty(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	raw, err := os.ReadFile(filepath.Join(filepath.Dir(file), "..", "..", "README.zh-CN.md"))
	if err != nil {
		t.Fatal(err)
	}
	rs := string(raw)
	for _, want := range []string{
		"不会写成 $0",
		"远程部署阻塞项",
		"全体 AI 用户",
		"DO_NOT_TRACK",
		"这台客户端上传过的那些天",
		"窑墙「全部」",
	} {
		if !strings.Contains(rs, want) {
			t.Errorf("Chinese README missing honesty %q", want)
		}
	}
	if regexp.MustCompile(`WHERETOKEN_COMMUNITY_URL\s*=\s*https?://`).MatchString(rs) {
		t.Fatal("Chinese README must not ship a public WHERETOKEN_COMMUNITY_URL")
	}
	for _, bad := range []string{"rank.wheretoken.", "community.wheretoken.", "https://wheretoken.com"} {
		if strings.Contains(rs, bad) {
			t.Fatalf("Chinese README invented a public rank URL %q", bad)
		}
	}
}

func TestHelpHonestyCopy(t *testing.T) {
	h := HelpText()
	for _, want := range []string{
		"not a subscription bill",
		"worldwide",
		"all-AI-users",
		"unsigned",
		"brew tap rainhuang0220/wheretoken",
		"never $0",
		"never #0",
		"request ids",
		"uploaded days",
		"kiln 全部",
	} {
		if !strings.Contains(h, want) {
			t.Errorf("help missing honesty %q", want)
		}
	}
	if strings.Contains(h, "does not estimate cost") {
		t.Fatal("help must not claim the product does not estimate cost")
	}
}
