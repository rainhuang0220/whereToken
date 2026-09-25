package publicprofile

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/rainhuang0220/whereToken/internal/adapter"
)

const RefreshSwitchSchema = 1

// RefreshConfigPath is the on/off switch beside community.json.
// It stores only schema_version and enabled. The watermark is the hosted
// envelope, not this file.
func RefreshConfigPath(home adapter.Home) string {
	return filepath.Join(filepath.Dir(ConfigPathIn(home)), "profile-refresh.json")
}

type refreshSwitchFile struct {
	SchemaVersion int  `json:"schema_version"`
	Enabled       bool `json:"enabled"`
}

// LoadRefreshSwitch reads the switch. Missing file is not an error for the
// caller to treat as off: a non-nil error means the file could not be read
// or parsed. Unknown JSON fields are ignored.
func LoadRefreshSwitch(path string) (bool, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	var f refreshSwitchFile
	if err := json.Unmarshal(raw, &f); err != nil {
		return false, err
	}
	return f.Enabled, nil
}

// SaveRefreshSwitch writes enabled and schema_version only, mode 0600,
// directory 0700. It does not copy paths, tokens, totals, or snapshots.
func SaveRefreshSwitch(path string, enabled bool) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(refreshSwitchFile{SchemaVersion: RefreshSwitchSchema, Enabled: enabled}, "", "  ")
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
