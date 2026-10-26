package cli

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/rainhuang0220/whereToken/internal/adapter"
	"github.com/rainhuang0220/whereToken/internal/adapter/testhome"
	"github.com/rainhuang0220/whereToken/internal/event"
	"github.com/rainhuang0220/whereToken/internal/ledger"
	"github.com/rainhuang0220/whereToken/internal/publicprofile"
)

func TestRefreshCarriesAPermitAndClearsItOnlyAfterPublish(t *testing.T) {
	home := testhome.New(t.TempDir())
	path, err := ledger.PathFor(home)
	if err != nil {
		t.Fatal(err)
	}
	lg, err := ledger.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	orig := event.UsageEvent{
		Source: "codex", SessionID: "s", RequestID: "r", Model: "m", Vendor: "v",
		Timestamp: time.Date(2026, 10, 23, 0, 0, 0, 0, time.UTC),
		Miss:      10, Output: 1, Quality: event.QualityAuthoritative,
	}
	if err := lg.CommitUsage([]event.UsageEvent{orig}); err != nil {
		t.Fatal(err)
	}
	fixed := orig
	fixed.Miss = 4
	if _, err := lg.Correct(fixed, "dedup evidence", ledger.ParserVersion+1); err != nil {
		t.Fatal(err)
	}
	lg.Close()

	app := &App{}
	ctx := app.withCorrectionPermit(context.Background(), home)
	got := publicprofile.CorrectionFrom(ctx)
	if got.Reason != "dedup evidence" || got.Delta != 6 {
		t.Fatalf("carried %+v", got)
	}
	var reason, delta string
	respond := func(req *http.Request) (*http.Response, error) {
		reason = req.Header.Get("Wheretoken-Correction-Reason")
		delta = req.Header.Get("Wheretoken-Correction-Delta")
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"ok":true}`)), Header: make(http.Header)}, nil
	}
	if _, err := app.putSanitizedProjectionDo(context.Background(), "device", []byte(`{}`), respond); err != nil {
		t.Fatal(err)
	}
	if reason != "" || delta != "" {
		t.Fatalf("plain put sent %q %q", reason, delta)
	}
	if _, err := app.putSanitizedProjectionDo(ctx, "device", []byte(`{}`), respond); err != nil {
		t.Fatal(err)
	}
	if reason != "dedup evidence" || delta != "6" {
		t.Fatalf("headers %q %q", reason, delta)
	}
	app.clearPermitIfPublished(home, publicprofile.ApplyResult{Decision: publicprofile.Decision{Code: publicprofile.CodeBlockedRegression}})
	if _, _, ok, err := permitOf(t, home); err != nil || !ok {
		t.Fatalf("regression cleared the permit ok=%v err=%v", ok, err)
	}
	app.clearPermitIfPublished(home, publicprofile.ApplyResult{Transport: true, Decision: publicprofile.Decision{Code: publicprofile.CodeVerified}})
	if _, _, ok, _ := permitOf(t, home); !ok {
		t.Fatal("transport cleared the permit")
	}
	app.clearPermitIfPublished(home, publicprofile.ApplyResult{Decision: publicprofile.Decision{Code: publicprofile.CodeVerified}})
	if _, _, ok, err := permitOf(t, home); err != nil || ok {
		t.Fatalf("published permit ok=%v err=%v", ok, err)
	}
}

func permitOf(t *testing.T, home adapter.Home) (string, int64, bool, error) {
	t.Helper()
	path, err := ledger.PathFor(home)
	if err != nil {
		t.Fatal(err)
	}
	lg, err := ledger.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer lg.Close()
	return lg.CorrectionPermit()
}
