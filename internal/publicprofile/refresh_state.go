package publicprofile

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/rainhuang0220/whereToken/internal/adapter"
)

const RefreshStateSchema = 1

// RefreshStatePath is the last refresh result beside profile-refresh.json.
// It stores schema_version, phase, code, and checked_at. It does not store
// totals, snapshot ids, paths, or tokens.
func RefreshStatePath(home adapter.Home) string {
	return filepath.Join(filepath.Dir(ConfigPathIn(home)), "profile-refresh-state.json")
}

// RefreshState is one local result. Code is empty when a run has not
// produced one of the fixed result codes.
type RefreshState struct {
	Phase     string
	Code      string
	CheckedAt time.Time
}

type refreshStateFile struct {
	SchemaVersion int    `json:"schema_version"`
	Phase         string `json:"phase"`
	Code          string `json:"code"`
	CheckedAt     string `json:"checked_at"`
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
	st := RefreshState{Phase: cleanPhase(f.Phase), Code: f.Code}
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
	}, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	if err := os.Chmod(tmp, 0o600); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, path)
}

func cleanPhase(phase string) string {
	switch phase {
	case PhaseIdle, PhasePublished, PhaseSkipped, PhaseFailed, PhaseWaiting:
		return phase
	default:
		return PhaseSkipped
	}
}
