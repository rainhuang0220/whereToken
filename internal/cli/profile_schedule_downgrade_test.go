package cli

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/rainhuang0220/whereToken/internal/adapter/testhome"
	"github.com/rainhuang0220/whereToken/internal/ledger"
)

func TestScheduleDowngradeKeepsEvidenceAndStaysRunnable(t *testing.T) {
	home := testhome.New(t.TempDir())
	now := time.Date(2026, 11, 18, 17, 30, 0, 0, time.UTC)
	original := profileRefreshSchedule{
		Slot: now.Add(-30 * time.Minute), Attempts: 8, Failures: 4, Done: false,
		RetryAt: now.Add(4 * time.Hour), LastRetryReason: "NETWORK",
	}
	raw, err := json.Marshal(original)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dirOf(profileRefreshSchedulePath(home)), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(profileRefreshSchedulePath(home), append(raw, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	app, out, errb := testApp([]string{"profile", "refresh", "prepare-downgrade"})
	app.Home = home
	app.Now = func() time.Time { return now }
	app.Loc = time.UTC
	if code := app.Run(); code != ExitOK {
		t.Fatalf("prepare %d %s", code, errb.String())
	}
	savedRaw, err := os.ReadFile(profileRefreshSchedulePath(home))
	if err != nil {
		t.Fatal(err)
	}
	if !legacyV0719Accepts(savedRaw) {
		t.Fatalf("v0.7.19 would reject the schedule: %s", savedRaw)
	}
	saved := readScheduleFile(t, profileRefreshSchedulePath(home))
	if saved.Attempts != legacyProfileRefreshAttempts || saved.Failures != 4 || saved.LastRetryReason != "NETWORK" {
		t.Fatalf("schedule lost the outage %+v", saved)
	}
	evidence := readScheduleFile(t, profileRefreshEvidencePath(home))
	if evidence.Failures != 4 || evidence.LastRetryReason != "NETWORK" {
		t.Fatalf("evidence %+v", evidence)
	}
	backup, err := os.ReadFile(profileRefreshSchedulePath(home) + ".pre-downgrade")
	if err != nil {
		t.Fatal(err)
	}
	var backed profileRefreshSchedule
	if err := json.Unmarshal(backup, &backed); err != nil || backed.Attempts != 8 {
		t.Fatalf("backup %+v %v", backed, err)
	}
	legacy := profileRefreshSchedule{Slot: saved.Slot, Attempts: saved.Attempts, RetryAt: saved.RetryAt, Done: saved.Done}
	legacyRaw, err := json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(profileRefreshSchedulePath(home), append(legacyRaw, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	after := readScheduleFile(t, profileRefreshEvidencePath(home))
	if after.Failures != 4 || after.LastRetryReason != "NETWORK" {
		t.Fatalf("legacy rewrite erased evidence %+v", after)
	}
	ledgerFile, err := ledger.PathFor(home)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(ledgerFile); !os.IsNotExist(err) {
		t.Fatalf("downgrade touched the ledger: %v", err)
	}
	if !strings.Contains(out.String(), "downgrade_ready=yes") || !strings.Contains(out.String(), "ledger_untouched=yes") || !strings.Contains(out.String(), "schedule_attempts=3") {
		t.Fatalf("output %s", out.String())
	}
}

func legacyV0719Accepts(raw []byte) bool {
	var st struct {
		Attempts int       `json:"attempts"`
		Slot     time.Time `json:"slot"`
	}
	if err := json.Unmarshal(raw, &st); err != nil || st.Slot.IsZero() || st.Attempts < 0 || st.Attempts > 3 {
		return false
	}
	return true
}

func readScheduleFile(t *testing.T, path string) profileRefreshSchedule {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var st profileRefreshSchedule
	if err := json.Unmarshal(raw, &st); err != nil {
		t.Fatal(err)
	}
	return st
}

func dirOf(path string) string {
	i := strings.LastIndex(path, "/")
	if i < 0 {
		return "."
	}
	return path[:i]
}
