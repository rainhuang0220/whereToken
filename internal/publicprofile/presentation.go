package publicprofile

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/rainhuang0220/whereToken/internal/adapter"
)

const (
	PaletteCobalt    = "cobalt"
	PaletteMagenta   = "magenta"
	PaletteNewsprint = "newsprint"
	DefaultPalette   = PaletteNewsprint

	PresentationSchema = 1

	// Publication states describe the local owner config and the local bundle.
	// They do not claim a remote host was updated.
	StatusUnconfigured   = "unconfigured"
	StatusSavedLocally   = "saved_locally"
	StatusPending        = "pending"
	StatusReadyToPublish = "ready_to_publish"
	StatusFailed         = "failed"
)

// Presentation is the public bundle's palette contract. It is not usage data
// and must not include local paths.
type Presentation struct {
	SchemaVersion int    `json:"schema_version"`
	PublicPalette string `json:"public_palette"`
	Revision      string `json:"revision"`
	UpdatedAt     string `json:"updated_at,omitempty"`
}

// OwnerSettings is the local authoritative palette. BundleDir is the last
// local bundle this machine wrote; it is omitted from the public file.
type OwnerSettings struct {
	SchemaVersion int    `json:"schema_version"`
	PublicPalette string `json:"public_palette"`
	Revision      string `json:"revision"`
	UpdatedAt     string `json:"updated_at,omitempty"`
	BundleDir     string `json:"bundle_dir,omitempty"`
}

func (o OwnerSettings) Presentation() Presentation {
	return Presentation{
		SchemaVersion: o.SchemaVersion,
		PublicPalette: o.PublicPalette,
		Revision:      o.Revision,
		UpdatedAt:     o.UpdatedAt,
	}
}

func DefaultPresentation() Presentation {
	return Presentation{SchemaVersion: PresentationSchema, PublicPalette: DefaultPalette, Revision: "0"}
}

func DefaultOwner() OwnerSettings {
	p := DefaultPresentation()
	return OwnerSettings{SchemaVersion: p.SchemaVersion, PublicPalette: p.PublicPalette, Revision: p.Revision}
}

func KnownPalette(id string) bool {
	switch strings.TrimSpace(id) {
	case PaletteCobalt, PaletteMagenta, PaletteNewsprint:
		return true
	default:
		return false
	}
}

func ValidatePalette(id string) error {
	if !KnownPalette(id) {
		return fmt.Errorf("publicprofile: unknown palette %q", strings.TrimSpace(id))
	}
	return nil
}

// CoercePresentation keeps a valid palette and otherwise returns the default.
// An unknown schema version degrades the same way: the page still opens.
func CoercePresentation(p Presentation) Presentation {
	out := DefaultPresentation()
	if KnownPalette(p.PublicPalette) {
		out.PublicPalette = p.PublicPalette
	}
	if p.SchemaVersion == PresentationSchema {
		out.SchemaVersion = PresentationSchema
	}
	rev := strings.TrimSpace(p.Revision)
	if rev != "" && len(rev) <= 80 && !strings.ContainsAny(rev, "\r\n") {
		out.Revision = rev
	}
	if ts := strings.TrimSpace(p.UpdatedAt); ts != "" && len(ts) <= 40 && !strings.ContainsAny(ts, " \r\n") {
		out.UpdatedAt = ts
	}
	return out
}

func MarshalPresentation(p Presentation) ([]byte, error) {
	p = CoercePresentation(p)
	raw, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(raw, '\n'), nil
}

func ParsePresentation(raw []byte) (Presentation, error) {
	var p Presentation
	if err := json.Unmarshal(raw, &p); err != nil {
		return DefaultPresentation(), err
	}
	if !KnownPalette(p.PublicPalette) {
		return DefaultPresentation(), fmt.Errorf("publicprofile: unknown palette %q", p.PublicPalette)
	}
	return CoercePresentation(p), nil
}

// ConfigPath is the owner palette file. It lives beside community.json and
// never inside the usage index. WHERETOKEN_PUBLIC_PROFILE_FILE overrides it.
func ConfigPath(home adapter.Home) string {
	if v := strings.TrimSpace(os.Getenv("WHERETOKEN_PUBLIC_PROFILE_FILE")); v != "" {
		return v
	}
	return ConfigPathIn(home)
}

// ConfigPathIn ignores the environment override. Callers that stub env lookup
// use this after checking their own environment.
func ConfigPathIn(home adapter.Home) string {
	if runtime.GOOS == "windows" {
		return filepath.Join(home.AppData("whereToken"), "public-profile.json")
	}
	return filepath.Join(home.XDGConfig("wheretoken"), "public-profile.json")
}

func LoadOwner(path string) (OwnerSettings, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return OwnerSettings{}, err
	}
	var o OwnerSettings
	if err := json.Unmarshal(raw, &o); err != nil {
		return OwnerSettings{}, err
	}
	if !KnownPalette(o.PublicPalette) {
		return OwnerSettings{}, fmt.Errorf("publicprofile: unknown palette %q", o.PublicPalette)
	}
	coerced := CoercePresentation(o.Presentation())
	o.SchemaVersion = coerced.SchemaVersion
	o.PublicPalette = coerced.PublicPalette
	o.Revision = coerced.Revision
	o.UpdatedAt = coerced.UpdatedAt
	o.BundleDir = strings.TrimSpace(o.BundleDir)
	return o, nil
}

// FallbackOwner returns a saved palette or the documented default. A missing
// or invalid file does not break a build.
func FallbackOwner(path string) OwnerSettings {
	o, err := LoadOwner(path)
	if err != nil {
		return DefaultOwner()
	}
	return o
}

func (o *OwnerSettings) ApplyPalette(id, updatedAt string) error {
	if err := ValidatePalette(id); err != nil {
		return err
	}
	o.SchemaVersion = PresentationSchema
	o.PublicPalette = id
	o.Revision = nextRevision(o.Revision)
	o.UpdatedAt = updatedAt
	return nil
}

func SaveOwner(path string, o OwnerSettings) error {
	if err := ValidatePalette(o.PublicPalette); err != nil {
		return err
	}
	if o.SchemaVersion == 0 {
		o.SchemaVersion = PresentationSchema
	}
	raw, err := json.MarshalIndent(o, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".public-profile-*.json")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	ok := false
	defer func() {
		if !ok {
			os.Remove(tmpName)
		}
	}()
	if _, err := tmp.Write(raw); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpName, 0o644); err != nil && runtime.GOOS != "windows" {
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return err
	}
	ok = true
	return nil
}

func nextRevision(cur string) string {
	n, err := strconv.Atoi(strings.TrimSpace(cur))
	if err != nil || n < 0 {
		n = 0
	}
	return strconv.Itoa(n + 1)
}

func OwnerSaved(o OwnerSettings) bool {
	return strings.TrimSpace(o.UpdatedAt) != "" || (o.Revision != "" && o.Revision != "0")
}

// Publication is the local save/export view. Status never means a remote
// site changed.
type Publication struct {
	SchemaVersion  int    `json:"schema_version"`
	PublicPalette  string `json:"public_palette"`
	Revision       string `json:"revision"`
	UpdatedAt      string `json:"updated_at,omitempty"`
	BundlePalette  string `json:"bundle_palette,omitempty"`
	BundleRevision string `json:"bundle_revision,omitempty"`
	BundleDir      string `json:"bundle_dir,omitempty"`
	Status         string `json:"status"`
	PreviewPath    string `json:"preview_path,omitempty"`
	ExportCommand  string `json:"export_command"`
	Error          string `json:"error,omitempty"`
}

func DescribePublication(owner OwnerSettings, bundleDir string, bundle Presentation, haveBundle bool, fail string) Publication {
	ownerP := CoercePresentation(owner.Presentation())
	if !OwnerSaved(owner) {
		ownerP.UpdatedAt = ""
	}
	out := Publication{
		SchemaVersion: PresentationSchema,
		PublicPalette: ownerP.PublicPalette,
		Revision:      ownerP.Revision,
		UpdatedAt:     ownerP.UpdatedAt,
		ExportCommand: ExportCommand(bundleDir, ownerP.PublicPalette),
		Status:        StatusUnconfigured,
	}
	if haveBundle {
		b := CoercePresentation(bundle)
		out.BundlePalette = b.PublicPalette
		out.BundleRevision = b.Revision
		out.BundleDir = bundleDir
		out.PreviewPath = "/preview/public-profile/"
	}
	if fail != "" {
		out.Status = StatusFailed
		out.Error = fail
		return out
	}
	if !OwnerSaved(owner) {
		out.Status = StatusUnconfigured
		return out
	}
	if !haveBundle {
		out.Status = StatusSavedLocally
		return out
	}
	if out.BundlePalette != ownerP.PublicPalette || out.BundleRevision != ownerP.Revision {
		out.Status = StatusPending
		return out
	}
	out.Status = StatusReadyToPublish
	return out
}

func ExportCommand(dir, palette string) string {
	if strings.TrimSpace(dir) == "" {
		dir = "./public-profile"
	}
	if !KnownPalette(palette) {
		palette = DefaultPalette
	}
	if strings.ContainsAny(dir, " \t'\"") {
		dir = strconv.Quote(dir)
	}
	return "wheretoken profile build " + dir + " --public-palette " + palette
}

func ReadBundlePresentation(dir string) (Presentation, error) {
	raw, err := os.ReadFile(filepath.Join(dir, "presentation.json"))
	if err != nil {
		return Presentation{}, err
	}
	return ParsePresentation(raw)
}

// ResolveBundleDir finds an existing local bundle without accepting an
// arbitrary browser-supplied path. Order: env, saved dir, ./public-profile.
func ResolveBundleDir(owner OwnerSettings) string {
	if v := strings.TrimSpace(os.Getenv("WHERETOKEN_PUBLIC_PROFILE_DIR")); v != "" && IsBundle(v) {
		return v
	}
	if owner.BundleDir != "" && IsBundle(owner.BundleDir) {
		return owner.BundleDir
	}
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	cand := filepath.Join(cwd, "public-profile")
	if IsBundle(cand) {
		return cand
	}
	return ""
}

func IsBundle(dir string) bool {
	if strings.TrimSpace(dir) == "" {
		return false
	}
	st, err := os.Stat(filepath.Join(dir, "profile.json"))
	return err == nil && !st.IsDir()
}

func Install(dir string, files map[string][]byte) error {
	if strings.TrimSpace(dir) == "" {
		return errors.New("publicprofile: missing path")
	}
	if err := os.MkdirAll(filepath.Join(dir, "assets"), 0o755); err != nil {
		return err
	}
	for _, name := range GeneratedFiles {
		payload, ok := files[name]
		if !ok {
			return fmt.Errorf("publicprofile: missing %s", name)
		}
		if err := writeAtomic(filepath.Join(dir, filepath.FromSlash(name)), payload); err != nil {
			return err
		}
	}
	for _, name := range RetiredGeneratedFiles {
		if err := os.Remove(filepath.Join(dir, filepath.FromSlash(name))); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func WriteBundle(dir string, snap Snapshot, p Presentation) error {
	files, err := BundleWith(snap, p)
	if err != nil {
		return err
	}
	return Install(dir, files)
}

func writeAtomic(dest string, payload []byte) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(dest), ".wt-bundle-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	ok := false
	defer func() {
		if !ok {
			os.Remove(tmpName)
		}
	}()
	if _, err := tmp.Write(payload); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpName, 0o644); err != nil && runtime.GOOS != "windows" {
		return err
	}
	if err := os.Rename(tmpName, dest); err != nil {
		return err
	}
	ok = true
	return nil
}
