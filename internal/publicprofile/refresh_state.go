package publicprofile

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rainhuang0220/whereToken/internal/adapter"
)

const RefreshStateSchema = 1

// RefreshStatePath is the last refresh result beside profile-refresh.json.
// It stores fixed diagnostics and dates, with no raw adapter errors or credentials.
func RefreshStatePath(home adapter.Home) string {
	return filepath.Join(filepath.Dir(ConfigPathIn(home)), "profile-refresh-state.json")
}

// RefreshState is one local result. Code is empty when a run has not
// produced one of the fixed result codes.
type RefreshState struct {
	Phase       string
	Code        string
	CheckedAt   time.Time
	Collection  *CollectionEvidence
	Publication PublicationEvidence
}

type PublicationEvidence struct {
	SnapshotGeneratedAt *time.Time `json:"snapshot_generated_at,omitempty"`
	SnapshotAsOfDate    string     `json:"as_of_date,omitempty"`
	HostedAcceptedAt    *time.Time `json:"hosted_accepted_at,omitempty"`
	VerifiedAt          *time.Time `json:"publication_verified_at,omitempty"`
	VerifiedAsOfDate    string     `json:"publication_as_of_date,omitempty"`
	VerifiedSnapshotID  string     `json:"publication_snapshot_id,omitempty"`
	PagesExpected       bool       `json:"pages_expected,omitempty"`
	Category            string     `json:"category,omitempty"`
}

type refreshStateFile struct {
	SchemaVersion int                 `json:"schema_version"`
	Phase         string              `json:"phase"`
	Code          string              `json:"code"`
	CheckedAt     string              `json:"checked_at"`
	Collection    *CollectionEvidence `json:"collection,omitempty"`
	Publication   PublicationEvidence `json:"publication,omitzero"`
}

// LoadRefreshState reads the last result. A missing file is os.ErrNotExist.
// Unknown JSON fields are ignored.
func LoadRefreshState(path string) (RefreshState, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return RefreshState{}, err
	}
	var f refreshStateFile
	if err := json.Unmarshal(raw, &f); err != nil {
		return RefreshState{}, err
	}
	st := RefreshState{
		Phase: cleanPhase(f.Phase), Code: f.Code,
		Collection: cleanCollection(f.Collection), Publication: cleanPublication(f.Publication),
	}
	if !AllowedRefreshCode(st.Code) {
		st.Code = ""
	}
	if f.CheckedAt != "" {
		t, err := time.Parse(time.RFC3339, f.CheckedAt)
		if err != nil {
			return RefreshState{}, err
		}
		st.CheckedAt = t
	}
	return st, nil
}

// SaveRefreshState writes the allowlisted fields only, mode 0600.
func SaveRefreshState(path string, st RefreshState) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return err
	}
	code := st.Code
	if !AllowedRefreshCode(code) {
		code = ""
	}
	checked := ""
	if !st.CheckedAt.IsZero() {
		checked = st.CheckedAt.Format(time.RFC3339)
	}
	raw, err := json.MarshalIndent(refreshStateFile{
		SchemaVersion: RefreshStateSchema,
		Phase:         cleanPhase(st.Phase),
		Code:          code,
		CheckedAt:     checked,
		Collection:    cleanCollection(st.Collection),
		Publication:   cleanPublication(st.Publication),
	}, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	tmp, err := os.CreateTemp(dir, ".profile-refresh-state-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(raw); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

func cleanCollection(in *CollectionEvidence) *CollectionEvidence {
	if in == nil {
		return nil
	}
	out := &CollectionEvidence{}
	seen := make(map[string]bool, len(in.Sources))
	for _, check := range in.Sources {
		if !ValidateSourceCoverage(check.SourceCoverage) || seen[check.Source] {
			continue
		}
		switch check.Category {
		case "", "AUTH", "NETWORK", "RATE_LIMIT", "SCHEMA", "COVERAGE", "SOURCE_UNAVAILABLE", "LOCAL_READ", "LEDGER", "RECONCILIATION":
		default:
			check.Category = "SOURCE_UNAVAILABLE"
		}
		seen[check.Source] = true
		out.Sources = append(out.Sources, check)
	}
	return out
}

func cleanPublication(p PublicationEvidence) PublicationEvidence {
	if !validLocalDate(p.SnapshotAsOfDate) {
		p.SnapshotAsOfDate = ""
	}
	if !validLocalDate(p.VerifiedAsOfDate) {
		p.VerifiedAsOfDate = ""
	}
	if !validSnapshotDigest(p.VerifiedSnapshotID) {
		p.VerifiedSnapshotID = ""
	}
	switch p.Category {
	case "", "PUBLISH", "NETWORK", "RATE_LIMIT", "AUTH", "COVERAGE":
	default:
		p.Category = "PUBLISH"
	}
	return p
}

func cleanPhase(phase string) string {
	switch phase {
	case PhaseIdle, PhasePublished, PhaseSkipped, PhaseFailed, PhaseWaiting:
		return phase
	default:
		return PhaseSkipped
	}
}

func validSnapshotDigest(id string) bool {
	hash, ok := strings.CutPrefix(id, "sha256:")
	if !ok || len(hash) != 64 || strings.ToLower(hash) != hash {
		return false
	}
	_, err := hex.DecodeString(hash)
	return err == nil
}
