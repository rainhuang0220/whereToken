package metric

import (
	"encoding/json"
	"testing"

	"github.com/rainhuang0220/whereToken/internal/event"
)

func TestAggregateDrillConservesAcrossModelWorkspaceSession(t *testing.T) {
	events := []event.UsageEvent{
		{Source: "claude", Vendor: "anthropic", Model: "opus", Workspace: "/a", SessionID: "s1", RequestID: "1", Miss: 100, Output: 10},
		{Source: "claude", Vendor: "minimax", Model: "m3", Workspace: "/a", SessionID: "s2", RequestID: "2", Miss: 50, Output: 5},
		{Source: "kimi", Vendor: "moonshot", Model: "k2", Workspace: "/b", SessionID: "s3", RequestID: "3", Miss: 20, Output: 3},
	}
	sum := Aggregate(events, []event.TurnEvent{
		{Source: "claude", SessionID: "s1", Workspace: "/a"},
		{Source: "kimi", SessionID: "s3", Workspace: "/b"},
	})
	all := sum.All.Total()
	if got := sliceSum(sum.DrillAll.Models); got != all {
		t.Fatalf("models=%d all=%d", got, all)
	}
	if got := sliceSum(sum.DrillAll.Workspaces); got != all {
		t.Fatalf("workspaces=%d all=%d", got, all)
	}
	if got := sessionSum(sum.DrillAll.Sessions); got != all {
		t.Fatalf("sessions=%d all=%d", got, all)
	}
	claude := sliceByID(sum.BySource, "claude")
	if claude == nil {
		t.Fatal("missing claude source")
	}
	pack := sum.DrillBySource["claude"]
	if got := sliceSum(pack.Models); got != claude.Total() {
		t.Fatalf("claude models=%d source=%d", got, claude.Total())
	}
	mm := sliceByID(sum.ByVendor, "minimax")
	if mm == nil {
		t.Fatal("missing minimax vendor")
	}
	if got := sliceSum(sum.DrillByVendor["minimax"].Models); got != mm.Total() {
		t.Fatalf("minimax models=%d vendor=%d", got, mm.Total())
	}
	wsA := sliceByID(sum.DrillAll.Workspaces, "/a")
	if wsA == nil || wsA.Total() != 100+10+50+5 {
		t.Fatalf("workspace /a %+v", wsA)
	}
	var s1 *SessionSlice
	for i := range sum.DrillAll.Sessions {
		if sum.DrillAll.Sessions[i].ID == "s1" {
			s1 = &sum.DrillAll.Sessions[i]
			break
		}
	}
	if s1 == nil || s1.Total() != 110 || s1.UserTurns != 1 || s1.Model != "opus" {
		t.Fatalf("session s1 %+v", s1)
	}
	if got := sliceRequests(sum.DrillAll.Models); got != sum.All.Requests {
		t.Fatalf("model requests=%d all=%d", got, sum.All.Requests)
	}
	if got := sliceCost(sum.DrillAll.Models); got != sum.All.CostMicro {
		t.Fatalf("model cost=%d all=%d", got, sum.All.CostMicro)
	}
}

func TestVendorDrillSessionsKeepUserTurns(t *testing.T) {
	events := []event.UsageEvent{
		{Source: "claude", Vendor: "anthropic", Model: "opus", SessionID: "s1", RequestID: "1", Miss: 10},
	}
	sum := Aggregate(events, []event.TurnEvent{{Source: "claude", SessionID: "s1"}})
	pack := sum.DrillByVendor["anthropic"]
	if len(pack.Sessions) != 1 || pack.Sessions[0].UserTurns != 1 {
		t.Fatalf("vendor session turns %+v", pack.Sessions)
	}
}

func TestViewDrillOmitsUnavailableCostUSD(t *testing.T) {
	sum := Aggregate([]event.UsageEvent{
		{Source: "claude", Vendor: "anthropic", Model: "claude-opus-4.6", RequestID: "a", SessionID: "s1", Miss: 1_000_000, Output: 1_000_000},
		{Source: "kimi", Vendor: "moonshot", Model: "k3", RequestID: "b", SessionID: "s2", Miss: 100, Output: 10},
	}, nil)
	raw, err := json.Marshal(ViewDrill(sum.DrillAll))
	if err != nil {
		t.Fatal(err)
	}
	var parsed struct {
		Models []map[string]any `json:"models"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatal(err)
	}
	byID := map[string]map[string]any{}
	for _, m := range parsed.Models {
		id, _ := m["id"].(string)
		byID[id] = m
	}
	opus := byID["claude-opus-4.6"]
	if opus == nil || opus["cost_usd"] != "$30.0000" || opus["cost_status"] != "complete" {
		t.Fatalf("priced model %+v", opus)
	}
	k3 := byID["k3"]
	if k3 == nil {
		t.Fatal("missing k3")
	}
	if _, ok := k3["cost_usd"]; ok {
		t.Fatalf("unavailable drill slice must omit cost_usd: %v", k3)
	}
	if k3["cost_status"] != "unavailable" {
		t.Fatalf("k3 status=%v", k3["cost_status"])
	}
	kimi := ViewDrill(sum.DrillBySource["kimi"])
	if len(kimi.Models) != 1 || kimi.Models[0].CostUSD != "" || kimi.Models[0].CostStatus != "unavailable" {
		t.Fatalf("kimi models %+v", kimi.Models)
	}
}

func TestDrillDoesNotDoubleCountReasoning(t *testing.T) {
	events := []event.UsageEvent{
		{Source: "minimax", Vendor: "minimax", Model: "minimax-m2.5", RequestID: "1", SessionID: "s1", Miss: 100, Output: 8, Reasoning: 2},
		{Source: "codex", Vendor: "openai", Model: "gpt-5", RequestID: "2", SessionID: "s2", Miss: 10, Output: 12, Reasoning: 2},
	}
	sum := Aggregate(events, nil)
	want := int64(100 + 8 + 10 + 12)
	if sum.All.Total() != want {
		t.Fatalf("all=%d want %d (reasoning is not a sixth addend)", sum.All.Total(), want)
	}
	if got := sliceSum(sum.DrillAll.Models); got != want {
		t.Fatalf("models=%d all=%d", got, want)
	}
	if got := sliceSum(sum.DrillAll.Workspaces); got != want {
		t.Fatalf("workspaces=%d all=%d", got, want)
	}
	if got := sessionSum(sum.DrillAll.Sessions); got != want {
		t.Fatalf("sessions=%d all=%d", got, want)
	}
	m25 := sliceByID(sum.DrillAll.Models, "minimax-m2.5")
	if m25 == nil || m25.Total() != 108 || m25.Output != 8 {
		t.Fatalf("m2.5 %+v", m25)
	}
	gpt := sliceByID(sum.DrillAll.Models, "gpt-5")
	if gpt == nil || gpt.Total() != 22 || gpt.Output != 12 {
		t.Fatalf("gpt-5 %+v", gpt)
	}
	priced := Aggregate([]event.UsageEvent{
		{Source: "minimax", Vendor: "minimax", Model: "minimax-m2.5", RequestID: "1", Output: 1_000_000, Reasoning: 1_000_000},
	}, nil)
	row := sliceByID(priced.DrillAll.Models, "minimax-m2.5")
	if row == nil || row.Total() != 1_000_000 || row.CostMicro != 1_200_000 {
		t.Fatalf("reasoning must not be charged again %+v", row)
	}
}

func TestUnlabeledDrillID(t *testing.T) {
	if !UnlabeledDrillID(unlabeledModel) || !UnlabeledDrillID(unlabeledWorkspace) || !UnlabeledDrillID(unlabeledSession) {
		t.Fatal("fallback buckets")
	}
	if UnlabeledDrillID("opus") || UnlabeledDrillID("") {
		t.Fatal("named and empty ids are not fallback buckets")
	}
}

func sliceSum(rows []Slice) int64 {
	var n int64
	for _, s := range rows {
		n += s.Total()
	}
	return n
}

func sliceRequests(rows []Slice) int64 {
	var n int64
	for _, s := range rows {
		n += s.Requests
	}
	return n
}

func sliceCost(rows []Slice) int64 {
	var n int64
	for _, s := range rows {
		n += s.CostMicro
	}
	return n
}

func sessionSum(rows []SessionSlice) int64 {
	var n int64
	for _, s := range rows {
		n += s.Total()
	}
	return n
}

func sliceByID(rows []Slice, id string) *Slice {
	for i := range rows {
		if rows[i].ID == id {
			return &rows[i]
		}
	}
	return nil
}

func TestDrillUnknownCostOmitsUSD(t *testing.T) {
	sum := Aggregate([]event.UsageEvent{
		{Source: "kimi", Vendor: "moonshot", Model: "k3", SessionID: "s", RequestID: "r", Miss: 1000, Output: 10},
	}, nil)
	if sum.All.CostStatus != "unavailable" {
		t.Fatalf("all %+v", sum.All)
	}
	v := ViewDrill(sum.DrillAll)
	if len(v.Models) == 0 {
		t.Fatal("no models")
	}
	if v.Models[0].CostUSD != "" || v.Models[0].CostStatus != "unavailable" {
		t.Fatalf("model drill must omit $0: %+v", v.Models[0])
	}
	if len(v.Sessions) == 0 {
		t.Fatal("no sessions")
	}
	if v.Sessions[0].CostUSD != "" {
		t.Fatalf("session drill must omit $0: %+v", v.Sessions[0])
	}
}
