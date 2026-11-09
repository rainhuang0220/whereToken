package cli

import "testing"

func TestCursorBackfillRequiresExplicitUTCRange(t *testing.T) {
	flags, err := Parse([]string{"cursor", "backfill", "--from", "2025-10-21", "--to", "2026-11-09"})
	if err != nil || flags.CursorAction != "backfill" || flags.From != "2025-10-21" || flags.To != "2026-11-09" {
		t.Fatalf("flags=%+v err=%v", flags, err)
	}
	for _, args := range [][]string{{"cursor", "backfill"}, {"cursor", "backfill", "--from", "2026-11-01", "--to", "2026-11-02", "--offline"}} {
		app, _, _ := testApp(args)
		if code := app.Run(); code != ExitUsage {
			t.Fatalf("args=%v code=%d", args, code)
		}
	}
}
