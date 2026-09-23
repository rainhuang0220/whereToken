package publicprofile

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/rainhuang0220/whereToken/internal/adapter"
)

// Runner runs git or gh with a fixed argument vector. It must not invoke a shell.
type Runner interface {
	Run(ctx context.Context, name string, args []string, stdin string) (string, error)
}

// Getter reads a public URL. The publisher only asks for its configured Pages origin.
type Getter interface {
	Get(ctx context.Context, rawURL string) ([]byte, int, error)
}

type Publisher struct {
	Home   adapter.Home
	Now    func() time.Time
	Run    Runner
	Get    Getter
	Sleep  func(time.Duration)
	Getwd  func() (string, error)
	LookUp func(string) error
}

type ExecRunner struct{}

func (ExecRunner) Run(ctx context.Context, name string, args []string, stdin string) (string, error) {
	if name != "git" && name != "gh" {
		return "", fmt.Errorf("publicprofile: refused to run %s", name)
	}
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdin = strings.NewReader(stdin)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "GH_PROMPT_DISABLED=1")
	err := cmd.Run()
	out := strings.TrimSpace(stdout.String())
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = out
		}
		return "", fmt.Errorf("%s", Redact(msg))
	}
	return out, nil
}

type HTTPGetter struct {
	Client *http.Client
}

func (g HTTPGetter) Get(ctx context.Context, rawURL string) ([]byte, int, error) {
	client := g.Client
	if client == nil {
		client = http.DefaultClient
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Cache-Control", "no-cache")
	res, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 8<<20))
	if err != nil {
		return nil, res.StatusCode, err
	}
	return body, res.StatusCode, nil
}

func NewPublisher(home adapter.Home) *Publisher {
	return &Publisher{
		Home:  home,
		Now:   time.Now,
		Run:   ExecRunner{},
		Get:   HTTPGetter{Client: &http.Client{Timeout: 20 * time.Second}},
		Sleep: time.Sleep,
		Getwd: os.Getwd,
		LookUp: func(name string) error {
			_, err := exec.LookPath(name)
			return err
		},
	}
}

func (p *Publisher) now() time.Time {
	if p.Now != nil {
		return p.Now()
	}
	return time.Now()
}

func (p *Publisher) sleep(d time.Duration) {
	if p.Sleep != nil && d > 0 {
		p.Sleep(d)
	}
}

func (p *Publisher) run(ctx context.Context, name string, args ...string) (string, error) {
	if p.Run == nil {
		return "", errors.New("publicprofile: publisher has no command runner")
	}
	return p.Run.Run(ctx, name, args, "")
}

func (p *Publisher) Preflight(ctx context.Context, palette string) (Preflight, error) {
	if err := ValidatePalette(palette); err != nil {
		return Preflight{}, err
	}
	cfg, err := p.prepareConfig()
	if err != nil {
		return blockedPreflight(palette, err.Error()), nil
	}
	checkout, err := p.resolveCheckout(ctx, cfg)
	if err != nil {
		return blockedPreflight(palette, err.Error()), nil
	}
	cfg.Checkout = checkout
	return p.assemble(ctx, cfg, palette)
}

func (p *Publisher) prepareConfig() (PublishConfig, error) {
	path := PublishConfigPath(p.Home)
	cfg, err := LoadPublishConfig(path)
	if err != nil {
		if os.IsNotExist(err) {
			return PublishConfig{}, errors.New("还没有发布配置。在本机运行 wheretoken profile publish --product owner/whereToken --profile-repo owner/owner --pages https://owner.github.io/whereToken/profile/")
		}
		return PublishConfig{}, err
	}
	return cfg, nil
}

func (p *Publisher) SaveConfig(cfg PublishConfig) error {
	cfg = normalizePublishConfig(cfg)
	if err := ValidatePublishConfig(cfg); err != nil {
		return err
	}
	return SavePublishConfig(PublishConfigPath(p.Home), cfg)
}

func blockedPreflight(palette, msg string) Preflight {
	return Preflight{
		ID:          newID(),
		Phase:       PhasePreflight,
		PhaseLabel:  PhaseLabel(PhasePreflight),
		Palette:     palette,
		StaticSVG:   true,
		ReleaseNote: "这次确认不包含 v0.7.4。安装包要另外的明确同意。",
		Blockers:    []string{Redact(msg)},
	}
}

func (p *Publisher) resolveCheckout(ctx context.Context, cfg PublishConfig) (string, error) {
	if strings.TrimSpace(cfg.Checkout) != "" {
		return p.verifyCheckout(ctx, cfg.Checkout, cfg.ProductRepo)
	}
	wd, err := p.Getwd()
	if err != nil {
		return "", errors.New("没有配置 whereToken 检出目录，也读不到当前目录。")
	}
	dir := wd
	for {
		if st, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil && !st.IsDir() {
			if _, err := p.verifyCheckout(ctx, dir, cfg.ProductRepo); err == nil {
				return dir, nil
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", errors.New("没有找到 origin 与配置一致的 whereToken 检出。用 wheretoken profile publish --checkout 指定一次。")
}

func (p *Publisher) verifyCheckout(ctx context.Context, dir, repo string) (string, error) {
	top, err := p.run(ctx, "git", "-C", dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", errors.New("这个目录不是 Git 仓库。")
	}
	top = filepath.Clean(top)
	if filepath.Clean(dir) != top {
		dir = top
	}
	remote, err := p.run(ctx, "git", "-C", dir, "remote", "get-url", "origin")
	if err != nil || !remoteMatches(remote, repo) {
		return "", errors.New("origin 与配置的产品仓库不一致。")
	}
	if !IsBundle(filepath.Join(dir, "public-profile")) {
		return "", errors.New("检出里没有 public-profile/profile.json。")
	}
	return dir, nil
}

func (p *Publisher) assemble(ctx context.Context, cfg PublishConfig, palette string) (Preflight, error) {
	in := PlanInput{
		Palette:   palette,
		BundleDir: filepath.Join(cfg.Checkout, "public-profile"),
		Now:       p.now(),
		Config:    cfg,
	}
	if p.LookUp != nil {
		if err := p.LookUp("gh"); err != nil {
			in.GhNote = "没有找到 gh。安装 GitHub CLI 并 gh auth login 之后才能发布。只保存在本机不需要它。"
		}
	}
	if in.GhNote == "" {
		login, err := p.run(ctx, "gh", "api", "user", "--jq", ".login")
		if err != nil {
			in.GhNote = "gh 还没有登录。运行 gh auth login。只保存在本机不需要它。"
		} else {
			in.Login = login
		}
	}
	if in.Login != "" {
		in.ProductCanPush = p.canPush(ctx, cfg.ProductRepo)
		in.ProfileCanPush = p.canPush(ctx, cfg.ProfileRepo)
		in.Protection = p.protection(ctx, cfg.ProductRepo, cfg.ProductBranch)
		readme, sha, blob, err := p.readReadme(ctx, cfg)
		if err != nil {
			in.GhNote = "读不到个人主页 README：" + Redact(err.Error())
		} else {
			in.Readme = readme
			in.ReadmeSHA = sha
			in.ReadmeBlob = blob
		}
	}
	in.Git = p.gitView(ctx, cfg)
	if live, err := p.live(ctx, cfg.PagesURL); err == nil {
		in.LivePalette = live.Palette
		in.LiveSnapshotID = live.SnapshotID
		in.LiveAssetRevision = live.Asset
		in.LiveProvenance = live.Provenance
	} else {
		in.GhNote = strings.TrimSpace(in.GhNote + " 读不到线上 presentation.json。")
	}
	return AssemblePlan(in)
}

type liveState struct {
	Palette    string
	SnapshotID string
	Asset      string
	Provenance string
}

func (p *Publisher) live(ctx context.Context, pagesURL string) (liveState, error) {
	base := strings.TrimRight(pagesURL, "/")
	body, status, err := p.get(ctx, base+"/presentation.json")
	if err != nil || status != http.StatusOK {
		return liveState{}, errors.New("presentation")
	}
	pres, err := ParsePresentation(body)
	if err != nil {
		return liveState{}, err
	}
	manBody, status, err := p.get(ctx, base+"/manifest.json")
	if err != nil || status != http.StatusOK {
		return liveState{}, errors.New("manifest")
	}
	var man struct {
		AssetRevision string `json:"asset_revision"`
		SnapshotID    string `json:"snapshot_id"`
		Provenance    string `json:"provenance"`
		PublicPalette string `json:"public_palette"`
	}
	if err := json.Unmarshal(manBody, &man); err != nil {
		return liveState{}, err
	}
	return liveState{Palette: pres.PublicPalette, SnapshotID: man.SnapshotID, Asset: man.AssetRevision, Provenance: man.Provenance}, nil
}

func (p *Publisher) get(ctx context.Context, rawURL string) ([]byte, int, error) {
	if p.Get == nil {
		return nil, 0, errors.New("no getter")
	}
	return p.Get.Get(ctx, rawURL)
}

func (p *Publisher) canPush(ctx context.Context, repo string) bool {
	out, err := p.run(ctx, "gh", "api", "repos/"+repo, "--jq", ".permissions.push")
	return err == nil && out == "true"
}

func (p *Publisher) protection(ctx context.Context, repo, branch string) string {
	_, err := p.run(ctx, "gh", "api", "repos/"+repo+"/branches/"+branch+"/protection", "--jq", ".url")
	if err != nil {
		return "未读到分支保护，或该分支没有保护规则。直接推送若被拒绝会停住，不会 force push。"
	}
	return "该分支有保护规则。若它要求 pull request，推送会失败并停住，不会绕过。"
}

func (p *Publisher) readReadme(ctx context.Context, cfg PublishConfig) (text, commitSHA, blob string, err error) {
	commitSHA, err = p.run(ctx, "gh", "api", "repos/"+cfg.ProfileRepo+"/commits/"+cfg.ProfileBranch, "--jq", ".sha")
	if err != nil {
		return "", "", "", err
	}
	raw, err := p.run(ctx, "gh", "api", "repos/"+cfg.ProfileRepo+"/contents/"+cfg.ProfileReadme+"?ref="+cfg.ProfileBranch, "--jq", "{sha:.sha,content:.content}")
	if err != nil {
		return "", "", "", err
	}
	var body struct {
		SHA     string `json:"sha"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal([]byte(raw), &body); err != nil {
		return "", "", "", err
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(body.Content, "\n", ""))
	if err != nil {
		return "", "", "", err
	}
	return string(decoded), commitSHA, body.SHA, nil
}

func (p *Publisher) gitView(ctx context.Context, cfg PublishConfig) GitView {
	dir := cfg.Checkout
	view := GitView{}
	view.Toplevel, _ = p.run(ctx, "git", "-C", dir, "rev-parse", "--show-toplevel")
	view.RemoteURL, _ = p.run(ctx, "git", "-C", dir, "remote", "get-url", "origin")
	_, _ = p.run(ctx, "git", "-C", dir, "fetch", "origin", cfg.ProductBranch)
	view.Head, _ = p.run(ctx, "git", "-C", dir, "rev-parse", "HEAD")
	view.Origin, _ = p.run(ctx, "git", "-C", dir, "rev-parse", "origin/"+cfg.ProductBranch)
	view.Porcelain, _ = p.run(ctx, "git", "-C", dir, "status", "--porcelain")
	view.Staged, _ = p.run(ctx, "git", "-C", dir, "diff", "--cached", "--name-only")
	if view.Head == "" || view.Origin == "" {
		view.Relation = "unknown"
		return view
	}
	if view.Head == view.Origin {
		view.Relation = "equal"
		return view
	}
	ahead, _ := p.run(ctx, "git", "-C", dir, "merge-base", "--is-ancestor", view.Origin, view.Head)
	_ = ahead
	originIsAncestor := p.ancestor(ctx, dir, view.Origin, view.Head)
	headIsAncestor := p.ancestor(ctx, dir, view.Head, view.Origin)
	switch {
	case originIsAncestor && !headIsAncestor:
		view.Relation = "ahead"
		log, _ := p.run(ctx, "git", "-C", dir, "log", "--format=%h %s", view.Origin+".."+view.Head)
		if log != "" {
			view.Ahead = strings.Split(log, "\n")
		}
	case headIsAncestor && !originIsAncestor:
		view.Relation = "behind"
	default:
		view.Relation = "diverged"
	}
	return view
}

func (p *Publisher) ancestor(ctx context.Context, dir, older, newer string) bool {
	_, err := p.run(ctx, "git", "-C", dir, "merge-base", "--is-ancestor", older, newer)
	return err == nil
}

// Approve recomputes the plan and performs the remote writes. Callers must
// already have shown the same palette's preflight to the owner.
func (p *Publisher) Approve(ctx context.Context, palette string, report func(Job)) (Job, error) {
	pf, err := p.Preflight(ctx, palette)
	if err != nil {
		return failJob(palette, err.Error()), err
	}
	if !pf.Ready {
		job := jobFrom(pf, PhaseFailed)
		if pf.Idempotent {
			job.Phase = PhaseVerified
			job.PhaseLabel = PhaseLabel(PhaseVerified)
			return job, nil
		}
		if len(pf.Blockers) > 0 {
			job.Error = strings.Join(pf.Blockers, " ")
		}
		if job.Error == "" {
			job.Error = "预检没有通过"
		}
		return job, errors.New(job.Error)
	}
	job := jobFrom(pf, PhaseUpdatingBundle)
	reportJob(report, job)
	if pf.ProductCommit {
		if err := p.writeBundle(pf); err != nil {
			job.Phase = PhaseFailed
			job.PhaseLabel = PhaseLabel(PhaseFailed)
			job.Error = Redact(err.Error())
			return job, err
		}
		job.Phase = PhasePushingProduct
		job.PhaseLabel = PhaseLabel(job.Phase)
		reportJob(report, job)
		sha, err := p.commitBundle(ctx, pf)
		if err != nil {
			job.Phase = PhaseFailed
			job.PhaseLabel = PhaseLabel(PhaseFailed)
			job.Error = Redact(err.Error())
			return job, err
		}
		job.ProductSHA = sha
	} else {
		job.ProductSHA = pf.Head
	}
	if pf.ProductPush {
		job.Phase = PhasePushingProduct
		job.PhaseLabel = PhaseLabel(job.Phase)
		reportJob(report, job)
		if err := p.push(ctx, pf); err != nil {
			job.Phase = PhaseFailed
			job.PhaseLabel = PhaseLabel(PhaseFailed)
			job.Error = "推送 whereToken 被拒绝。没有 force push。" + Redact(err.Error())
			return job, err
		}
		if job.ProductSHA == "" {
			job.ProductSHA, _ = p.run(ctx, "git", "-C", pf.Checkout, "rev-parse", "HEAD")
		}
		job.ProductURL = "https://github.com/" + pf.ProductRepo + "/commit/" + job.ProductSHA
		job.Phase = PhaseWaitingPages
		job.PhaseLabel = PhaseLabel(job.Phase)
		reportJob(report, job)
		runURL, err := p.waitPages(ctx, pf, job.ProductSHA)
		if err != nil {
			job.Phase = PhaseFailed
			job.PhaseLabel = PhaseLabel(PhaseFailed)
			job.PagesRunURL = runURL
			job.Error = Redact(err.Error())
			return job, err
		}
		job.PagesRunURL = runURL
	}
	job.Phase = PhaseLiveVerified
	job.PhaseLabel = PhaseLabel(job.Phase)
	reportJob(report, job)
	live, err := p.live(ctx, pf.PagesURL)
	if err != nil || live.Palette != pf.Palette || live.SnapshotID != pf.SnapshotID || live.Asset != pf.AssetRevision {
		job.Phase = PhaseFailed
		job.PhaseLabel = PhaseLabel(PhaseFailed)
		job.Error = "线上资源还不是这次要发布的配色或 asset_revision。没有改个人主页 README。"
		return job, errors.New(job.Error)
	}
	job.LivePalette = live.Palette
	job.AssetRevision = live.Asset
	job.SnapshotID = live.SnapshotID
	if !pf.ReadmeChange {
		job.Phase = PhaseVerified
		job.PhaseLabel = PhaseLabel(job.Phase)
		job.LivePage = pf.PagesURL + "/"
		return job, nil
	}
	job.Phase = PhaseUpdatingReadme
	job.PhaseLabel = PhaseLabel(job.Phase)
	reportJob(report, job)
	profileSHA, err := p.updateReadme(ctx, pf)
	if err != nil {
		job.Phase = PhasePartiallyPublished
		job.PhaseLabel = PhaseLabel(job.Phase)
		job.RetryReadme = true
		job.Error = "公开页已核验。个人 README 没有写上：" + Redact(err.Error())
		return job, err
	}
	job.ProfileSHA = profileSHA
	job.ProfileURL = "https://github.com/" + pf.ProfileRepo + "/commit/" + profileSHA
	if err := p.verifyReadme(ctx, pf); err != nil {
		job.Phase = PhasePartiallyPublished
		job.PhaseLabel = PhaseLabel(job.Phase)
		job.RetryReadme = true
		job.Error = Redact(err.Error())
		return job, err
	}
	job.Phase = PhaseVerified
	job.PhaseLabel = PhaseLabel(job.Phase)
	job.Error = ""
	job.RetryReadme = false
	job.LivePage = strings.TrimRight(pf.PagesURL, "/") + "/"
	job.PreviewLinks = []string{
		job.LivePage + "preview-dark.svg?v=" + pf.CacheKey,
		job.LivePage + "preview-light.svg?v=" + pf.CacheKey,
	}
	job.CacheKey = pf.CacheKey
	return job, nil
}

// RetryReadme repeats only the personal README stage after a verified product deploy.
func (p *Publisher) RetryReadme(ctx context.Context, palette string, report func(Job)) (Job, error) {
	pf, err := p.Preflight(ctx, palette)
	if err != nil {
		return failJob(palette, err.Error()), err
	}
	if pf.ProductCommit || pf.ProductPush {
		return failJob(palette, "产品仓库还有未推送的改动。先完成那一步，再重试 README。"), errors.New("product not settled")
	}
	live, err := p.live(ctx, pf.PagesURL)
	if err != nil || live.Palette != palette || live.Asset != pf.AssetRevision {
		return failJob(palette, "线上页面还没有这次的资源，不能更新个人主页。"), errors.New("live mismatch")
	}
	job := jobFrom(pf, PhaseUpdatingReadme)
	job.ProductSHA = pf.Head
	job.LivePalette = live.Palette
	reportJob(report, job)
	sha, err := p.updateReadme(ctx, pf)
	if err != nil {
		job.Phase = PhasePartiallyPublished
		job.PhaseLabel = PhaseLabel(job.Phase)
		job.RetryReadme = true
		job.Error = Redact(err.Error())
		return job, err
	}
	job.ProfileSHA = sha
	job.ProfileURL = "https://github.com/" + pf.ProfileRepo + "/commit/" + sha
	if err := p.verifyReadme(ctx, pf); err != nil {
		job.Phase = PhasePartiallyPublished
		job.PhaseLabel = PhaseLabel(job.Phase)
		job.RetryReadme = true
		job.Error = Redact(err.Error())
		return job, err
	}
	job.Phase = PhaseVerified
	job.PhaseLabel = PhaseLabel(job.Phase)
	job.RetryReadme = false
	job.Error = ""
	job.CacheKey = pf.CacheKey
	job.LivePage = strings.TrimRight(pf.PagesURL, "/") + "/"
	return job, nil
}

func (p *Publisher) writeBundle(pf Preflight) error {
	snap, err := LoadFile(pf.BundleDir)
	if err != nil {
		return err
	}
	if snap.SnapshotID != pf.SnapshotID {
		return errors.New("snapshot_id changed before write")
	}
	pres, err := ParsePresentation([]byte(pf.PresentationAfter))
	if err != nil {
		return err
	}
	files, err := BundleWith(snap, pres)
	if err != nil {
		return err
	}
	if bundleAssetRevision(files) != pf.AssetRevision {
		return errors.New("asset revision changed before write")
	}
	for _, name := range GeneratedFiles {
		if name == "profile.json" {
			disk, err := os.ReadFile(filepath.Join(pf.BundleDir, name))
			if err == nil && string(disk) == string(files[name]) {
				continue
			}
			if snapshotFromJSON(files[name]) != snap.SnapshotID {
				return errors.New("refusing to rewrite profile.json with a new snapshot_id")
			}
		}
		disk, err := os.ReadFile(filepath.Join(pf.BundleDir, filepath.FromSlash(name)))
		if err == nil && string(disk) == string(files[name]) {
			continue
		}
		if err := writeAtomic(filepath.Join(pf.BundleDir, filepath.FromSlash(name)), files[name]); err != nil {
			return err
		}
	}
	return nil
}

func (p *Publisher) commitBundle(ctx context.Context, pf Preflight) (string, error) {
	staged, _ := p.run(ctx, "git", "-C", pf.Checkout, "diff", "--cached", "--name-only")
	if strings.TrimSpace(staged) != "" {
		return "", errors.New("index already has staged files")
	}
	var paths []string
	for _, change := range pf.Files {
		if change.Action == "update" {
			paths = append(paths, change.Path)
		}
	}
	if len(paths) == 0 {
		return p.run(ctx, "git", "-C", pf.Checkout, "rev-parse", "HEAD")
	}
	add := append([]string{"-C", pf.Checkout, "add", "--"}, paths...)
	if _, err := p.run(ctx, "git", add...); err != nil {
		return "", err
	}
	names, err := p.run(ctx, "git", "-C", pf.Checkout, "diff", "--cached", "--name-only")
	if err != nil {
		return "", err
	}
	for _, name := range strings.Split(names, "\n") {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if !allowedPublishPath(name) {
			return "", fmt.Errorf("refusing to commit %s", name)
		}
	}
	msg := "publish the " + pf.Palette + " public profile\n\nPalette-only update of the sanitized bundle. snapshot_id stays the same.\n"
	commit := append([]string{"-C", pf.Checkout, "commit", "-m", msg, "--"}, paths...)
	if _, err := p.run(ctx, "git", commit...); err != nil {
		return "", err
	}
	return p.run(ctx, "git", "-C", pf.Checkout, "rev-parse", "HEAD")
}

func allowedPublishPath(p string) bool {
	p = filepath.ToSlash(p)
	return p == "public-profile" || strings.HasPrefix(p, "public-profile/")
}

func (p *Publisher) push(ctx context.Context, pf Preflight) error {
	_, err := p.run(ctx, "git", "-C", pf.Checkout, "push", "origin", "HEAD:refs/heads/"+pf.ProductBranch)
	return err
}

func (p *Publisher) waitPages(ctx context.Context, pf Preflight, sha string) (string, error) {
	delay := 2 * time.Second
	var lastURL string
	for i := 0; i < 40; i++ {
		if err := ctx.Err(); err != nil {
			return lastURL, err
		}
		raw, err := p.run(ctx, "gh", "run", "list", "--repo", pf.ProductRepo, "--commit", sha, "--workflow", "pages.yml", "--limit", "5", "--json", "databaseId,status,conclusion,headSha,url,event")
		if err == nil && strings.TrimSpace(raw) != "" && raw != "[]" {
			var runs []struct {
				Status     string `json:"status"`
				Conclusion string `json:"conclusion"`
				HeadSHA    string `json:"headSha"`
				URL        string `json:"url"`
				Event      string `json:"event"`
			}
			if json.Unmarshal([]byte(raw), &runs) == nil {
				for _, run := range runs {
					if run.HeadSHA != sha {
						continue
					}
					lastURL = run.URL
					switch run.Status {
					case "completed":
						if run.Conclusion == "success" {
							return run.URL, nil
						}
						return run.URL, fmt.Errorf("pages %s", run.Conclusion)
					}
				}
			}
		}
		p.sleep(delay)
		if delay < 15*time.Second {
			delay += 3 * time.Second
		}
	}
	return lastURL, errors.New("pages 没有在时限内完成")
}

func (p *Publisher) updateReadme(ctx context.Context, pf Preflight) (string, error) {
	text, commitSHA, blob, err := p.readReadme(ctx, pf.Config())
	if err != nil {
		return "", err
	}
	if commitSHA != pf.ReadmeSHA || blob != pf.ReadmeBlob {
		return "", errors.New("个人 README 在确认之后变过。重新预检后再发布。")
	}
	updated, _, err := RewriteProfileReadme(text, pf.PagesURL, pf.SnapshotID, pf.AssetRevision)
	if err != nil {
		return "", err
	}
	payload, err := json.Marshal(map[string]string{
		"message": "docs: point the profile preview at the published whereToken palette\n",
		"content": base64.StdEncoding.EncodeToString([]byte(updated)),
		"sha":     blob,
		"branch":  pf.ProfileBranch,
	})
	if err != nil {
		return "", err
	}
	out, err := p.Run.Run(ctx, "gh", []string{"api", "--method", "PUT", "repos/" + pf.ProfileRepo + "/contents/" + pf.ReadmePath, "--input", "-"}, string(payload))
	if err != nil {
		return "", err
	}
	var body struct {
		Commit struct {
			SHA string `json:"sha"`
		} `json:"commit"`
	}
	if err := json.Unmarshal([]byte(out), &body); err != nil || body.Commit.SHA == "" {
		return "", errors.New("GitHub 没有返回 README 提交号")
	}
	return body.Commit.SHA, nil
}

func (pf Preflight) Config() PublishConfig {
	return PublishConfig{
		SchemaVersion: publishSchema,
		ProductRepo:   pf.ProductRepo,
		ProductBranch: pf.ProductBranch,
		ProfileRepo:   pf.ProfileRepo,
		ProfileBranch: pf.ProfileBranch,
		ProfileReadme: pf.ReadmePath,
		PagesURL:      pf.PagesURL,
		Checkout:      pf.Checkout,
	}
}

func (p *Publisher) verifyReadme(ctx context.Context, pf Preflight) error {
	text, _, _, err := p.readReadme(ctx, pf.Config())
	if err != nil {
		return err
	}
	updated, edits, err := RewriteProfileReadme(text, pf.PagesURL, pf.SnapshotID, pf.AssetRevision)
	if err != nil {
		return err
	}
	if updated != text {
		return errors.New("个人 README 上的三处预览地址还不是这次的缓存键")
	}
	for _, edit := range edits {
		if !strings.Contains(text, edit.After) {
			return errors.New("个人 README 缺少预览地址")
		}
	}
	if !strings.Contains(text, `media="(prefers-color-scheme: dark)"`) || !strings.Contains(text, `media="(prefers-color-scheme: light)"`) {
		return errors.New("个人 README 的 picture 条件丢了")
	}
	if !strings.Contains(text, strings.TrimRight(pf.PagesURL, "/")+"/") && !strings.Contains(text, strings.TrimRight(pf.PagesURL, "/")) {
		return errors.New("个人 README 不再链到公开页")
	}
	return nil
}

func jobFrom(pf Preflight, phase string) Job {
	return Job{
		Phase:         phase,
		PhaseLabel:    PhaseLabel(phase),
		Palette:       pf.Palette,
		ProductRepo:   pf.ProductRepo,
		ProfileRepo:   pf.ProfileRepo,
		CacheKey:      pf.CacheKey,
		SnapshotID:    pf.SnapshotID,
		AssetRevision: pf.AssetRevision,
		PreflightID:   pf.ID,
		LivePage:      strings.TrimRight(pf.PagesURL, "/") + "/",
	}
}

func failJob(palette, msg string) Job {
	return Job{Phase: PhaseFailed, PhaseLabel: PhaseLabel(PhaseFailed), Palette: palette, Error: Redact(msg)}
}

func reportJob(report func(Job), job Job) {
	if report != nil {
		report(job)
	}
}
