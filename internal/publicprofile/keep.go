package publicprofile

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
