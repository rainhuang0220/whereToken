package publicprofile

import "testing"

func TestShouldReplaceProjectionKeepsHistory(t *testing.T) {
	complete := projectionWith("sha256:aa", StatusAvailable, agentRow("claude", 100, StatusAvailable, ""))
	updated := projectionWith("sha256:bb", StatusAvailable, agentRow("claude", 120, StatusAvailable, ""))
	partial := projectionWith("sha256:cc", StatusPartial, agentRow("claude", 100, StatusPartial, ""))
	restored := projectionWith("sha256:dd", StatusAvailable, agentRow("claude", 100, StatusAvailable, ""), agentRow("cursor", 50, StatusAvailable, ""))
	withCursor := projectionWith("sha256:ee", StatusPartial, agentRow("claude", 100, StatusAvailable, ""), agentRow("cursor", 900, StatusAvailable, ""))
	cursorDown := projectionWith("sha256:ff", StatusPartial, agentRow("claude", 100, StatusAvailable, ""), agentRow("cursor", 0, StatusUnavailable, ReasonAPIFailed))

	if !ShouldReplaceProjection(complete, updated) {
		t.Fatal("complete to complete update was rejected")
	}
	if ShouldReplaceProjection(complete, partial) {
		t.Fatal("complete to partial update replaced the public snapshot")
	}
	if !ShouldReplaceProjection(partial, restored) {
		t.Fatal("partial to complete update was rejected")
	}
	if ShouldReplaceProjection(withCursor, cursorDown) {
		t.Fatal("a provider API failure replaced measured public history")
	}
	if !ShouldReplaceProjection(Snapshot{}, complete) {
		t.Fatal("first snapshot was rejected")
	}
}

func TestUnmeasuredCursorDoesNotFreezeOtherAgents(t *testing.T) {
	prev := projectionWith("sha256:prev", StatusAvailable, agentRow("grok", 1_000, StatusAvailable, ""), agentRow("codex", 1_000, StatusAvailable, ""))
	next := projectionWith("sha256:next", StatusPartial,
		agentRow("grok", 3_000_000, StatusAvailable, ""),
		agentRow("codex", 2_000_000, StatusAvailable, ""),
		agentRow("cursor", 0, StatusUnavailable, ReasonAPIFailed),
	)
	if !ShouldReplaceProjection(prev, next) {
		t.Fatal("an unmeasured cursor row blocked grok and codex")
	}
}

func TestRetainedCursorObservationStaysReplaceable(t *testing.T) {
	const observed int64 = 8_700_453_509
	prev := projectionWith("sha256:fresh", StatusAvailable, agentRow("cursor", observed, StatusAvailable, ""))
	next := projectionWith("sha256:retained", StatusPartial, retainedAccountRow("cursor", observed))
	if !ShouldReplaceProjection(prev, next) {
		t.Fatal("a held account observation was treated as lost coverage")
	}
	shrunk := retainedAccountRow("cursor", observed-1)
	if ShouldReplaceProjection(prev, projectionWith("sha256:shrunk", StatusPartial, shrunk)) {
		t.Fatal("a smaller account observation replaced the public snapshot")
	}
	failed := agentRow("cursor", observed, StatusPartial, ReasonAPIFailed)
	failed.Coverage.TokenSource = TokenSourceAccountAPI
	if ShouldReplaceProjection(prev, projectionWith("sha256:failed", StatusPartial, failed)) {
		t.Fatal("a failed account row replaced a measured cursor total")
	}
}

func retainedAccountRow(id string, total int64) Breakdown {
	row := agentRow(id, total, StatusPartial, "")
	row.Coverage.TokenSource = TokenSourceAccountAPI
	to := "2026-10-23"
	row.Coverage.TokenWindow = &DateRange{To: to, Label: "account usage"}
	return row
}

func projectionWith(id, status string, agents ...Breakdown) Snapshot {
	return Snapshot{
		SnapshotID: id,
		DataStatus: status,
		Periods:    Periods{All: Period{ByAgent: agents}},
	}
}

func agentRow(id string, total int64, status, reason string) Breakdown {
	var value *int64
	if status != StatusUnavailable {
		n := total
		value = &n
	}
	return Breakdown{
		ID: id,
		Totals: Totals{Total: Component{
			Value:  value,
			Status: status,
		}},
		Coverage: Coverage{Tokens: status, Reason: reason},
	}
}
