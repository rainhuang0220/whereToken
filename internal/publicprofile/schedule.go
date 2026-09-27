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
