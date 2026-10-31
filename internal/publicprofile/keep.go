package publicprofile

import "sort"

// ShouldReplaceProjection reports whether a newly uploaded snapshot may
// replace the public one. A weaker coverage status, or a provider that
// previously had measured tokens and is now missing or failed, keeps the
// current projection. The hosted ledger and the README are left unchanged.
func ShouldReplaceProjection(prev, next Snapshot) bool {
	if prev.SnapshotID == "" {
		return true
	}
	if lostMeasuredProvider(prev, next) {
		return false
	}
	// A weaker snapshot status still blocks when a measured agent itself got
	// weaker. A held account observation may display as partial, and a newly
	// unmeasured agent must not freeze every other agent.
	if coverageRank(next.DataStatus) < coverageRank(prev.DataStatus) && !measuredSetHeld(prev, next) {
		return false
	}
	return true
}

// measuredSetHeld reports that every previously measured agent is still
// measured, and that any drop in its token status is a held account
// observation rather than a weaker reading of that agent.
func measuredSetHeld(prev, next Snapshot) bool {
	nextAgents := make(map[string]Breakdown, len(next.Periods.All.ByAgent))
	for _, row := range next.Periods.All.ByAgent {
		nextAgents[row.ID] = row
	}
	saw := false
	for _, row := range prev.Periods.All.ByAgent {
		if !measuredAgent(row) {
			continue
		}
		saw = true
		nxt, ok := nextAgents[row.ID]
		if !ok || !measuredAgent(nxt) {
			return false
		}
		if coverageRank(nxt.Coverage.Tokens) < coverageRank(row.Coverage.Tokens) ||
			coverageRank(nxt.Totals.Total.Status) < coverageRank(row.Totals.Total.Status) {
			if !heldAccountObservation(row, nxt) {
				return false
			}
		}
	}
	return saw
}

// heldAccountObservation is a reused account measurement: same source, no
// failure reason, and a total that did not shrink. It is not a fresh pull
// and it is not local tokenCount.
func heldAccountObservation(prev, next Breakdown) bool {
	if next.Coverage.TokenSource != TokenSourceAccountAPI || next.Coverage.Reason != "" {
		return false
	}
	if next.Coverage.Tokens != StatusPartial {
		return false
	}
	win := next.Coverage.TokenWindow
	if win == nil || win.Label != "account usage" || win.To == "" {
		return false
	}
	if prev.Totals.Total.Value == nil || next.Totals.Total.Value == nil {
		return false
	}
	return *next.Totals.Total.Value >= *prev.Totals.Total.Value
}

func coverageRank(status string) int {
	switch status {
	case StatusAvailable:
		return 3
	case StatusPartial:
		return 2
	default:
		return 1
	}
}

// lostProviderIDs lists catalog ids that were measured and are now missing
// or failed. The result is safe to print: lowercase agent ids, comma separated.
func lostProviderIDs(prev, next Snapshot) string {
	nextAgents := make(map[string]Breakdown, len(next.Periods.All.ByAgent))
	for _, row := range next.Periods.All.ByAgent {
		nextAgents[row.ID] = row
	}
	var ids []string
	for _, row := range prev.Periods.All.ByAgent {
		if !measuredAgent(row) || !safeAgentID(row.ID) {
			continue
		}
		nxt, ok := nextAgents[row.ID]
		if ok && measuredAgent(nxt) {
			continue
		}
		ids = append(ids, row.ID)
	}
	sort.Strings(ids)
	out := ""
	for i, id := range ids {
		if i > 0 {
			out += ","
		}
		out += id
	}
	return out
}

func safeAgentID(id string) bool {
	if id == "" {
		return false
	}
	for _, r := range id {
		if r < 'a' || r > 'z' {
			if r < '0' || r > '9' {
				return false
			}
		}
	}
	return true
}

func lostMeasuredProvider(prev, next Snapshot) bool {
	nextAgents := make(map[string]Breakdown, len(next.Periods.All.ByAgent))
	for _, row := range next.Periods.All.ByAgent {
		nextAgents[row.ID] = row
	}
	for _, row := range prev.Periods.All.ByAgent {
		if !measuredAgent(row) {
			continue
		}
		nxt, ok := nextAgents[row.ID]
		if !ok || !measuredAgent(nxt) {
			return true
		}
	}
	return false
}

func measuredAgent(row Breakdown) bool {
	if row.Totals.Total.Status == StatusUnavailable || row.Coverage.Tokens == StatusUnavailable {
		return false
	}
	switch row.Coverage.Reason {
	case ReasonAPIFailed, ReasonAuthMissing, ReasonAccountAPISkipped:
		return false
	}
	return row.Totals.Total.Value != nil && *row.Totals.Total.Value > 0
}
