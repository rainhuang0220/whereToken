package publicprofile

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const (
	LiveSchema        = "wheretoken.public-profile-live"
	LiveSchemaVersion = 1
	FreshnessMode     = "near_real_time"
	FreshnessHosted   = "hosted"

	// ReadmeMinInterval is the fastest ordinary usage materialization.
	// A palette change does not wait for it.
	ReadmeMinInterval = 30 * time.Minute
	// ReadmeQuietWindow publishes a smaller usage change that has been
	// waiting, so a quiet day still refreshes the static README.
	ReadmeQuietWindow = 6 * time.Hour
	// ReadmeMinTokenDelta is the smallest all-time token movement that
	// may refresh the README once the minimum interval has passed.
	ReadmeMinTokenDelta int64 = 100_000
)

// MaterializeInput is the README coalesce decision. ThemeChange publishes
// immediately. Usage changes wait for a real token movement or the quiet window.
type MaterializeInput struct {
	SnapshotChanged  bool
	PrevTotal        int64
	NextTotal        int64
	LastMaterialized time.Time
	Now              time.Time
	ThemeChange      bool
}

// ShouldMaterializeReadme reports whether the static GitHub README should be
// rewritten. The hosted projection itself updates on every accepted sync.
func ShouldMaterializeReadme(in MaterializeInput) bool {
	if in.ThemeChange {
		return true
	}
	if !in.SnapshotChanged {
		return false
	}
	if in.LastMaterialized.IsZero() {
		return true
	}
	if in.Now.Sub(in.LastMaterialized) < ReadmeMinInterval {
		return false
	}
	delta := in.NextTotal - in.PrevTotal
	if delta < 0 {
		delta = -delta
	}
	if delta >= ReadmeMinTokenDelta {
		return true
	}
	return in.Now.Sub(in.LastMaterialized) >= ReadmeQuietWindow
}

// TotalTokens is the public all-time total, or 0 when that component is
// unavailable. It is not a substitute for the component status.
func TotalTokens(s Snapshot) int64 {
	if s.Periods.All.Totals.Total.Value == nil {
		return 0
	}
	return *s.Periods.All.Totals.Total.Value
}

// AcceptProjection validates a device-uploaded public snapshot. The stored
// document is the re-marshaled allowlisted snapshot, never the raw body.
func AcceptProjection(raw []byte) (Snapshot, []byte, error) {
	if len(raw) == 0 || len(raw) > 2<<20 {
		return Snapshot{}, nil, errors.New("publicprofile: projection size")
	}
	if Sensitive(string(raw)) {
		return Snapshot{}, nil, errors.New("publicprofile: projection contains private data")
	}
	var snap Snapshot
	if err := json.Unmarshal(raw, &snap); err != nil {
		return Snapshot{}, nil, fmt.Errorf("publicprofile: projection json: %w", err)
	}
	if snap.Provenance.Kind != ProvenanceLocal || snap.Provenance.RefreshMode != RefreshManualPublish || snap.Provenance.LiveSync {
		return Snapshot{}, nil, errors.New("publicprofile: projection provenance")
	}
	if err := Validate(snap); err != nil {
		return Snapshot{}, nil, err
	}
	canonical, err := Marshal(snap)
	if err != nil {
		return Snapshot{}, nil, err
	}
	if Sensitive(string(canonical)) {
		return Snapshot{}, nil, errors.New("publicprofile: projection contains private data")
	}
	return snap, canonical, nil
}

// HostedAssetRevision names the preview bytes and palette. It is not the
// usage snapshot id.
func HostedAssetRevision(light, dark []byte, palette string) string {
	h := sha256.New()
	h.Write([]byte("preview-light.svg"))
	h.Write([]byte{0})
	h.Write(light)
	h.Write([]byte{0})
	h.Write([]byte("preview-dark.svg"))
	h.Write([]byte{0})
	h.Write(dark)
	h.Write([]byte{0})
	h.Write([]byte(palette))
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}

var previewURLPattern = regexp.MustCompile(`https://[A-Za-z0-9._~:/?#\[\]@!$&'()*+,;=%-]*preview-(light|dark)\.svg\?v=[0-9a-f]{64}-[0-9a-f]{64}`)

// RawPreviewBase is the GitHub-hosted prefix for materialized preview SVGs.
// The profile repository is the README's image origin so Camo does not depend
// on the hosted process staying up.
func RawPreviewBase(repo, branch, previewDir string) (string, error) {
	owner, name, err := splitRepo(repo)
	if err != nil {
		return "", err
	}
	if !validBranch(branch) {
		return "", errors.New("publicprofile: profile branch")
	}
	dir := strings.Trim(strings.TrimSpace(previewDir), "/")
	if dir == "" || strings.Contains(dir, "..") || strings.Contains(dir, "/") {
		return "", errors.New("publicprofile: preview directory")
	}
	return "https://raw.githubusercontent.com/" + owner + "/" + name + "/" + branch + "/" + dir, nil
}

func splitRepo(repo string) (owner, name string, err error) {
	parts := strings.Split(strings.TrimSpace(repo), "/")
	if len(parts) != 2 || !validRepoSegment(parts[0]) || !validRepoSegment(parts[1]) {
		return "", "", errors.New("publicprofile: profile repo")
	}
	return parts[0], parts[1], nil
}

func validRepoSegment(s string) bool {
	if s == "" || len(s) > 100 || s == "." || s == ".." {
		return false
	}
	for _, c := range s {
		if (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') && (c < '0' || c > '9') && c != '-' && c != '_' && c != '.' {
			return false
		}
	}
	return true
}

// RewriteReadmeProjection replaces only the three whereToken preview URLs.
// The new origin must be the raw GitHub base for this installation. Every
// other line, including other images, stays byte-for-byte.
func RewriteReadmeProjection(src, assetBase, snapshotID, assetRevision string) (string, []ReadmeEdit, error) {
	key, err := CacheKey(snapshotID, assetRevision)
	if err != nil {
		return "", nil, err
	}
	base := strings.TrimRight(strings.TrimSpace(assetBase), "/")
	if err := validateRawBase(base); err != nil {
		return "", nil, err
	}
	spans := previewURLPattern.FindAllStringIndex(src, -1)
	if len(spans) != 3 {
		return "", nil, errors.New("publicprofile: README preview references are missing or ambiguous")
	}
	var b strings.Builder
	edits := make([]ReadmeEdit, 0, 3)
	prev := 0
	dark, light := 0, 0
	lightSeen := 0
	for _, span := range spans {
		raw := src[span[0]:span[1]]
		if err := allowedPreviewHost(raw); err != nil {
			return "", nil, err
		}
		file := "preview-light.svg"
		slot := "img"
		if strings.Contains(raw, "preview-dark.svg") {
			file = "preview-dark.svg"
			slot = "dark_srcset"
			dark++
		} else {
			light++
			if lightSeen == 0 {
				slot = "light_srcset"
			}
			lightSeen++
		}
		after := base + "/" + file + "?v=" + key
		b.WriteString(src[prev:span[0]])
		b.WriteString(after)
		edits = append(edits, ReadmeEdit{Slot: slot, Before: raw, After: after})
		prev = span[1]
	}
	if dark != 1 || light != 2 {
		return "", nil, errors.New("publicprofile: README preview slots")
	}
	b.WriteString(src[prev:])
	out := b.String()
	if strings.Count(out, "preview-dark.svg") != 1 || strings.Count(out, "preview-light.svg") != 2 {
		return "", nil, errors.New("publicprofile: README has more than one whereToken preview picture")
	}
	return out, edits, nil
}

func validateRawBase(base string) error {
	u, err := url.Parse(base)
	if err != nil || u.Scheme != "https" || u.Host != "raw.githubusercontent.com" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return errors.New("publicprofile: preview base")
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) != 4 {
		return errors.New("publicprofile: preview base path")
	}
	if _, _, err := splitRepo(parts[0] + "/" + parts[1]); err != nil {
		return err
	}
	if !validBranch(parts[2]) || strings.Contains(parts[3], "..") {
		return errors.New("publicprofile: preview base path")
	}
	return nil
}

func allowedPreviewHost(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.User != nil {
		return errors.New("publicprofile: preview URL")
	}
	switch u.Host {
	case "raw.githubusercontent.com", "github.com", "rainhuang0220.github.io":
		return nil
	default:
		return errors.New("publicprofile: preview host")
	}
}
