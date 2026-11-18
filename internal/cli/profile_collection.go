package cli

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/rainhuang0220/whereToken/internal/adapter"
	"github.com/rainhuang0220/whereToken/internal/adapter/cursor"
	"github.com/rainhuang0220/whereToken/internal/publicprofile"
	"github.com/rainhuang0220/whereToken/internal/scan"
)

const defaultPagesProfile = "https://rainhuang0220.github.io/whereToken/profile/"

func mergeRefreshCollection(res scan.Result, prev *publicprofile.CollectionEvidence, now time.Time) *publicprofile.CollectionEvidence {
	current := res.Collection
	if current == nil {
		// Injected scans and legacy callers can report errors or events but
		// cannot prove a completed collection window.
		current = &publicprofile.CollectionEvidence{}
		for _, id := range failedAccountSources(res.Errors) {
			status, category := publicprofile.SourceUnavailable, "SOURCE_UNAVAILABLE"
			if id == "cursor" {
				for _, failure := range res.Errors {
					if failure == "cursor: "+cursor.KindOSPermission || failure == "cursor: OS_PERMISSION" {
						status, category = publicprofile.SourceAuthRequired, "AUTH"
					}
				}
			}
			current.Sources = append(current.Sources, publicprofile.SourceCheck{
				SourceCoverage: publicprofile.SourceCoverage{Source: id, Status: status},
				Category:       category, LastAttemptAt: now,
			})
		}
	}
	if prev == nil {
		return current
	}
	out := &publicprofile.CollectionEvidence{Sources: append([]publicprofile.SourceCheck(nil), current.Sources...)}
	for _, check := range prev.Sources {
		found := false
		for i := range out.Sources {
			fresh := &out.Sources[i]
			if fresh.Source == check.Source {
				found = true
				if fresh.LastObservationAt == nil {
					fresh.LastObservationAt = check.LastObservationAt
				}
				break
			}
		}
		if !found {
			check.Status, check.Category = publicprofile.SourceUnknown, ""
			out.Sources = append(out.Sources, check)
		}
	}
	return out
}

func snapshotSources(collection *publicprofile.CollectionEvidence, now time.Time) []publicprofile.SourceCoverage {
	checks, _ := collection.At(now)
	var out []publicprofile.SourceCoverage
	for _, check := range checks {
		if publicprofile.ValidateSourceCoverage(check.SourceCoverage) {
			out = append(out, check.SourceCoverage)
		}
	}
	return out
}

func (a *App) saveRefreshCollection(home adapter.Home, collection *publicprofile.CollectionEvidence, snap publicprofile.Snapshot, applied publicprofile.ApplyResult, pagesExpected bool) error {
	path := publicprofile.RefreshStatePath(home)
	st, _ := publicprofile.LoadRefreshState(path)
	st.Collection = collection
	p := &st.Publication
	p.PagesExpected = pagesExpected
	if t, err := time.Parse(time.RFC3339, snap.GeneratedAt); err == nil {
		p.SnapshotGeneratedAt, p.SnapshotAsOfDate = &t, snap.AsOfDate
	}
	if t := applied.HostedAcceptedAt; !t.IsZero() {
		p.HostedAcceptedAt = &t
	}
	if wall := applied.VerifiedWall; wall.Proven && !wall.PublishedAt.IsZero() {
		p.VerifiedAt, p.VerifiedAsOfDate, p.VerifiedSnapshotID = &wall.PublishedAt, wall.AsOfDate, wall.SnapshotID
	}
	p.Category = applied.FailureCategory
	switch {
	case applied.Auth:
		p.Category = "AUTH"
	case applied.Rejected || applied.Decision.Code == publicprofile.CodePendingGitHub:
		p.Category = "PUBLISH"
	case applied.Decision.Code == publicprofile.CodeBlockedData || applied.Decision.Code == publicprofile.CodeBlockedRegression:
		p.Category = "COVERAGE"
	}
	return publicprofile.SaveRefreshState(path, st)
}

type refreshHealth struct {
	Enabled             bool                              `json:"enabled"`
	Scheduler           string                            `json:"scheduler"`
	SchedulerHealthy    bool                              `json:"scheduler_healthy"`
	CollectionHealthy   bool                              `json:"collection_healthy"`
	DataFreshness       string                            `json:"data_freshness"`
	PublicationVerified bool                              `json:"publication_verified"`
	PagesMirrorHealthy  *bool                             `json:"pages_mirror_healthy,omitempty"`
	PagesMirror         string                            `json:"pages_mirror"`
	Overall             string                            `json:"overall_health"`
	Last                string                            `json:"last"`
	CheckedAt           time.Time                         `json:"checked_at,omitzero"`
	NextCheck           time.Time                         `json:"next_check,omitzero"`
	Sources             []publicprofile.SourceCheck       `json:"source_status,omitempty"`
	Publication         publicprofile.PublicationEvidence `json:"publication"`
	Plist               string                            `json:"plist,omitempty"`
	Attempts            int                               `json:"slot_attempts,omitempty"`
	ConsecutiveFailures int                               `json:"failure_backoff_level,omitempty"`
	RetryCategory       string                            `json:"retry_category,omitempty"`
	CursorAuthNoticeAt  time.Time                         `json:"cursor_auth_notice_at,omitzero"`
	DurationMS          int64                             `json:"last_duration_ms,omitempty"`
}

func (a *App) pagesMirrorStatus(p publicprofile.PublicationEvidence) string {
	if p.VerifiedSnapshotID == "" || p.VerifiedAsOfDate == "" {
		return "UNKNOWN"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	do := a.HTTPDo
	if do == nil {
		do = (&http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		}}).Do
	}
	for _, file := range []string{"manifest.json", "profile.json", "preview-light.svg", "preview-dark.svg"} {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, defaultPagesProfile+file, nil)
		if err != nil {
			return "UNKNOWN"
		}
		req.Header.Set("Cache-Control", "no-cache")
		resp, err := do(req)
		if err != nil {
			return "SOURCE_UNAVAILABLE"
		}
		matches := resp.StatusCode == http.StatusOK && pagesFileMatches(file, resp.Body, p)
		_ = resp.Body.Close()
		if !matches {
			if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNotFound {
				return "MISMATCH"
			}
			return "SOURCE_UNAVAILABLE"
		}
	}
	return "FRESH"
}

func pagesFileMatches(name string, body io.ReadCloser, p publicprofile.PublicationEvidence) bool {
	body = http.MaxBytesReader(nil, body, 2<<20)
	if strings.HasSuffix(name, ".json") {
		var obj struct {
			SnapshotID string `json:"snapshot_id"`
			AsOfDate   string `json:"as_of_date"`
		}
		return json.NewDecoder(body).Decode(&obj) == nil && obj.SnapshotID == p.VerifiedSnapshotID && obj.AsOfDate == p.VerifiedAsOfDate
	}
	var svg struct {
		XMLName xml.Name `xml:"svg"`
		Desc    string   `xml:"desc"`
		Meta    []struct {
			ID   string `xml:"id,attr"`
			Text string `xml:",chardata"`
		} `xml:"metadata"`
	}
	if xml.NewDecoder(body).Decode(&svg) != nil ||
		!strings.HasPrefix(svg.Desc, "Coding-agent token usage through "+p.VerifiedAsOfDate+".") {
		return false
	}
	for _, m := range svg.Meta {
		if m.ID == "wheretoken-snapshot-id" {
			return m.Text == p.VerifiedSnapshotID
		}
	}
	return false
}

func printRefreshCollection(checks []publicprofile.SourceCheck, doctor bool, print func(string, ...any)) {
	for _, check := range checks {
		var info strings.Builder
		fmt.Fprintf(&info, "%s=%s", check.Source, check.Status)
		if check.Category != "" {
			fmt.Fprintf(&info, " category=%s", check.Category)
		}
		if check.LastObservationAt != nil {
			fmt.Fprintf(&info, " last_observed=%s", check.LastObservationAt.UTC().Format(time.RFC3339))
		}
		if check.CoverageThroughUTC != "" {
			fmt.Fprintf(&info, " coverage_through_utc=%s", check.CoverageThroughUTC)
		}
		if doctor {
			for _, gap := range check.GapsUTC {
				fmt.Fprintf(&info, " gap_utc=%s..%s", gap.From, gap.To)
			}
		}
		print("source=%s\n", info.String())
		if doctor && check.Status == publicprofile.SourceAuthRequired && check.Source == "cursor" {
			print("cursor_recovery=Run wheretoken cursor connect in an interactive terminal. Sealed sessions require a foreground collection again when authorization is unavailable.\n")
		}
	}
}
