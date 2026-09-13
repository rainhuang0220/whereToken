package publicprofile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPagesWorkflowNeverPublishesDemoAsProduction(t *testing.T) {
	for _, rel := range []string{
		filepath.Join("..", "..", ".github", "workflows", "pages.yml"),
		filepath.Join("..", "..", "ci", "github-workflows", "pages.yml"),
	} {
		raw, err := os.ReadFile(rel)
		if err != nil {
			t.Fatal(err)
		}
		workflow := string(raw)
		if strings.Contains(workflow, "docs/media/public-profile-demo/. _pages/profile/") {
			t.Fatalf("%s deploys demo data as the production profile", rel)
		}
		for _, want := range []string{"public-profile/profile.json", "local_sanitized_snapshot", "_pages/profile-demo"} {
			if !strings.Contains(workflow, want) {
				t.Fatalf("%s missing production boundary %q", rel, want)
			}
		}
	}
}
