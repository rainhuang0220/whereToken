package profilewebembed

import (
	"strings"
	"testing"
)

func TestLivePageHasNoDashboardAPIs(t *testing.T) {
	js, err := Read("assets/profile.js")
	if err != nil {
		t.Fatal(err)
	}
	s := string(js)
	for _, bad := range []string{"/api/summary", "/api/v1/", "wheretoken login", "wheretoken sync", "community"} {
		if strings.Contains(s, bad) {
			t.Fatalf("forbidden %q", bad)
		}
	}
	if !strings.Contains(s, "./profile.json") {
		t.Fatal("must fetch ./profile.json")
	}
}
