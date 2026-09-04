package community

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

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

// configMu serializes config-file access process-wide: Save renames over the
// live path, which Windows refuses while a concurrent Load holds it open, and
// interleaved writers could otherwise rename over each other's tmp file.
var configMu sync.Mutex

func Load(path string) (*File, error) {
	configMu.Lock()
	defer configMu.Unlock()
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
	configMu.Lock()
	defer configMu.Unlock()
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
	if v == "0" || v == "false" || v == "off" {
		return true
	}
	// DO_NOT_TRACK=1 (true/on/yes) opts out; empty and 0 do not.
	d := strings.ToLower(strings.TrimSpace(getenv("DO_NOT_TRACK")))
	return d == "1" || d == "true" || d == "on" || d == "yes"
}

func EnvURL(getenv func(string) string) string {
	if getenv == nil {
		return ""
	}
	s, err := ParseServiceURL(getenv("WHERETOKEN_COMMUNITY_URL"))
	if err != nil {
		return ""
	}
	return s
}

// ParseServiceURL accepts only http(s) rank endpoints. file:, userinfo, and
// empty hosts are rejected so the client never opens those URLs.
func ParseServiceURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("empty community url")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("invalid community url")
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return "", fmt.Errorf("community url must be http or https")
	}
	if u.Host == "" || u.User != nil || strings.Contains(raw, "@") {
		return "", fmt.Errorf("invalid community url")
	}
	u.Scheme = scheme
	u.Fragment = ""
	u.RawQuery = ""
	return strings.TrimRight(u.String(), "/"), nil
}
