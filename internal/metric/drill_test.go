package metric

import (
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
}

func sliceSum(rows []Slice) int64 {
	var n int64
	for _, s := range rows {
		n += s.Total()
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
