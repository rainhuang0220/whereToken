package publicprofile

import "time"

// NextRefreshWake is the next time a profile-refresh watch should scan.
// Ordinary checks stay on interval. A strictly later owner-local midnight
// that arrives sooner is the daily publication target. The returned instant
// is absolute. A sleeping host that misses it runs on the next wake and
// publishes only the latest valid local date.
func NextRefreshWake(now time.Time, loc *time.Location, interval time.Duration) time.Time {
	if interval <= 0 {
		interval = 15 * time.Minute
	}
	if loc == nil {
		loc = time.UTC
	}
	if now.IsZero() {
		now = time.Now()
	}
	next := now.Add(interval)
	local := now.In(loc)
	year, month, day := local.Date()
	midnight := time.Date(year, month, day, 0, 0, 0, 0, loc)
	if !midnight.After(now) {
		midnight = time.Date(year, month, day+1, 0, 0, 0, 0, loc)
	}
	if midnight.Before(next) {
		return midnight
	}
	return next
}

// FixedRefreshSlots defines owner-local scan checks. They are independent
// of the hosted GitHub publication gate (including verified-wall cadence,
// coverage and 0.01-unit growth); those protections remain unchanged.
var fixedRefreshSlots = [][2]int{{0, 0}, {10, 30}, {17, 0}}

// ScheduledRefreshSlot returns the newest 00:00, 10:30, or 17:00 owner-local
// slot at or before now. Go's local calendar rules handle 23/25-hour DST days.
func ScheduledRefreshSlot(now time.Time, loc *time.Location) time.Time {
	if loc == nil {
		loc = time.Local
	}
	if now.IsZero() {
		now = time.Now()
	}
	local := now.In(loc)
	y, m, d := local.Date()
	for i := len(fixedRefreshSlots) - 1; i >= 0; i-- {
		slot := fixedRefreshSlots[i]
		at := time.Date(y, m, d, slot[0], slot[1], 0, 0, loc)
		if !at.After(now) {
			return at
		}
	}
	return time.Date(y, m, d-1, 17, 0, 0, 0, loc)
}

// NextScheduledRefreshWake is strictly later than now. The watcher sleeps
// between these three instants, with at most one catch-up after macOS wakes.
func NextScheduledRefreshWake(now time.Time, loc *time.Location) time.Time {
	if loc == nil {
		loc = time.Local
	}
	if now.IsZero() {
		now = time.Now()
	}
	local := now.In(loc)
	y, m, d := local.Date()
	for _, slot := range fixedRefreshSlots {
		at := time.Date(y, m, d, slot[0], slot[1], 0, 0, loc)
		if at.After(now) {
			return at
		}
	}
	return time.Date(y, m, d+1, 0, 0, 0, 0, loc)
}
