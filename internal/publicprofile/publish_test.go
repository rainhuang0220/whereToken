package publicprofile

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const sampleReadme = `<p align="center">
  <a href="https://rainhuang0220.github.io/whereToken/profile/">
    <picture>
      <source media="(prefers-color-scheme: dark)" srcset="https://rainhuang0220.github.io/whereToken/profile/preview-dark.svg?v=15c96d356fc90d8073524036f29bef83ef236f76f7b02a11e862b9cbb4c52e62-2f221b6893380bf9ebaecd0cce4fe8bf2abea777805c8dac1c88d9d34c4245cc">
      <source media="(prefers-color-scheme: light)" srcset="https://rainhuang0220.github.io/whereToken/profile/preview-light.svg?v=15c96d356fc90d8073524036f29bef83ef236f76f7b02a11e862b9cbb4c52e62-2f221b6893380bf9ebaecd0cce4fe8bf2abea777805c8dac1c88d9d34c4245cc">
      <img src="https://rainhuang0220.github.io/whereToken/profile/preview-light.svg?v=15c96d356fc90d8073524036f29bef83ef236f76f7b02a11e862b9cbb4c52e62-2f221b6893380bf9ebaecd0cce4fe8bf2abea777805c8dac1c88d9d34c4245cc" width="800" alt="Coding activity snapshot">
    </picture>
  </a>
</p>
`

func TestRewriteProfileReadmeUpdatesOnlyThreePreviewURLs(t *testing.T) {
	const snap = "15c96d356fc90d8073524036f29bef83ef236f76f7b02a11e862b9cbb4c52e62"
	const asset = "3e30d4babdb260f4570aa1fea89a21ff9f00376c190a0615cdf6cdbfa0d76fe5"
	got, edits, err := RewriteProfileReadme(sampleReadme, "https://rainhuang0220.github.io/whereToken/profile/", "sha256:"+snap, "sha256:"+asset)
	if err != nil {
		t.Fatal(err)
	}
	if len(edits) != 3 {
		t.Fatalf("edits %d", len(edits))
	}
	key := snap + "-" + asset
	if strings.Count(got, key) != 3 {
		t.Fatalf("cache key count\n%s", got)
	}
	if strings.Contains(got, "2f221b6893380bf9ebaecd0cce4fe8bf2abea777805c8dac1c88d9d34c4245cc") {
		t.Fatal("old asset revision remained")
	}
	for _, want := range []string{
		`media="(prefers-color-scheme: dark)"`,
		`media="(prefers-color-scheme: light)"`,
		`width="800"`,
		`alt="Coding activity snapshot"`,
		`href="https://rainhuang0220.github.io/whereToken/profile/"`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("lost %s", want)
		}
	}
	again, _, err := RewriteProfileReadme(got, "https://rainhuang0220.github.io/whereToken/profile/", snap, asset)
	if err != nil {
		t.Fatal(err)
	}
	if again != got {
		t.Fatal("rewrite was not idempotent")
	}
}

func TestRewriteProfileReadmeRejectsAmbiguousPictures(t *testing.T) {
	extra := sampleReadme + `<img src="https://rainhuang0220.github.io/whereToken/profile/preview-light.svg?v=15c96d356fc90d8073524036f29bef83ef236f76f7b02a11e862b9cbb4c52e62-2f221b6893380bf9ebaecd0cce4fe8bf2abea777805c8dac1c88d9d34c4245cc">`
	if _, _, err := RewriteProfileReadme(extra, "https://rainhuang0220.github.io/whereToken/profile/", strings.Repeat("ab", 32), strings.Repeat("cd", 32)); err == nil {
		t.Fatal("expected ambiguous readme to fail")
	}
	if _, _, err := RewriteProfileReadme(sampleReadme, "https://example.com/other/", strings.Repeat("ab", 32), strings.Repeat("cd", 32)); err == nil {
		t.Fatal("expected a foreign pages URL to fail")
	}
}

func TestPublishConfigRejectsInjection(t *testing.T) {
	base := PublishConfig{
		SchemaVersion: 1,
		ProductRepo:   "rainhuang0220/whereToken",
		ProductBranch: "main",
		ProfileRepo:   "rainhuang0220/rainhuang0220",
		ProfileBranch: "main",
		ProfileReadme: "README.md",
		PagesURL:      "https://rainhuang0220.github.io/whereToken/profile/",
	}
	bad := []PublishConfig{
		withRepo(base, "product", "rainhuang0220/whereToken;rm"),
		withRepo(base, "profile", "../rainhuang0220"),
		withRepo(base, "branch", "main && touch /tmp/x"),
		withRepo(base, "readme", "README.md;id"),
		withRepo(base, "pages", "http://rainhuang0220.github.io/whereToken/profile/"),
		withRepo(base, "pages", "https://user:token@rainhuang0220.github.io/whereToken/profile/"),
	}
	for _, cfg := range bad {
		if err := ValidatePublishConfig(normalizePublishConfig(cfg)); err == nil {
			t.Fatalf("accepted %#v", cfg)
		}
	}
	if err := ValidatePublishConfig(normalizePublishConfig(base)); err != nil {
		t.Fatal(err)
	}
}

func withRepo(cfg PublishConfig, field, value string) PublishConfig {
	switch field {
	case "product":
		cfg.ProductRepo = value
	case "profile":
		cfg.ProfileRepo = value
	case "branch":
		cfg.ProductBranch = value
	case "readme":
		cfg.ProfileReadme = value
	case "pages":
		cfg.PagesURL = value
	}
	return cfg
}

func TestAssemblePlanKeepsSnapshotAndRequiresApproval(t *testing.T) {
	dir := copyPublicBundle(t)
	in := readyPlan(dir)
	in.Palette = "magenta"
	got, err := AssemblePlan(in)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Ready || got.Phase != PhaseAwaitingApproval {
		t.Fatalf("phase %s ready %v blockers %v", got.Phase, got.Ready, got.Blockers)
	}
	if got.SnapshotID != "sha256:15c96d356fc90d8073524036f29bef83ef236f76f7b02a11e862b9cbb4c52e62" {
		t.Fatalf("snapshot %s", got.SnapshotID)
	}
	if got.LocalPalette != "newsprint" || got.Palette != "magenta" {
		t.Fatalf("palettes local %s selected %s", got.LocalPalette, got.Palette)
	}
	if !got.ProductCommit || !got.ReadmeChange {
		t.Fatalf("commit %v readme %v", got.ProductCommit, got.ReadmeChange)
	}
	if strings.Contains(got.PresentationAfter, got.Checkout) || strings.Contains(mustJSON(t, got), "/Users/") {
		t.Fatal("preflight leaked a local path")
	}
	if strings.Contains(mustJSON(t, got), "ghp_") {
		t.Fatal("preflight leaked a token")
	}
	same := in
	same.Palette = "newsprint"
	same.Readme = mustCurrentReadme(t, dir)
	again, err := AssemblePlan(same)
	if err != nil {
		t.Fatal(err)
	}
	if again.ProductCommit {
		t.Fatal("identical newsprint bundle asked for a commit")
	}
}

func TestAssemblePlanBlocksDirtyBundleAndBadPalette(t *testing.T) {
	dir := copyPublicBundle(t)
	in := readyPlan(dir)
	in.Git.Porcelain = " M public-profile/presentation.json\n"
	got, err := AssemblePlan(in)
	if err != nil {
		t.Fatal(err)
	}
	if got.Ready || len(got.Blockers) == 0 {
		t.Fatalf("dirty bundle was publishable: %+v", got.Blockers)
	}
	if _, err := AssemblePlan(PlanInput{Palette: "kiln", BundleDir: dir, Config: in.Config}); err == nil {
		t.Fatal("kiln was accepted")
	}
	in = readyPlan(dir)
	in.Git.Staged = "README.md\n"
	got, err = AssemblePlan(in)
	if err != nil {
		t.Fatal(err)
	}
	if got.Ready {
		t.Fatal("staged files were ignored")
	}
}

func TestPreflightDoesNotPush(t *testing.T) {
	dir := copyPublicBundle(t)
	pub, script := scriptedPublisher(t, dir, "newsprint")
	if _, err := pub.Preflight(context.Background(), "magenta"); err != nil {
		t.Fatal(err)
	}
	for _, call := range script.calls {
		if strings.Contains(call, " push ") || strings.Contains(call, "--method PUT") {
			t.Fatalf("preflight mutated a remote: %s", call)
		}
	}
}

func TestApproveStopsWhenPagesFails(t *testing.T) {
	dir := copyPublicBundle(t)
	pub, script := scriptedPublisher(t, dir, "magenta")
	script.pages = "failure"
	job, err := pub.Approve(context.Background(), "magenta", nil)
	if err == nil {
		t.Fatal("expected pages failure")
	}
	if job.Phase != PhaseFailed {
		t.Fatalf("phase %s", job.Phase)
	}
	for _, call := range script.calls {
		if strings.Contains(call, "--method PUT") {
			t.Fatal("readme was updated after pages failed")
		}
	}
}

func TestApprovePartialReadmeCanRetry(t *testing.T) {
	dir := copyPublicBundle(t)
	pub, script := scriptedPublisher(t, dir, "magenta")
	script.readmePut = "conflict"
	job, err := pub.Approve(context.Background(), "magenta", nil)
	if err == nil || job.Phase != PhasePartiallyPublished || !job.RetryReadme {
		t.Fatalf("job %+v err %v", job, err)
	}
	productPushes := 0
	for _, call := range script.calls {
		if strings.Contains(call, " push ") {
			productPushes++
		}
	}
	script.readmePut = ""
	script.calls = nil
	job, err = pub.RetryReadme(context.Background(), "magenta", nil)
	if err != nil || job.Phase != PhaseVerified {
		t.Fatalf("retry %+v err %v", job, err)
	}
	for _, call := range script.calls {
		if strings.Contains(call, " push ") {
			t.Fatal("readme retry pushed the product repository again")
		}
	}
	if productPushes != 1 {
		t.Fatalf("pushes %d", productPushes)
	}
}

func TestRedactStripsTokens(t *testing.T) {
	got := Redact("ghp_abcdefghijklmnopqrstuvwxyz0123456789 refused")
	if strings.Contains(got, "ghp_") {
		t.Fatal(got)
	}
}

func readyPlan(dir string) PlanInput {
	repo := filepath.Dir(dir)
	return PlanInput{
		Palette:           "newsprint",
		BundleDir:         dir,
		LivePalette:       "newsprint",
		LiveSnapshotID:    "sha256:15c96d356fc90d8073524036f29bef83ef236f76f7b02a11e862b9cbb4c52e62",
		LiveAssetRevision: "sha256:3e30d4babdb260f4570aa1fea89a21ff9f00376c190a0615cdf6cdbfa0d76fe5",
		LiveProvenance:    ProvenanceLocal,
		Readme:            sampleReadme,
		ReadmeSHA:         "572dd8e4e037e2c194716cdcecb857d2cf9a1cdd",
		ReadmeBlob:        "424db286f25f4360ad3e2dc7705984f7e9db1147",
		Login:             "rainhuang0220",
		ProductCanPush:    true,
		ProfileCanPush:    true,
		Protection:        "none",
		Git: GitView{
			Head:      "520c391816f690bbecde29c92ac8dd4cc9ef5f33",
			Origin:    "520c391816f690bbecde29c92ac8dd4cc9ef5f33",
			Relation:  "equal",
			RemoteURL: "https://github.com/rainhuang0220/whereToken.git",
			Toplevel:  repo,
		},
		Config: PublishConfig{
			SchemaVersion: 1,
			ProductRepo:   "rainhuang0220/whereToken",
			ProductBranch: "main",
			ProfileRepo:   "rainhuang0220/rainhuang0220",
			ProfileBranch: "main",
			ProfileReadme: "README.md",
			PagesURL:      "https://rainhuang0220.github.io/whereToken/profile/",
			Checkout:      repo,
		},
	}
}

func mustCurrentReadme(t *testing.T, dir string) string {
	t.Helper()
	in := readyPlan(dir)
	in.Palette = "newsprint"
	got, err := AssemblePlan(in)
	if err != nil {
		t.Fatal(err)
	}
	text, _, err := RewriteProfileReadme(sampleReadme, in.Config.PagesURL, got.SnapshotID, got.AssetRevision)
	if err != nil {
		t.Fatal(err)
	}
	return text
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func copyPublicBundle(t *testing.T) string {
	t.Helper()
	src := filepath.Join("..", "..", "public-profile")
	repo := filepath.Join(t.TempDir(), "repo")
	dst := filepath.Join(repo, "public-profile")
	err := filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		out, err := os.Create(target)
		if err != nil {
			return err
		}
		defer out.Close()
		_, err = io.Copy(out, in)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return dst
}

type scriptRunner struct {
	t         *testing.T
	repo      string
	bundle    string
	calls     []string
	pages     string
	readmePut string
	pushed    bool
	head      string
	readme    string
	asset     string
	palette   string
}

func scriptedPublisher(t *testing.T, bundle, palette string) (*Publisher, *scriptRunner) {
	t.Helper()
	repo := filepath.Dir(bundle)
	script := &scriptRunner{
		t: t, repo: repo, bundle: bundle,
		head: "520c391816f690bbecde29c92ac8dd4cc9ef5f33", readme: sampleReadme,
		palette: "newsprint", asset: "sha256:3e30d4babdb260f4570aa1fea89a21ff9f00376c190a0615cdf6cdbfa0d76fe5",
	}
	pub := &Publisher{
		Now:    timeZero,
		Run:    script,
		Get:    script,
		Sleep:  func(time.Duration) {},
		Getwd:  func() (string, error) { return repo, nil },
		LookUp: func(string) error { return nil },
	}
	cfg := PublishConfig{
		SchemaVersion: 1,
		ProductRepo:   "rainhuang0220/whereToken",
		ProductBranch: "main",
		ProfileRepo:   "rainhuang0220/rainhuang0220",
		ProfileBranch: "main",
		ProfileReadme: "README.md",
		PagesURL:      "https://rainhuang0220.github.io/whereToken/profile/",
		Checkout:      repo,
	}
	path := filepath.Join(t.TempDir(), "profile-publish.json")
	t.Setenv("WHERETOKEN_PROFILE_PUBLISH_FILE", path)
	if err := SavePublishConfig(path, cfg); err != nil {
		t.Fatal(err)
	}
	_ = palette
	return pub, script
}

func timeZero() time.Time { return time.Date(2026, 9, 23, 0, 0, 0, 0, time.UTC) }

func (s *scriptRunner) Run(_ context.Context, name string, args []string, stdin string) (string, error) {
	call := name + " " + strings.Join(args, " ")
	s.calls = append(s.calls, call)
	switch {
	case strings.Contains(call, "rev-parse --show-toplevel"):
		return s.repo, nil
	case strings.Contains(call, "remote get-url"):
		return "git@github.com:rainhuang0220/whereToken.git", nil
	case strings.Contains(call, "api user"):
		return "rainhuang0220", nil
	case strings.Contains(call, "permissions.push"):
		return "true", nil
	case strings.Contains(call, "protection"):
		return "", os.ErrNotExist
	case strings.Contains(call, "/commits/"):
		return "572dd8e4e037e2c194716cdcecb857d2cf9a1cdd", nil
	case strings.Contains(call, "/contents/") && !strings.Contains(call, "--method"):
		raw, _ := json.Marshal(map[string]string{
			"sha":     "424db286f25f4360ad3e2dc7705984f7e9db1147",
			"content": base64.StdEncoding.EncodeToString([]byte(s.readme)),
		})
		return string(raw), nil
	case strings.Contains(call, "fetch"):
		return "", nil
	case strings.Contains(call, "rev-parse HEAD"), strings.Contains(call, "rev-parse origin/"):
		return s.head, nil
	case strings.Contains(call, "status --porcelain"), strings.Contains(call, "diff --cached"):
		return "", nil
	case strings.Contains(call, " add "):
		return "", nil
	case strings.Contains(call, " commit "):
		s.head = "abc1234567890abc1234567890abc1234567890a"
		return "", nil
	case strings.Contains(call, " push "):
		s.pushed = true
		return "", nil
	case strings.Contains(call, "run list"):
		conclusion := "success"
		if s.pages == "failure" {
			conclusion = "failure"
		}
		raw, _ := json.Marshal([]map[string]string{{
			"status": "completed", "conclusion": conclusion, "headSha": s.head, "url": "https://github.com/rainhuang0220/whereToken/actions/runs/1", "event": "push",
		}})
		return string(raw), nil
	case strings.Contains(call, "--method PUT"):
		if s.readmePut == "conflict" {
			return "", os.ErrExist
		}
		var body struct {
			Content string `json:"content"`
		}
		if err := json.Unmarshal([]byte(stdin), &body); err != nil {
			return "", err
		}
		decoded, err := base64.StdEncoding.DecodeString(body.Content)
		if err != nil {
			return "", err
		}
		s.readme = string(decoded)
		raw, _ := json.Marshal(map[string]any{"commit": map[string]string{"sha": "9999dd8e4e037e2c194716cdcecb857d2cf9a1cdd"}})
		return string(raw), nil
	default:
		s.t.Fatalf("unexpected command %s", call)
		return "", os.ErrInvalid
	}
}

func (s *scriptRunner) Get(_ context.Context, rawURL string) ([]byte, int, error) {
	s.calls = append(s.calls, "GET "+rawURL)
	if s.pushed {
		name := "presentation.json"
		if strings.HasSuffix(rawURL, "manifest.json") {
			name = "manifest.json"
		}
		raw, err := os.ReadFile(filepath.Join(s.bundle, name))
		return raw, 200, err
	}
	if strings.HasSuffix(rawURL, "presentation.json") {
		raw, _ := json.Marshal(map[string]any{"schema_version": 1, "public_palette": s.palette, "revision": "0"})
		return raw, 200, nil
	}
	if strings.HasSuffix(rawURL, "manifest.json") {
		raw, _ := json.Marshal(map[string]string{
			"snapshot_id":    "sha256:15c96d356fc90d8073524036f29bef83ef236f76f7b02a11e862b9cbb4c52e62",
			"asset_revision": s.asset,
			"provenance":     ProvenanceLocal,
			"public_palette": s.palette,
		})
		return raw, 200, nil
	}
	return nil, 404, os.ErrNotExist
}
