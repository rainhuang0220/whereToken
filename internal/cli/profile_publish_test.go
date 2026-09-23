package cli

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/rainhuang0220/whereToken/internal/adapter/testhome"
)

func TestProfilePublishParsesDryRunWithoutYes(t *testing.T) {
	flags, err := Parse([]string{"profile", "publish", "magenta"})
	if err != nil {
		t.Fatal(err)
	}
	if flags.ProfileAction != "publish" || flags.PublicPalette != "magenta" || flags.PublishYes {
		t.Fatalf("%+v", flags)
	}
	if _, err := Parse([]string{"profile", "publish", "--yes", "--dry-run", "newsprint"}); err == nil {
		t.Fatal("accepted both --yes and --dry-run")
	}
	if _, err := Parse([]string{"profile", "publish", "kiln"}); err == nil {
		t.Fatal("accepted kiln")
	}
	yes, err := Parse([]string{"profile", "publish", "--yes", "cobalt"})
	if err != nil {
		t.Fatal(err)
	}
	if !yes.PublishYes || yes.PublicPalette != "cobalt" {
		t.Fatalf("%+v", yes)
	}
}

func TestProfilePublishDryRunWithoutConfigDoesNotClaimSuccess(t *testing.T) {
	var stdout, stderr bytes.Buffer
	app := &App{Stdout: &stdout, Stderr: &stderr, Now: time.Now}
	code := app.runProfilePublish(Flags{ProfileAction: "publish", PublicPalette: "newsprint"}, testhome.New(t.TempDir()))
	if code == ExitOK {
		t.Fatal("missing config returned success")
	}
	text := stdout.String() + stderr.String()
	if strings.Contains(text, "PUBLISHED") || strings.Contains(text, "WAITING_USER_APPROVAL") {
		t.Fatal(text)
	}
	if !strings.Contains(text, "profile publish") && !strings.Contains(text, "blocker=") {
		t.Fatal(text)
	}
}
