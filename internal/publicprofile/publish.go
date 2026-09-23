package publicprofile

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/rainhuang0220/whereToken/internal/adapter"
)

const (
	PhaseUnsaved            = "unsaved"
	PhaseSavedLocally       = "saved_locally"
	PhasePreflight          = "preflight"
	PhaseAwaitingApproval   = "awaiting_approval"
	PhaseUpdatingBundle     = "updating_bundle"
	PhasePushingProduct     = "pushing_product"
	PhaseWaitingPages       = "waiting_pages"
	PhaseLiveVerified       = "live_verified"
	PhaseUpdatingReadme     = "updating_readme"
	PhaseVerified           = "verified"
	PhasePartiallyPublished = "partially_published"
	PhaseFailed             = "failed"

	publishSchema = 1
)

// PhaseLabel is the owner-facing status. It describes a verified publisher
// phase, not a button click.
func PhaseLabel(phase string) string {
	switch phase {
	case PhaseUnsaved:
		return "未保存"
	case PhaseSavedLocally:
		return "本机已保存"
	case PhasePreflight:
		return "待预检"
	case PhaseAwaitingApproval:
		return "待本人确认"
	case PhaseUpdatingBundle:
		return "正在更新公开包"
	case PhasePushingProduct:
		return "正在推送 whereToken"
	case PhaseWaitingPages:
		return "等待 Pages"
	case PhaseLiveVerified:
		return "线上已核验"
	case PhaseUpdatingReadme:
		return "正在更新个人 README"
	case PhaseVerified:
		return "GitHub 主页已核验"
	case PhasePartiallyPublished:
		return "公开页已上线，个人 README 未更新"
	case PhaseFailed:
		return "发布失败"
	default:
		return "待预检"
	}
}

// PublishConfig is the owner's explicit two-repository target. It lives beside
// the local palette file and is never copied into a public bundle.
type PublishConfig struct {
	SchemaVersion int    `json:"schema_version"`
	ProductRepo   string `json:"product_repo"`
	ProductBranch string `json:"product_branch"`
	ProfileRepo   string `json:"profile_repo"`
	ProfileBranch string `json:"profile_branch"`
	ProfileReadme string `json:"profile_readme"`
	PagesURL      string `json:"pages_url"`
	Checkout      string `json:"checkout,omitempty"`
}

type ReadmeEdit struct {
	Slot   string `json:"slot"`
	Before string `json:"before"`
	After  string `json:"after"`
}

type FileChange struct {
	Path   string `json:"path"`
	Action string `json:"action"`
}

// Preflight is the read-only proposal shown before any remote write.
// Checkout is kept off the JSON so a local path is not returned to the page.
type Preflight struct {
	ID                 string       `json:"preflight_id"`
	Phase              string       `json:"phase"`
	PhaseLabel         string       `json:"phase_label"`
	Ready              bool         `json:"ready"`
	Idempotent         bool         `json:"idempotent"`
	Palette            string       `json:"palette"`
	LivePalette        string       `json:"live_palette,omitempty"`
	LocalPalette       string       `json:"local_palette,omitempty"`
	ProductRepo        string       `json:"product_repo,omitempty"`
	ProductBranch      string       `json:"product_branch,omitempty"`
	ProfileRepo        string       `json:"profile_repo,omitempty"`
	ProfileBranch      string       `json:"profile_branch,omitempty"`
	ReadmePath         string       `json:"readme_path,omitempty"`
	PagesURL           string       `json:"pages_url,omitempty"`
	BundleRel          string       `json:"bundle_rel,omitempty"`
	SnapshotID         string       `json:"snapshot_id,omitempty"`
	AssetRevision      string       `json:"asset_revision,omitempty"`
	LiveAssetRevision  string       `json:"live_asset_revision,omitempty"`
	CacheKey           string       `json:"cache_key,omitempty"`
	ProductCommit      bool         `json:"product_commit"`
	ProductPush        bool         `json:"product_push"`
	ReadmeChange       bool         `json:"readme_change"`
	Ahead              []string     `json:"ahead,omitempty"`
	Dirty              []string     `json:"dirty,omitempty"`
	Files              []FileChange `json:"files,omitempty"`
	PresentationBefore string       `json:"presentation_before,omitempty"`
	PresentationAfter  string       `json:"presentation_after,omitempty"`
	ReadmeEdits        []ReadmeEdit `json:"readme_edits,omitempty"`
	ReadmeSHA          string       `json:"readme_sha,omitempty"`
	ReadmeBlob         string       `json:"readme_blob,omitempty"`
	GitHubLogin        string       `json:"github_login,omitempty"`
	ProductPushRight   bool         `json:"product_push_right"`
	ProfilePushRight   bool         `json:"profile_push_right"`
	BranchProtection   string       `json:"branch_protection,omitempty"`
	Validation         string       `json:"validation,omitempty"`
	Provenance         string       `json:"provenance,omitempty"`
	StaticSVG          bool         `json:"static_svg"`
	ReleaseNote        string       `json:"release_note"`
	Blockers           []string     `json:"blockers,omitempty"`
	Checkout           string       `json:"-"`
	BundleDir          string       `json:"-"`
	Head               string       `json:"-"`
}

type Job struct {
	Phase         string   `json:"phase"`
	PhaseLabel    string   `json:"phase_label"`
	Palette       string   `json:"palette"`
	ProductRepo   string   `json:"product_repo,omitempty"`
	ProductSHA    string   `json:"product_sha,omitempty"`
	ProductURL    string   `json:"product_url,omitempty"`
	PagesRunURL   string   `json:"pages_run_url,omitempty"`
	ProfileRepo   string   `json:"profile_repo,omitempty"`
	ProfileSHA    string   `json:"profile_sha,omitempty"`
	ProfileURL    string   `json:"profile_url,omitempty"`
	CacheKey      string   `json:"cache_key,omitempty"`
	SnapshotID    string   `json:"snapshot_id,omitempty"`
	AssetRevision string   `json:"asset_revision,omitempty"`
	LivePalette   string   `json:"live_palette,omitempty"`
	PreviewLinks  []string `json:"preview_links,omitempty"`
	LivePage      string   `json:"live_page,omitempty"`
	Error         string   `json:"error,omitempty"`
	RetryReadme   bool     `json:"retry_readme"`
	PreflightID   string   `json:"preflight_id,omitempty"`
}

func PublishConfigPath(home adapter.Home) string {
	if v := strings.TrimSpace(os.Getenv("WHERETOKEN_PROFILE_PUBLISH_FILE")); v != "" {
		return v
	}
	return filepath.Join(filepath.Dir(ConfigPathIn(home)), "profile-publish.json")
}

func LoadPublishConfig(path string) (PublishConfig, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return PublishConfig{}, err
	}
	var cfg PublishConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return PublishConfig{}, err
	}
	cfg = normalizePublishConfig(cfg)
	if err := ValidatePublishConfig(cfg); err != nil {
		return PublishConfig{}, err
	}
	return cfg, nil
}

func SavePublishConfig(path string, cfg PublishConfig) error {
	cfg = normalizePublishConfig(cfg)
	if err := ValidatePublishConfig(cfg); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return writeAtomic(path, raw)
}

func normalizePublishConfig(cfg PublishConfig) PublishConfig {
	cfg.SchemaVersion = publishSchema
	cfg.ProductRepo = strings.TrimSpace(cfg.ProductRepo)
	cfg.ProfileRepo = strings.TrimSpace(cfg.ProfileRepo)
	cfg.ProductBranch = strings.TrimSpace(cfg.ProductBranch)
	cfg.ProfileBranch = strings.TrimSpace(cfg.ProfileBranch)
	cfg.ProfileReadme = strings.TrimSpace(cfg.ProfileReadme)
	cfg.PagesURL = strings.TrimSpace(cfg.PagesURL)
	cfg.Checkout = strings.TrimSpace(cfg.Checkout)
	if cfg.ProductBranch == "" {
		cfg.ProductBranch = "main"
	}
	if cfg.ProfileBranch == "" {
		cfg.ProfileBranch = "main"
	}
	if cfg.ProfileReadme == "" {
		cfg.ProfileReadme = "README.md"
	}
	return cfg
}

var (
	repoPattern   = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,80}/[A-Za-z0-9][A-Za-z0-9_.-]{0,80}$`)
	branchPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/-]{0,80}$`)
	secretPattern = regexp.MustCompile(`(?i)(ghp_[A-Za-z0-9]+|github_pat_[A-Za-z0-9_]+|gho_[A-Za-z0-9]+|Bearer\s+\S+)`)
)

func ValidatePublishConfig(cfg PublishConfig) error {
	if cfg.SchemaVersion != publishSchema {
		return errors.New("publicprofile: publish config schema is not supported")
	}
	if !repoPattern.MatchString(cfg.ProductRepo) || strings.Contains(cfg.ProductRepo, "..") {
		return errors.New("publicprofile: product repository must be owner/name")
	}
	if !repoPattern.MatchString(cfg.ProfileRepo) || strings.Contains(cfg.ProfileRepo, "..") {
		return errors.New("publicprofile: profile repository must be owner/name")
	}
	if !validBranch(cfg.ProductBranch) || !validBranch(cfg.ProfileBranch) {
		return errors.New("publicprofile: branch name is not allowed")
	}
	if cfg.ProfileReadme != "README.md" {
		return errors.New("publicprofile: profile publish only edits README.md")
	}
	if !validPagesURL(cfg.PagesURL) {
		return errors.New("publicprofile: pages URL must be an https profile address")
	}
	return nil
}

func validBranch(s string) bool {
	return branchPattern.MatchString(s) && !strings.Contains(s, "..") && !strings.Contains(s, "@{")
}

func validPagesURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Host == "" || u.RawQuery != "" || u.Fragment != "" {
		return false
	}
	path := strings.TrimRight(u.Path, "/")
	return strings.HasSuffix(path, "/profile") && !strings.Contains(path, "..")
}

func Redact(s string) string {
	s = secretPattern.ReplaceAllString(s, "[redacted]")
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) > 400 {
		s = s[:400]
	}
	return strings.TrimSpace(s)
}

func CacheKey(snapshotID, assetRevision string) (string, error) {
	snap := strings.TrimPrefix(strings.TrimSpace(snapshotID), "sha256:")
	asset := strings.TrimPrefix(strings.TrimSpace(assetRevision), "sha256:")
	if !isHex64(snap) || !isHex64(asset) {
		return "", errors.New("publicprofile: cache key needs two sha256 digests")
	}
	return snap + "-" + asset, nil
}

func isHex64(s string) bool {
	if len(s) != 64 {
		return false
	}
	for _, c := range s {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

// RewriteProfileReadme replaces only the three public-profile preview URLs.
// Any other picture, or a second whereToken preview, is left untouched and
// reported as ambiguous.
func RewriteProfileReadme(src, pagesURL, snapshotID, assetRevision string) (string, []ReadmeEdit, error) {
	key, err := CacheKey(snapshotID, assetRevision)
	if err != nil {
		return "", nil, err
	}
	base := strings.TrimRight(strings.TrimSpace(pagesURL), "/")
	if !validPagesURL(base+"/") && !validPagesURL(base) {
		return "", nil, errors.New("publicprofile: pages URL must be an https profile address")
	}
	slots := []struct {
		name string
		attr string
		file string
	}{
		{"dark_srcset", "srcset", "preview-dark.svg"},
		{"light_srcset", "srcset", "preview-light.svg"},
		{"img", "src", "preview-light.svg"},
	}
	out := src
	edits := make([]ReadmeEdit, 0, 3)
	for _, slot := range slots {
		prefix := slot.attr + `="` + base + "/" + slot.file + "?v="
		matches := findCacheURLs(out, prefix)
		if len(matches) != 1 {
			return "", nil, fmt.Errorf("publicprofile: %s preview reference is missing or ambiguous", slot.name)
		}
		before := matches[0]
		after := base + "/" + slot.file + "?v=" + key
		if before != after {
			replaced := strings.Replace(out, prefix+trimToKey(before, prefix), prefix+key, 1)
			if replaced == out {
				return "", nil, fmt.Errorf("publicprofile: could not update %s", slot.name)
			}
			out = replaced
		}
		edits = append(edits, ReadmeEdit{Slot: slot.name, Before: before, After: after})
	}
	if strings.Count(out, "preview-dark.svg") != 1 || strings.Count(out, "preview-light.svg") != 2 {
		return "", nil, errors.New("publicprofile: README has more than one whereToken preview picture")
	}
	return out, edits, nil
}

func findCacheURLs(src, prefix string) []string {
	var found []string
	rest := src
	for {
		i := strings.Index(rest, prefix)
		if i < 0 {
			return found
		}
		start := i + len(slotAttr(prefix))
		// prefix includes attr=", URL starts at the pages host.
		urlStart := i + strings.Index(rest[i:], `="`) + 2
		end := strings.Index(rest[urlStart:], `"`)
		if end < 0 {
			return found
		}
		full := rest[urlStart : urlStart+end]
		if cacheKeyOf(full) == "" {
			return append(found, full)
		}
		found = append(found, full)
		rest = rest[urlStart+end+1:]
		_ = start
	}
}

func slotAttr(prefix string) string { return prefix }

func trimToKey(full, prefix string) string {
	attrEnd := strings.Index(prefix, `="`)
	if attrEnd < 0 {
		return ""
	}
	urlPrefix := prefix[attrEnd+2:]
	return strings.TrimPrefix(full, urlPrefix)
}

func cacheKeyOf(full string) string {
	i := strings.LastIndex(full, "?v=")
	if i < 0 {
		return ""
	}
	key := full[i+3:]
	parts := strings.Split(key, "-")
	if len(parts) != 2 || !isHex64(parts[0]) || !isHex64(parts[1]) {
		return ""
	}
	return key
}

type PlanInput struct {
	Palette           string
	BundleDir         string
	Now               time.Time
	LivePalette       string
	LiveSnapshotID    string
	LiveAssetRevision string
	LiveProvenance    string
	Readme            string
	ReadmeSHA         string
	ReadmeBlob        string
	Git               GitView
	Login             string
	ProductCanPush    bool
	ProfileCanPush    bool
	Protection        string
	GhNote            string
	Config            PublishConfig
}

type GitView struct {
	Head      string
	Origin    string
	Relation  string
	Ahead     []string
	Porcelain string
	Staged    string
	RemoteURL string
	Toplevel  string
}

func AssemblePlan(in PlanInput) (Preflight, error) {
	if err := ValidatePalette(in.Palette); err != nil {
		return Preflight{}, err
	}
	cfg := normalizePublishConfig(in.Config)
	if err := ValidatePublishConfig(cfg); err != nil {
		return Preflight{}, err
	}
	out := Preflight{
		ID:               newID(),
		Palette:          in.Palette,
		LivePalette:      in.LivePalette,
		ProductRepo:      cfg.ProductRepo,
		ProductBranch:    cfg.ProductBranch,
		ProfileRepo:      cfg.ProfileRepo,
		ProfileBranch:    cfg.ProfileBranch,
		ReadmePath:       cfg.ProfileReadme,
		PagesURL:         strings.TrimRight(cfg.PagesURL, "/"),
		BundleRel:        "public-profile",
		GitHubLogin:      in.Login,
		ProductPushRight: in.ProductCanPush,
		ProfilePushRight: in.ProfileCanPush,
		BranchProtection: in.Protection,
		StaticSVG:        true,
		ReleaseNote:      "这次确认不包含 v0.7.4。安装包要另外的明确同意。",
		ReadmeSHA:        in.ReadmeSHA,
		ReadmeBlob:       in.ReadmeBlob,
		Checkout:         cfg.Checkout,
		BundleDir:        in.BundleDir,
		Head:             in.Git.Head,
		Ahead:            in.Git.Ahead,
	}
	var blockers []string
	if in.GhNote != "" {
		blockers = append(blockers, in.GhNote)
	}
	if in.Login != "" && (!in.ProductCanPush || !in.ProfileCanPush) {
		blockers = append(blockers, "当前 GitHub 账号对两个目标仓库都需要 push 权限。")
	}
	if in.Login == "" && in.GhNote == "" {
		blockers = append(blockers, "没有读到 GitHub 登录名。先运行 gh auth login。")
	}
	if !remoteMatches(in.Git.RemoteURL, cfg.ProductRepo) {
		blockers = append(blockers, "本机仓库的 origin 不是配置的产品仓库。")
	}
	if in.Git.Toplevel != "" && cfg.Checkout != "" && filepath.Clean(in.Git.Toplevel) != filepath.Clean(cfg.Checkout) {
		blockers = append(blockers, "本机检出目录和 origin 不一致。")
	}
	switch in.Git.Relation {
	case "equal", "ahead":
	case "behind":
		blockers = append(blockers, "origin/main 比本机新。先快进更新，再发布。")
	case "diverged":
		blockers = append(blockers, "本机和 origin/main 已分叉。不会强行推送。")
	default:
		blockers = append(blockers, "还没有确认本机和 origin/main 的关系。")
	}
	if strings.TrimSpace(in.Git.Staged) != "" {
		blockers = append(blockers, "索引里已有暂存文件。先提交或取消暂存后再发布。")
	}
	dirty := porcelainPaths(in.Git.Porcelain)
	var otherDirty []string
	for _, p := range dirty {
		if p == "public-profile" || strings.HasPrefix(p, "public-profile/") {
			blockers = append(blockers, "public-profile 里已有未提交的改动。发布不会覆盖它们。")
			break
		}
		otherDirty = append(otherDirty, p)
	}
	out.Dirty = otherDirty

	snap, err := LoadFile(in.BundleDir)
	if err != nil {
		return Preflight{}, err
	}
	if err := ValidateProduction(snap, false); err != nil {
		blockers = append(blockers, "生产校验没有通过："+Redact(err.Error()))
		out.Validation = "failed"
	} else {
		out.Validation = "ok"
	}
	out.Provenance = snap.Provenance.Kind
	out.SnapshotID = snap.SnapshotID
	curPres, err := ReadBundlePresentation(in.BundleDir)
	if err != nil {
		return Preflight{}, err
	}
	curPres = CoercePresentation(curPres)
	out.LocalPalette = curPres.PublicPalette
	before, err := os.ReadFile(filepath.Join(in.BundleDir, "presentation.json"))
	if err != nil {
		return Preflight{}, err
	}
	out.PresentationBefore = string(before)
	nextPres := curPres
	if curPres.PublicPalette != in.Palette {
		nextPres.PublicPalette = in.Palette
		nextPres.Revision = nextRevision(curPres.Revision)
		if !in.Now.IsZero() {
			nextPres.UpdatedAt = in.Now.UTC().Format(time.RFC3339)
		}
	}
	nextPres = CoercePresentation(nextPres)
	files, err := BundleWith(snap, nextPres)
	if err != nil {
		return Preflight{}, err
	}
	if filesSnap := snapshotFromJSON(files["profile.json"]); filesSnap != snap.SnapshotID {
		blockers = append(blockers, "重新生成改变了 snapshot_id。发布已停。")
	}
	out.AssetRevision = bundleAssetRevision(files)
	key, err := CacheKey(snap.SnapshotID, out.AssetRevision)
	if err != nil {
		return Preflight{}, err
	}
	out.CacheKey = key
	out.LiveAssetRevision = in.LiveAssetRevision
	after := files["presentation.json"]
	out.PresentationAfter = string(after)
	var changes []FileChange
	for _, name := range GeneratedFiles {
		disk, readErr := os.ReadFile(filepath.Join(in.BundleDir, filepath.FromSlash(name)))
		action := "unchanged"
		if readErr != nil || string(disk) != string(files[name]) {
			action = "update"
			out.ProductCommit = true
		}
		if action == "update" {
			changes = append(changes, FileChange{Path: "public-profile/" + name, Action: action})
		}
	}
	out.Files = changes
	if in.Git.Relation == "ahead" {
		out.ProductPush = true
	}
	if out.ProductCommit {
		out.ProductPush = true
	}
	readme, edits, rerr := RewriteProfileReadme(in.Readme, cfg.PagesURL, snap.SnapshotID, out.AssetRevision)
	if rerr != nil {
		blockers = append(blockers, Redact(rerr.Error()))
	} else {
		out.ReadmeEdits = edits
		for _, edit := range edits {
			if edit.Before != edit.After {
				out.ReadmeChange = true
			}
		}
		_ = readme
	}
	if in.LivePalette != "" && in.LivePalette != in.Palette && !out.ProductPush {
		blockers = append(blockers, "线上配色和这次选择不同，但本机没有可推送的提交。")
	}
	out.Blockers = blockers
	if len(blockers) > 0 {
		out.Phase = PhasePreflight
		out.Ready = false
	} else if !out.ProductPush && !out.ReadmeChange {
		out.Phase = PhaseVerified
		out.Idempotent = true
		out.Ready = false
	} else {
		out.Phase = PhaseAwaitingApproval
		out.Ready = true
	}
	out.PhaseLabel = PhaseLabel(out.Phase)
	return out, nil
}

func snapshotFromJSON(raw []byte) string {
	var body struct {
		SnapshotID string `json:"snapshot_id"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		return ""
	}
	return body.SnapshotID
}

func porcelainPaths(s string) []string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		if strings.TrimSpace(line) == "" || len(line) < 4 {
			continue
		}
		p := strings.TrimSpace(line[3:])
		if strings.HasPrefix(p, `"`) {
			continue
		}
		if i := strings.LastIndex(p, " -> "); i >= 0 {
			p = strings.TrimSpace(p[i+4:])
		}
		if p != "" {
			out = append(out, filepath.ToSlash(p))
		}
	}
	return out
}

func remoteMatches(remote, repo string) bool {
	remote = strings.TrimSpace(remote)
	remote = strings.TrimSuffix(remote, ".git")
	remote = strings.TrimSuffix(remote, "/")
	return strings.HasSuffix(remote, "/"+repo) || strings.HasSuffix(remote, ":"+repo)
}

func newID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return hex.EncodeToString([]byte(time.Now().UTC().Format(time.RFC3339Nano)))
	}
	return hex.EncodeToString(b[:])
}
