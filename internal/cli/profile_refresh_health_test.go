package cli

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rainhuang0220/whereToken/internal/adapter/testhome"
	"github.com/rainhuang0220/whereToken/internal/publicprofile"
)

func trustedCollection(at time.Time) *publicprofile.CollectionEvidence {
	observed := at.Add(-time.Hour)
	return &publicprofile.CollectionEvidence{Sources: []publicprofile.SourceCheck{{
		SourceCoverage: publicprofile.SourceCoverage{
			Source: "cursor", Status: publicprofile.SourceFresh, CoverageThroughUTC: "2026-11-18",
		},
		LastAttemptAt: at, LastObservationAt: &observed,
	}}}
}

func TestRefreshDataHealthRequiresTrustedEvidence(t *testing.T) {
	now := time.Date(2026, 11, 18, 18, 0, 0, 0, time.FixedZone("CST", 8*3600))
	recent := now.Add(-time.Hour)
	expired := now.Add(-30 * time.Hour)
	cases := []struct {
		name    string
		state   publicprofile.RefreshState
		raw     string
		data    string
		healthy string
		fail    bool
	}{
		{
			name:    "verified without collection",
			state:   publicprofile.RefreshState{Phase: publicprofile.PhasePublished, Code: publicprofile.CodeVerified, CheckedAt: recent},
			data:    "unknown",
			healthy: "no",
		},
		{
			name:    "offline without collection",
			state:   publicprofile.RefreshState{Phase: publicprofile.PhaseSkipped, Code: publicprofile.CodeOffline, CheckedAt: recent},
			data:    "unknown",
			healthy: "no",
		},
		{
			name:    "cooldown without collection",
			state:   publicprofile.RefreshState{Phase: publicprofile.PhaseSkipped, Code: publicprofile.CodeSkippedCooldown, CheckedAt: recent},
			data:    "unknown",
			healthy: "no",
		},
		{
			name:    "delta without collection",
			state:   publicprofile.RefreshState{Phase: publicprofile.PhaseSkipped, Code: publicprofile.CodeSkippedDelta, CheckedAt: recent},
			data:    "unknown",
			healthy: "no",
		},
		{
			name:    "old file has no check time",
			raw:     "{\"schema_version\":1,\"phase\":\"published\",\"code\":\"VERIFIED\"}\n",
			data:    "unknown",
			healthy: "no",
		},
		{
			name: "expired check is not healthy",
			state: publicprofile.RefreshState{
				Phase: publicprofile.PhasePublished, Code: publicprofile.CodeVerified,
				CheckedAt: expired, Collection: trustedCollection(expired),
			},
			data: "no", healthy: "no", fail: true,
		},
		{
			name: "offline keeps a fresh collection from looking proven",
			state: publicprofile.RefreshState{
				Phase: publicprofile.PhaseSkipped, Code: publicprofile.CodeOffline,
				CheckedAt: recent, Collection: trustedCollection(recent),
			},
			data: "unknown", healthy: "no",
		},
		{
			name: "verified collection is healthy",
			state: publicprofile.RefreshState{
				Phase: publicprofile.PhasePublished, Code: publicprofile.CodeVerified,
				CheckedAt: recent, Collection: trustedCollection(recent),
			},
			data: "yes", healthy: "yes",
		},
		{
			name: "quiet delta with fresh evidence stays healthy",
			state: publicprofile.RefreshState{
				Phase: publicprofile.PhaseSkipped, Code: publicprofile.CodeSkippedDelta,
				CheckedAt: recent, Collection: trustedCollection(recent),
			},
			data: "yes", healthy: "yes",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			home := testhome.New(t.TempDir())
			path := publicprofile.RefreshStatePath(home)
			if tc.raw != "" {
				if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(tc.raw), 0o600); err != nil {
					t.Fatal(err)
				}
			} else if err := publicprofile.SaveRefreshState(path, tc.state); err != nil {
				t.Fatal(err)
			}
			app := &App{Now: func() time.Time { return now }, Loc: now.Location()}
			data, healthy, fail := app.refreshDataHealth(home)
			if data != tc.data || healthy != tc.healthy || fail != tc.fail {
				t.Fatalf("data=%s healthy=%s fail=%v", data, healthy, fail)
			}
		})
	}
}
