package hosted

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/rainhuang0220/whereToken/internal/publicprofile"
)

const (
	phasePublishingProfile = "publishing_profile"
	phasePublishingReadme  = "publishing_readme"
	phaseVerifying         = "verifying"
	phasePublished         = "published"
	phaseAlreadyPublished  = "already_published"
	phasePartialFailure    = "partial_failure"
	phaseFailed            = "failed"
	phaseConflict          = "conflict"

	resultAlreadyPublished = "ALREADY_PUBLISHED"
)

func phaseLabel(phase string) string {
	switch phase {
	case phasePublishingProfile:
		return "正在更新公开主题"
	case phasePublishingReadme:
		return "正在更新 GitHub 主页"
	case phaseVerifying:
		return "正在核验"
	case phasePublished:
		return "已应用"
	case phaseAlreadyPublished:
		return "已是当前主题"
	case phasePartialFailure:
		return "主题已保存，主页 README 未更新"
	case phaseConflict:
		return "远端已变化，没有覆盖"
	case phaseFailed:
		return "发布失败"
	default:
		return "待发布"
	}
}

func (s *server) contentsClient() (ContentsClient, error) {
	if s.opts.Contents != nil {
		return s.opts.Contents, nil
	}
	return newAppContents(s.opts.Config.Publish, s.opts.Now)
}

type publishTarget struct {
	Repo       string
	Branch     string
	ReadmePath string
	LightPath  string
	DarkPath   string
	AssetBase  string
}

func (s *server) publishTarget() (publishTarget, error) {
	cfg := s.opts.Config.Publish
	if cfg.ReadmePath == "" {
		cfg.ReadmePath = "README.md"
	}
	if cfg.Branch == "" {
		cfg.Branch = "main"
	}
	if cfg.PreviewDir == "" {
		cfg.PreviewDir = "wheretoken"
	}
	if cfg.ReadmePath != "README.md" {
		return publishTarget{}, errors.New("readme path is not allowlisted")
	}
	base, err := publicprofile.RawPreviewBase(cfg.Repo, cfg.Branch, cfg.PreviewDir)
	if err != nil {
		return publishTarget{}, err
	}
	return publishTarget{
		Repo:       cfg.Repo,
		Branch:     cfg.Branch,
		ReadmePath: cfg.ReadmePath,
		LightPath:  cfg.PreviewDir + "/preview-light.svg",
		DarkPath:   cfg.PreviewDir + "/preview-dark.svg",
		AssetBase:  base,
	}, nil
}

func renderPreviews(snap publicprofile.Snapshot, palette string) (light, dark []byte, err error) {
	lightTheme, darkTheme := publicprofile.ThemesFor(palette)
	var lb, db bytes.Buffer
	if err = publicprofile.RenderPreview(&lb, snap, lightTheme); err != nil {
		return nil, nil, err
	}
	if err = publicprofile.RenderPreview(&db, snap, darkTheme); err != nil {
		return nil, nil, err
	}
	return lb.Bytes(), db.Bytes(), nil
}

func (s *server) publishPalette(ctx context.Context, user User, palette, kind string, retry *publishJob) (publishJob, error) {
	s.publishMu.Lock()
	defer s.publishMu.Unlock()
	if err := publicprofile.ValidatePalette(palette); err != nil {
		return publishJob{}, err
	}
	target, err := s.publishTarget()
	if err != nil {
		return s.failPublish(ctx, user, palette, kind, retry, "publisher is not configured"), err
	}
	repoOwner := strings.Split(target.Repo, "/")[0]
	if !strings.EqualFold(repoOwner, user.Login) {
		return s.failPublish(ctx, user, palette, kind, retry, "owner mismatch"), errors.New("owner mismatch")
	}
	client, err := s.contentsClient()
	if err != nil {
		return s.failPublish(ctx, user, palette, kind, retry, "github app unconfigured"), err
	}
	row, err := s.opts.Store.Projection(ctx, user.ID)
	if err != nil {
		return s.failPublish(ctx, user, palette, kind, retry, "no public snapshot"), err
	}
	var snap publicprofile.Snapshot
	if err := json.Unmarshal(row.SnapshotJSON, &snap); err != nil {
		return s.failPublish(ctx, user, palette, kind, retry, "stored snapshot"), err
	}
	if snap.SnapshotID != row.SnapshotID {
		return s.failPublish(ctx, user, palette, kind, retry, "snapshot identity"), errors.New("snapshot identity")
	}
	beforeID := snap.SnapshotID
	light, dark, err := renderPreviews(snap, palette)
	if err != nil {
		return s.failPublish(ctx, user, palette, kind, retry, "preview"), err
	}
	if bytes.Contains(bytes.ToLower(light), []byte("<script")) || bytes.Contains(bytes.ToLower(dark), []byte("<script")) {
		return s.failPublish(ctx, user, palette, kind, retry, "preview rejected"), errors.New("preview rejected")
	}
	assetRev := publicprofile.HostedAssetRevision(light, dark, palette)
	cacheKey, err := publicprofile.CacheKey(beforeID, assetRev)
	if err != nil {
		return s.failPublish(ctx, user, palette, kind, retry, "cache key"), err
	}
	pres, presErr := s.opts.Store.Presentation(ctx, user.ID)
	readme, readmeErr := client.GetFile(ctx, target.Repo, target.ReadmePath, target.Branch)
	if readmeErr != nil {
		return s.failPublish(ctx, user, palette, kind, retry, "readme read"), readmeErr
	}
	facts := publicprofile.ReadmeFacts{
		TotalDisplay: snap.Periods.All.Totals.Total.Display,
		DataStatus:   snap.DataStatus,
		AsOfDate:     snap.AsOfDate,
	}
	alt, altErr := publicprofile.ReadmeAlt(facts)
	if altErr != nil {
		return s.failPublish(ctx, user, palette, kind, retry, "readme alt"), altErr
	}
	if presErr == nil && pres.Palette == palette && pres.AssetRevision == assetRev && readmeHasCacheKey(readme.Content, cacheKey) && strings.Contains(string(readme.Content), `alt="`+alt+`"`) {
		job := s.beginJob(ctx, user, palette, kind, beforeID, retry)
		job.Phase = phaseAlreadyPublished
		job.ResultCode = resultAlreadyPublished
		job.AssetRevision = assetRev
		job.CacheKey = cacheKey
		job.ErrorText = ""
		_ = s.opts.Store.UpdateJob(ctx, job)
		return job, nil
	}
	job := s.beginJob(ctx, user, palette, kind, beforeID, retry)
	job.Phase = phasePublishingProfile
	job.AssetRevision = assetRev
	job.CacheKey = cacheKey
	if err := s.opts.Store.UpdateJob(ctx, job); err != nil {
		return job, err
	}
	revision := strings.TrimPrefix(assetRev, "sha256:")
	if len(revision) > 80 {
		revision = revision[:80]
	}
	if err := s.opts.Store.SavePresentation(ctx, user.ID, palette, revision, assetRev, light, dark); err != nil {
		return s.finish(ctx, job, phaseFailed, "could not store presentation")
	}
	if snap.SnapshotID != beforeID {
		return s.finish(ctx, job, phaseFailed, "snapshot identity changed")
	}
	job.Phase = phasePublishingReadme
	_ = s.opts.Store.UpdateJob(ctx, job)
	if err := s.putPreview(ctx, client, target, target.LightPath, light); err != nil {
		return s.finishGit(ctx, job, err)
	}
	if err := s.putPreview(ctx, client, target, target.DarkPath, dark); err != nil {
		return s.finishGit(ctx, job, err)
	}
	rewritten, _, err := publicprofile.RewriteReadmeProjection(string(readme.Content), target.AssetBase, beforeID, assetRev, facts)
	if err != nil {
		return s.finish(ctx, job, phasePartialFailure, err.Error())
	}
	if rewritten != string(readme.Content) {
		if _, err := client.PutFile(ctx, target.Repo, target.ReadmePath, target.Branch, "whereToken: update public profile preview", []byte(rewritten), readme.SHA); err != nil {
			return s.finishGit(ctx, job, err)
		}
	}
	job.Phase = phaseVerifying
	_ = s.opts.Store.UpdateJob(ctx, job)
	checked, err := client.GetFile(ctx, target.Repo, target.ReadmePath, target.Branch)
	if err != nil {
		return s.finish(ctx, job, phasePartialFailure, "readme verify")
	}
	if !readmeHasCacheKey(checked.Content, cacheKey) {
		return s.finish(ctx, job, phasePartialFailure, "readme cache key missing")
	}
	lightFile, err := client.GetFile(ctx, target.Repo, target.LightPath, target.Branch)
	if err != nil || !bytes.Equal(lightFile.Content, light) {
		return s.finish(ctx, job, phasePartialFailure, "light preview mismatch")
	}
	darkFile, err := client.GetFile(ctx, target.Repo, target.DarkPath, target.Branch)
	if err != nil || !bytes.Equal(darkFile.Content, dark) {
		return s.finish(ctx, job, phasePartialFailure, "dark preview mismatch")
	}
	if err := s.opts.Store.MarkReadmeMaterialized(ctx, user.ID, cacheKey, beforeID); err != nil {
		return s.finish(ctx, job, phasePartialFailure, "materialize stamp")
	}
	job.Phase = phasePublished
	job.ResultCode = "published"
	job.ErrorText = ""
	if err := s.opts.Store.UpdateJob(ctx, job); err != nil {
		return job, err
	}
	return job, nil
}

func (s *server) putPreview(ctx context.Context, client ContentsClient, target publishTarget, path string, content []byte) error {
	current, err := client.GetFile(ctx, target.Repo, path, target.Branch)
	if err != nil && !errors.Is(err, ErrGitMissing) {
		return err
	}
	if err == nil && bytes.Equal(current.Content, content) {
		return nil
	}
	sha := ""
	if err == nil {
		sha = current.SHA
	}
	_, putErr := client.PutFile(ctx, target.Repo, path, target.Branch, "whereToken: update public profile preview", content, sha)
	return putErr
}

func (s *server) finishGit(ctx context.Context, job publishJob, err error) (publishJob, error) {
	if errors.Is(err, ErrGitConflict) {
		return s.finish(ctx, job, phaseConflict, "remote content changed")
	}
	return s.finish(ctx, job, phasePartialFailure, err.Error())
}

func (s *server) finish(ctx context.Context, job publishJob, phase, message string) (publishJob, error) {
	job.Phase = phase
	job.ResultCode = phase
	job.ErrorText = publicprofile.Redact(message)
	_ = s.opts.Store.UpdateJob(ctx, job)
	return job, errors.New(job.ErrorText)
}

func (s *server) beginJob(ctx context.Context, user User, palette, kind, snapshotID string, retry *publishJob) publishJob {
	if retry != nil {
		retry.Palette = palette
		retry.Kind = kind
		retry.SnapshotID = snapshotID
		retry.ErrorText = ""
		retry.ResultCode = ""
		return *retry
	}
	id, err := newUUID4()
	if err != nil {
		id = fmt.Sprintf("job-%d", s.opts.Now().UnixNano())
	}
	job := publishJob{
		ID:         id,
		UserID:     user.ID,
		Kind:       kind,
		Palette:    palette,
		Phase:      phasePublishingProfile,
		SnapshotID: snapshotID,
	}
	_ = s.opts.Store.InsertJob(ctx, job)
	return job
}

func (s *server) failPublish(ctx context.Context, user User, palette, kind string, retry *publishJob, message string) publishJob {
	job := s.beginJob(ctx, user, palette, kind, "", retry)
	job.Phase = phaseFailed
	job.ResultCode = phaseFailed
	job.ErrorText = publicprofile.Redact(message)
	_ = s.opts.Store.UpdateJob(ctx, job)
	return job
}

func readmeHasCacheKey(content []byte, key string) bool {
	return strings.Count(string(content), "?v="+key) >= 3
}

func (j publishJob) browserJSON() map[string]any {
	return map[string]any{
		"id":           j.ID,
		"phase":        j.Phase,
		"phase_label":  phaseLabel(j.Phase),
		"palette":      j.Palette,
		"result":       j.ResultCode,
		"snapshot_id":  j.SnapshotID,
		"cache_key":    j.CacheKey,
		"retry_readme": j.Phase == phasePartialFailure || j.Phase == phaseConflict,
		"error":        publicprofile.Redact(j.ErrorText),
	}
}
