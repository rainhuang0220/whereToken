package community

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/rainhuang0220/whereToken/internal/adapter"
)

// File is the local participation record. It lives next to user config, never
// inside the usage SQLite index.
type File struct {
	ParticipantID string `json:"participant_id"`
	Enabled       bool   `json:"enabled"`
	JoinedAt      string `json:"joined_at"`
}

func ConfigPath(home adapter.Home) string {
	if v := strings.TrimSpace(os.Getenv("WHERETOKEN_COMMUNITY_FILE")); v != "" {
		return v
	}
	if runtime.GOOS == "windows" {
		return filepath.Join(home.AppData("whereToken"), "community.json")
	}
	return filepath.Join(home.XDGConfig("wheretoken"), "community.json")
}

func NewParticipantID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:]), nil
}

func LoadOrCreate(path, joinedAt string) (*File, error) {
	f, err := Load(path)
	if err == nil {
		return f, nil
	}
	if !os.IsNotExist(err) {
		return nil, err
	}
	id, err := NewParticipantID()
	if err != nil {
		return nil, err
	}
	f = &File{ParticipantID: id, Enabled: true, JoinedAt: joinedAt}
	if err := f.Save(path); err != nil {
		return nil, err
	}
	return f, nil
}

func Load(path string) (*File, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var f File
	if err := json.Unmarshal(raw, &f); err != nil {
		return nil, err
	}
	if !uuidRe.MatchString(f.ParticipantID) {
		return nil, fmt.Errorf("community.json: invalid participant_id")
	}
	return &f, nil
}

func (f *File) Save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func (f *File) SetEnabled(path string, on bool) error {
	f.Enabled = on
	return f.Save(path)
}

func EnvDisabled(getenv func(string) string) bool {
	if getenv == nil {
		return false
	}
	v := strings.ToLower(strings.TrimSpace(getenv("WHERETOKEN_COMMUNITY")))
	return v == "0" || v == "false" || v == "off"
}

func EnvURL(getenv func(string) string) string {
	if getenv == nil {
		return ""
	}
	return strings.TrimRight(strings.TrimSpace(getenv("WHERETOKEN_COMMUNITY_URL")), "/")
}
