package community

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
)

func TestCompetitionRankTiesShareBestAndSkip(t *testing.T) {
	tests := []struct {
		name   string
		scores []int64
		self   int64
		rank   int
		n      int
	}{
		{name: "100/100/80 first", scores: []int64{100, 100, 80}, self: 100, rank: 1, n: 3},
		{name: "100/100/80 third", scores: []int64{100, 100, 80}, self: 80, rank: 3, n: 3},
		{name: "unique top", scores: []int64{9, 5, 1}, self: 9, rank: 1, n: 3},
		{name: "unique mid", scores: []int64{9, 5, 1}, self: 5, rank: 2, n: 3},
		{name: "unique last", scores: []int64{9, 5, 1}, self: 1, rank: 3, n: 3},
		{name: "tied second", scores: []int64{100, 90, 90}, self: 90, rank: 2, n: 3},
		{name: "all tied", scores: []int64{50, 50, 50}, self: 50, rank: 1, n: 3},
		{name: "empty", scores: nil, self: 1, rank: 0, n: 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, n := CompetitionRank(tc.scores, tc.self)
			if got != tc.rank || n != tc.n {
				t.Fatalf("rank=%d n=%d want %d/%d", got, n, tc.rank, tc.n)
			}
		})
	}
}

func TestPercentileDefinition(t *testing.T) {
	tests := []struct {
		name string
		rank int
		n    int
		want float64
	}{
		{name: "first of 842", rank: 1, n: 842, want: 1},
		{name: "37 of 842", rank: 37, n: 842, want: 1 - float64(37-1)/842},
		{name: "second of 20 is not 0.90", rank: 2, n: 20, want: 0.95},
		{name: "last of 842", rank: 842, n: 842, want: 1.0 / 842.0},
		{name: "zero rank", rank: 0, n: 10, want: 0},
		{name: "zero n", rank: 1, n: 0, want: 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Percentile(tc.rank, tc.n)
			if math.Abs(got-tc.want) > 1e-12 {
				t.Fatalf("p=%v want %v (1-(rank-1)/n)", got, tc.want)
			}
		})
	}
}

func TestTopFractionMatchesCanonicalExample(t *testing.T) {
	got := TopFraction(37, 842)
	if math.Abs(got-37.0/842.0) > 1e-12 {
		t.Fatalf("%v", got)
	}
}

func TestFormatDisplayCanonical(t *testing.T) {
	tests := []struct {
		rank, n int
		want    string
	}{
		{37, 842, "#37 / 842"},
		{1, 3, "#1 / 3"},
		{0, 842, ""},
		{1, 0, ""},
		{-1, 10, ""},
	}
	for _, tc := range tests {
		name := tc.want
		if name == "" {
			name = "empty"
		}
		t.Run(name, func(t *testing.T) {
			if got := FormatDisplay(tc.rank, tc.n); got != tc.want {
				t.Fatalf("%q", got)
			}
		})
	}
}

func TestCaptionUnavailableIsEmDash(t *testing.T) {
	if Caption(Standing{Status: StatusUnavailable}) != "—" {
		t.Fatalf("%q", Caption(Standing{Status: StatusUnavailable}))
	}
	if Caption(Standing{Display: "#1 / 20", Rank: 1, Participants: 20}) != "#1 / 20" {
		t.Fatal("display")
	}
}

func TestFinishStandingBelowThresholdHidesRank(t *testing.T) {
	tests := []struct {
		name     string
		rank, n  int
		minN     int
		wantStat string
	}{
		{name: "three person podium", rank: 1, n: 3, minN: 20, wantStat: StatusInsufficientParticipants},
		{name: "nineteen of twenty", rank: 1, n: 19, minN: 20, wantStat: StatusInsufficientParticipants},
		{name: "single participant", rank: 1, n: 1, minN: 20, wantStat: StatusInsufficientParticipants},
		{name: "exactly twenty", rank: 1, n: 20, minN: 20, wantStat: StatusOK},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			st := FinishStanding(StatusOK, PeriodToday, MetricTokens, tc.rank, tc.n, tc.minN)
			if st.Status != tc.wantStat {
				t.Fatalf("status=%s want %s", st.Status, tc.wantStat)
			}
			if tc.wantStat == StatusInsufficientParticipants {
				if st.Rank != 0 || st.Display != "" || st.Percentile != nil {
					t.Fatalf("must not show a podium: %+v", st)
				}
				if strings.Contains(Caption(st), "#1") || strings.Contains(Caption(st), "#0") {
					t.Fatalf("caption=%q", Caption(st))
				}
				if Caption(st) != "—" {
					t.Fatalf("caption=%q", Caption(st))
				}
			}
			if st.Participants != tc.n {
				t.Fatalf("participants %+v", st)
			}
			if tc.wantStat == StatusOK && (st.Rank != tc.rank || st.Display != FormatDisplay(tc.rank, tc.n)) {
				t.Fatalf("ok %+v", st)
			}
		})
	}
}

func TestFinishStandingOK(t *testing.T) {
	st := FinishStanding(StatusOK, PeriodToday, MetricTokens, 37, 842, 20)
	if st.Status != StatusOK || st.Rank != 37 || st.Display != "#37 / 842" || st.Participants != 842 {
		t.Fatalf("%+v", st)
	}
	want := 1 - float64(37-1)/842
	if st.Percentile == nil || math.Abs(*st.Percentile-want) > 1e-12 {
		t.Fatalf("percentile %+v want %v", st.Percentile, want)
	}
}

func TestEmptyViewNeverZeroRank(t *testing.T) {
	v := EmptyView(StatusNetworkError, "")
	if v.Today.Rank != 0 || v.All.Rank != 0 || v.Today.Display != "" {
		t.Fatalf("%+v", v)
	}
	if v.Today.Status != StatusNetworkError || v.Note == "" {
		t.Fatalf("%+v", v)
	}
	raw, err := json.Marshal(v.Today)
	if err != nil {
		t.Fatal(err)
	}
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		t.Fatal(err)
	}
	if _, ok := obj["rank"]; ok {
		t.Fatalf("rank must be omitted: %s", raw)
	}
	if _, ok := obj["display"]; ok {
		t.Fatalf("display must be omitted: %s", raw)
	}
	if EmptyView(StatusServiceUnconfigured, "").Enabled {
		t.Fatal("unconfigured must not look opted-in")
	}
}

func TestSanitizeStandingDropsZeroPodium(t *testing.T) {
	tests := []struct {
		name    string
		in      Standing
		caption string
		rank    int
		display string
		status  string
	}{
		{
			name:    "hash zero of twenty",
			in:      Standing{Status: StatusOK, Rank: 0, Display: "#0 / 20"},
			caption: "—",
			status:  StatusNotRanked,
		},
		{
			name:    "hash zero bare",
			in:      Standing{Status: StatusNetworkError, Rank: 0, Display: "#0"},
			caption: "—",
			status:  StatusNetworkError,
		},
		{
			name:    "rank zero empty display",
			in:      Standing{Status: StatusOK, Rank: 0},
			caption: "—",
			status:  StatusNotRanked,
		},
		{
			name:    "display contains hash zero",
			in:      Standing{Status: StatusOK, Rank: 3, Display: "place #0 / 3"},
			caption: "—",
			status:  StatusNotRanked,
		},
		{
			name:    "real rank kept",
			in:      Standing{Status: StatusOK, Rank: 4, Participants: 20, Display: "#4 / 20"},
			caption: "#4 / 20",
			rank:    4,
			display: "#4 / 20",
			status:  StatusOK,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			st := SanitizeStanding(tc.in)
			if st.Status != tc.status || st.Rank != tc.rank || st.Display != tc.display {
				t.Fatalf("%+v", st)
			}
			if Caption(st) != tc.caption {
				t.Fatalf("caption=%q", Caption(st))
			}
			if strings.Contains(Caption(st), "#0") || strings.Contains(st.Display, "#0") {
				t.Fatalf("printed #0: %+v caption=%q", st, Caption(st))
			}
		})
	}
}
