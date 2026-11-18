package publicprofile

import (
	"time"

	"github.com/rainhuang0220/whereToken/internal/adapter"
)

const (
	SourceFresh         = "FRESH"
	SourceStale         = "STALE"
	SourceAuthRequired  = "AUTH_REQUIRED"
	SourceUnavailable   = "SOURCE_UNAVAILABLE"
	SourceCoverageGap   = "COVERAGE_GAP"
	SourceConfirmedZero = "CONFIRMED_ZERO"
	SourceUnknown       = "UNKNOWN"
	CollectionMaxAge    = 24 * time.Hour
	dayLayout           = "2006-01-02"
)

// UTCGap is an inclusive range of unverified UTC days.
type UTCGap struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// SourceCoverage is the public subset of collection evidence. For local
// readers, a scan can succeed without proving calendar coverage.
type SourceCoverage struct {
	Source             string   `json:"source"`
	Status             string   `json:"status"`
	CoverageThroughUTC string   `json:"coverage_through_utc,omitempty"`
	GapsUTC            []UTCGap `json:"gaps_utc,omitempty"`
}

type SourceCheck struct {
	SourceCoverage
	Category          string     `json:"category,omitempty"`
	LastAttemptAt     time.Time  `json:"last_attempt_at"`
	LastObservationAt *time.Time `json:"last_successful_observation_at,omitempty"`
}

type CollectionEvidence struct {
	Sources []SourceCheck `json:"sources"`
}

func (c *CollectionEvidence) RetryCategory() string {
	if c == nil {
		return ""
	}
	for _, s := range c.Sources {
		switch s.Category {
		case "NETWORK", "RATE_LIMIT", "SOURCE_UNAVAILABLE", "COVERAGE", "LEDGER", "LOCAL_READ":
			return s.Category
		}
	}
	return ""
}

func ValidateSourceCoverage(s SourceCoverage) bool {
	if s.Source == "" || adapter.Label(s.Source) == s.Source {
		return false
	}
	switch s.Status {
	case SourceFresh, SourceStale, SourceAuthRequired, SourceUnavailable, SourceCoverageGap, SourceConfirmedZero, SourceUnknown:
	default:
		return false
	}
	if s.CoverageThroughUTC != "" && !validLocalDate(s.CoverageThroughUTC) {
		return false
	}
	if len(s.GapsUTC) > WallDays {
		return false
	}
	last := ""
	for _, gap := range s.GapsUTC {
		if !validLocalDate(gap.From) || !validLocalDate(gap.To) || gap.From > gap.To || gap.From <= last {
			return false
		}
		last = gap.To
	}
	return true
}

// At re-evaluates persisted evidence against the read clock.
func (c *CollectionEvidence) At(now time.Time) ([]SourceCheck, bool) {
	if c == nil || len(c.Sources) == 0 {
		return nil, false
	}
	out := append([]SourceCheck(nil), c.Sources...)
	healthy := true
	for i := range out {
		s := &out[i]
		if s.Status == SourceFresh || s.Status == SourceConfirmedZero {
			if s.LastObservationAt == nil || s.LastObservationAt.After(now) || now.Sub(*s.LastObservationAt) >= CollectionMaxAge {
				s.Status = SourceStale
			}
		}
		if s.Status != SourceFresh && s.Status != SourceConfirmedZero {
			healthy = false
		}
	}
	return out, healthy
}

func incompleteSource(s SourceCoverage) bool {
	return s.Status != SourceFresh && s.Status != SourceConfirmedZero
}

func hasCoverageGap(sources []SourceCoverage, from, to string, loc *time.Location) bool {
	for _, s := range sources {
		if !incompleteSource(s) {
			continue
		}
		// A failed scan without dated coverage leaves missing cells unknown.
		if len(s.GapsUTC) == 0 {
			return true
		}
		for _, gap := range s.GapsUTC {
			start, _ := time.Parse(dayLayout, gap.From)
			end, _ := time.Parse(dayLayout, gap.To)
			gapFrom := start.In(loc).Format(dayLayout)
			gapTo := end.AddDate(0, 0, 1).Add(-time.Nanosecond).In(loc).Format(dayLayout)
			if gapFrom <= to && (from == "" || gapTo >= from) {
				return true
			}
		}
	}
	return false
}

func hasAgentCoverageGap(sources []SourceCoverage, id, date string, loc *time.Location) bool {
	for i, source := range sources {
		if source.Source == id && hasCoverageGap(sources[i:i+1], date, date, loc) {
			return true
		}
	}
	return false
}

func incompleteCollection(sources []SourceCoverage) bool {
	for _, source := range sources {
		if incompleteSource(source) {
			return true
		}
	}
	return false
}

// measuredQuietZero is true only when every source that applies to this
// cell has an audited zero covering the date. No source evidence, a fresh
// read, or an unknown source is not a measured zero.
func measuredQuietZero(sources []SourceCoverage, agent, date string) bool {
	matched := false
	for _, source := range sources {
		if agent != "" && source.Source != agent {
			continue
		}
		matched = true
		if source.Status != SourceConfirmedZero || source.CoverageThroughUTC == "" || date > source.CoverageThroughUTC {
			return false
		}
		for _, gap := range source.GapsUTC {
			if gap.From <= date && date <= gap.To {
				return false
			}
		}
	}
	return matched
}
