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
