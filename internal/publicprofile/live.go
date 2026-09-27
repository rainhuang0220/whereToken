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

// GitHub wall publication is DecidePublication. The hosted projection and
// the verified README no longer use a separate token or quiet-window gate.

const (
	LiveSchema        = "wheretoken.public-profile-live"
	LiveSchemaVersion = 1
	FreshnessMode     = "near_real_time"
	FreshnessHosted   = "hosted"
)

// laterLocalDate reports whether next is a later YYYY-MM-DD local date than prev.
// It does not interpret the dates as UTC instants beyond the calendar day.
func laterLocalDate(next, prev string) bool {
	n, errN := time.Parse("2006-01-02", strings.TrimSpace(next))
	p, errP := time.Parse("2006-01-02", strings.TrimSpace(prev))
	if errN != nil || errP != nil {
		return false
	}
	return n.After(p)
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

// ReadmeFacts is the sanitized snapshot text the README alt must match.
// It is separate from snapshot_id and from the preview asset revision.
type ReadmeFacts struct {
	TotalDisplay string
	DataStatus   string
	AsOfDate     string
}

// ReadmeAlt is the preview image alt built from the current sanitized snapshot.
func ReadmeAlt(facts ReadmeFacts) (string, error) {
	when, err := time.Parse("2006-01-02", strings.TrimSpace(facts.AsOfDate))
	if err != nil {
		return "", errors.New("publicprofile: readme alt date")
	}
	date := when.Format("January 2, 2006")
	display := strings.TrimSpace(facts.TotalDisplay)
	var total, coverage string
	switch facts.DataStatus {
	case StatusUnavailable:
		total = "unavailable"
		coverage = "coverage unavailable"
	case StatusPartial:
		coverage = "partial coverage"
		if !readmeDisplayOK(display) {
			return "", errors.New("publicprofile: readme alt total")
		}
		total = display + " measured tokens"
	case StatusAvailable:
		coverage = "complete coverage"
		if !readmeDisplayOK(display) {
			return "", errors.New("publicprofile: readme alt total")
		}
		total = display + " measured tokens"
	default:
		return "", errors.New("publicprofile: readme alt status")
	}
	return escapeAttr("Coding activity snapshot: " + total + ", " + coverage + ", updated " + date), nil
}

func readmeDisplayOK(display string) bool {
	if display == "" || display == emDash || display == "-" {
		return false
	}
	return !strings.ContainsAny(display, "\"<>\n\r&")
}

func escapeAttr(value string) string {
	value = strings.ReplaceAll(value, "&", "&amp;")
	value = strings.ReplaceAll(value, `"`, "&quot;")
	value = strings.ReplaceAll(value, "<", "&lt;")
	return value
}

// RewriteReadmeProjection replaces the three whereToken preview URLs and the
// alt on that same preview image. The new origin must be the raw GitHub base
// for this installation. Every other byte, including other images, stays.
func RewriteReadmeProjection(src, assetBase, snapshotID, assetRevision string, facts ReadmeFacts) (string, []ReadmeEdit, error) {
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
	alt, err := ReadmeAlt(facts)
	if err != nil {
		return "", nil, err
	}
	out, err = replacePreviewAlt(out, alt)
	if err != nil {
		return "", nil, err
	}
	return out, edits, nil
}

func replacePreviewAlt(src, alt string) (string, error) {
	if strings.Contains(alt, `"`) {
		return "", errors.New("publicprofile: readme preview alt")
	}
	var spans [][2]int
	for i := 0; i < len(src); {
		rel := strings.Index(src[i:], "<img")
		if rel < 0 {
			break
		}
		start := i + rel
		endRel := strings.Index(src[start:], ">")
		if endRel < 0 {
			return "", errors.New("publicprofile: readme img")
		}
		end := start + endRel + 1
		if strings.Contains(src[start:end], "preview-light.svg") {
			spans = append(spans, [2]int{start, end})
		}
		i = end
	}
	if len(spans) != 1 {
		return "", errors.New("publicprofile: readme preview alt")
	}
	start, end := spans[0][0], spans[0][1]
	tag := src[start:end]
	const attr = `alt="`
	if strings.Count(tag, attr) > 1 {
		return "", errors.New("publicprofile: readme preview alt")
	}
	var newTag string
	if at := strings.Index(tag, attr); at >= 0 {
		rest := tag[at+len(attr):]
		quote := strings.Index(rest, `"`)
		if quote < 0 {
			return "", errors.New("publicprofile: readme preview alt")
		}
		newTag = tag[:at] + attr + alt + `"` + rest[quote+1:]
	} else if strings.HasSuffix(tag, ">") {
		newTag = tag[:len(tag)-1] + ` alt="` + alt + `">`
	} else {
		return "", errors.New("publicprofile: readme preview alt")
	}
	return src[:start] + newTag + src[end:], nil
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
