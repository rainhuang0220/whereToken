package community

import (
	"math"
	"sync"
	"time"

	"github.com/rainhuang0220/whereToken/internal/price"
)

type dayRow struct {
	Tokens     int64
	CostUSD    *float64
	CostStatus string
	Version    string
	OffsetMin  int
	Updated    time.Time
}

// Store is the rank-service memory. It is not the local usage index.
type Store struct {
	mu   sync.Mutex
	days map[string]map[string]dayRow // participant -> period -> row
	hits map[string][]time.Time
	minN int
	now  func() time.Time
}

func NewStore(minN int) *Store {
	if minN <= 0 {
		minN = DefaultMinParticipants
	}
	return &Store{
		days: map[string]map[string]dayRow{},
		hits: map[string][]time.Time{},
		minN: minN,
		now:  time.Now,
	}
}

func (s *Store) MinParticipants() int { return s.minN }

func (s *Store) Put(u Upload) error {
	if err := ValidateUpload(u); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.rateLimitLocked(u.ParticipantID); err != nil {
		return err
	}
	byDay := s.days[u.ParticipantID]
	if byDay == nil {
		byDay = map[string]dayRow{}
		s.days[u.ParticipantID] = byDay
	}
	byDay[u.Period] = dayRow{
		Tokens:     u.Tokens,
		CostUSD:    u.EstimatedCostUSD,
		CostStatus: u.CostStatus,
		Version:    u.ClientVersion,
		OffsetMin:  u.UTCOffsetMinutes,
		Updated:    s.now(),
	}
	return nil
}

func (s *Store) Leave(id string) error {
	if !uuidRe.MatchString(id) {
		return errInvalidParticipant
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.days[id]; !ok {
		// Unknown ids stay never-seen; do not mint opted_out for random UUIDs.
		return nil
	}
	delete(s.days, id)
	delete(s.hits, id)
	// No tombstone: a "left" marker would be an oracle for whether a UUID
	// ever joined.
	return nil
}

func (s *Store) Rank(id, periodKind, periodDate, metric string) Standing {
	if metric == "" {
		metric = MetricTokens
	}
	if periodKind == "" {
		periodKind = PeriodToday
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	// A departed id must rank identically to a never-seen id. Returning
	// opted_out here would let anyone probe whether a UUID once joined.
	if metric == MetricCost && periodKind == PeriodAll {
		st := FinishStanding(StatusUnavailable, periodKind, metric, 0, 0, s.minN)
		st.Note = "All-time estimated-cost rank is unavailable until historical pricing is archived."
		return st
	}

	var scores []int64
	var self int64
	have := false
	for pid, days := range s.days {
		val, ok := scoreOf(days, periodKind, periodDate, metric)
		if !ok || val <= 0 {
			continue
		}
		scores = append(scores, val)
		if pid == id {
			self = val
			have = true
		}
	}
	n := len(scores)
	if n < s.minN {
		return FinishStanding(StatusInsufficientParticipants, periodKind, metric, 0, n, s.minN)
	}
	if !have {
		return FinishStanding(StatusNotRanked, periodKind, metric, 0, n, s.minN)
	}
	rank, _ := CompetitionRank(scores, self)
	return FinishStanding(StatusOK, periodKind, metric, rank, n, s.minN)
}

func scoreOf(days map[string]dayRow, periodKind, periodDate, metric string) (int64, bool) {
	if periodKind == PeriodAll {
		var sum int64
		any := false
		for _, row := range days {
			v, ok := rowValue(row, metric)
			if !ok || v <= 0 {
				if metric == MetricCost {
					return 0, false
				}
				continue
			}
			sum += v
			any = true
		}
		return sum, any && sum > 0
	}
	row, ok := days[periodDate]
	if !ok {
		return 0, false
	}
	return rowValue(row, metric)
}

func rowValue(row dayRow, metric string) (int64, bool) {
	if metric == MetricCost {
		if row.CostStatus != price.StatusComplete || row.CostUSD == nil {
			return 0, false
		}
		// micro-dollars so ranking stays integer; round so 0.1 does not become 99999.
		micro := int64(math.Round(*row.CostUSD * 1e6))
		return micro, micro > 0
	}
	return row.Tokens, row.Tokens > 0
}

const (
	maxPutsPerHour = 30
	minPutGap      = 2 * time.Second
)

func (s *Store) rateLimitLocked(id string) error {
	now := s.now()
	cut := now.Add(-time.Hour)
	kept := s.hits[id][:0]
	for _, t := range s.hits[id] {
		if t.After(cut) {
			kept = append(kept, t)
		}
	}
	if len(kept) >= maxPutsPerHour {
		return errRateLimited
	}
	if len(kept) > 0 && now.Sub(kept[len(kept)-1]) < minPutGap {
		// treat as a replacement of the same snapshot: allow overwrite
		// only when the last write is recent AND we already have a row.
		// Still count as one hit window; skip adding another stamp.
		s.hits[id] = kept
		return nil
	}
	s.hits[id] = append(kept, now)
	return nil
}

func (s *Store) ParticipantCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.days)
}
