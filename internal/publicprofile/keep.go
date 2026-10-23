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
	if coverageRank(next.DataStatus) < coverageRank(prev.DataStatus) {
		return false
	}
	return !lostMeasuredProvider(prev, next)
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
