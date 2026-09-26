package hosted

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/rainhuang0220/whereToken/internal/publicprofile"
)

var githubLoginPattern = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9-]{0,37}[A-Za-z0-9])?$`)

func (s *server) publicProfile(w http.ResponseWriter, r *http.Request) {
	if !s.publicCORS(w, r) {
		return
	}
	rest := strings.TrimPrefix(r.URL.Path, "/api/v1/public-profile/")
	if rest == "session" {
		s.exchangeProfileSession(w, r)
		return
	}
	parts := strings.Split(rest, "/")
	if len(parts) == 0 || !githubLoginPattern.MatchString(parts[0]) {
		http.NotFound(w, r)
		return
	}
	login := parts[0]
	switch {
	case len(parts) == 1 && r.Method == http.MethodGet:
		s.getPublicProfile(w, r, login)
	case len(parts) == 2 && r.Method == http.MethodGet && (parts[1] == "preview-light.svg" || parts[1] == "preview-dark.svg"):
		s.getPublicPreview(w, r, login, parts[1])
	case len(parts) == 2 && parts[1] == "owner" && r.Method == http.MethodGet:
		s.getPublicOwner(w, r, login)
	case len(parts) == 2 && parts[1] == "publish" && r.Method == http.MethodPost:
		s.postPublicPublish(w, r, login)
	case len(parts) == 3 && parts[1] == "jobs" && r.Method == http.MethodGet:
		s.getPublicJob(w, r, login, parts[2])
	case len(parts) == 4 && parts[1] == "jobs" && parts[3] == "retry" && r.Method == http.MethodPost:
		s.postPublicRetry(w, r, login, parts[2])
	default:
		http.NotFound(w, r)
	}
}

func (s *server) publicCORS(w http.ResponseWriter, r *http.Request) bool {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-CSRF-Token")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Max-Age", "600")
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return false
	}
	return true
}

func (s *server) putPublicProjection(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut && r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.limit(r, "public-profile-sync", 30) {
		http.Error(w, "rate limited", http.StatusTooManyRequests)
		return
	}
	_, user, err := s.deviceFromBearer(r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 2<<20)
	raw, err := readLimited(r)
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	snap, canonical, err := publicprofile.AcceptProjection(raw)
	if err != nil {
		http.Error(w, "invalid profile", http.StatusBadRequest)
		return
	}
	if prev, err := s.opts.Store.Projection(r.Context(), user.ID); err == nil {
		var prevSnap publicprofile.Snapshot
		parsed := json.Unmarshal(prev.SnapshotJSON, &prevSnap) == nil
		if parsed && (!publicprofile.ShouldReplaceProjection(prevSnap, snap) || tokenRegressed(prevSnap, snap)) {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ok":          true,
				"snapshot_id": prevSnap.SnapshotID,
				"kept":        "previous",
				"readme":      "kept",
			})
			return
		}
	}
	if err := s.opts.Store.SaveProjection(r.Context(), user.ID, canonical, snap); err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	readme := s.considerPublication(r.Context(), user, snap, false, false)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"ok":          true,
		"snapshot_id": snap.SnapshotID,
		"readme":      readme,
	})
}

func tokenRegressed(prev, next publicprofile.Snapshot) bool {
	a := prev.Periods.All.Totals.Total.Value
	b := next.Periods.All.Totals.Total.Value
	if a != nil && b == nil {
		return true
	}
	return a != nil && b != nil && *b < *a
}

func (s *server) considerPublication(ctx context.Context, user User, snap publicprofile.Snapshot, theme, fromWorker bool) string {
	verified := s.verifiedWall(ctx, user.ID)
	pending := s.pendingIntent(ctx, user.ID)
	accepted := snap
	decision := publicprofile.DecidePublication(publicprofile.PublicationInput{
		Candidate:   snap,
		Accepted:    &accepted,
		Verified:    verified,
		Pending:     pending,
		Now:         s.now(),
		ThemeChange: theme,
		FromWorker:  fromWorker,
	})
	switch decision.Action {
	case publicprofile.ActionNoChange:
		if snap.SnapshotID != "" && snap.SnapshotID == verified.SnapshotID {
			_ = s.opts.Store.SetReadmeApplied(ctx, user.ID, snap.SnapshotID)
		}
		return "coalesced"
	case publicprofile.ActionBlocked:
		if decision.Reason != publicprofile.ReasonUnknownVerified {
			_ = s.opts.Store.SetPublicationIntent(ctx, user.ID, snap.SnapshotID, "blocked_data", decision.Reason, publicprofile.CodeBlockedData, sql.NullTime{}, 0)
		}
		return "blocked"
	case publicprofile.ActionWaitUntil:
		_ = s.opts.Store.SetPublicationIntent(ctx, user.ID, snap.SnapshotID, "pending", decision.Reason, "", sql.NullTime{Time: decision.DueAt, Valid: !decision.DueAt.IsZero()}, 0)
		return "pending"
	case publicprofile.ActionRetryPending:
		row, _ := s.opts.Store.Projection(ctx, user.ID)
		_ = s.opts.Store.SetPublicationIntent(ctx, user.ID, snap.SnapshotID, "failed", publicprofile.ReasonRetry, row.ReadmeLastError, row.PendingDueAt, row.PublishRetryCount)
		return "coalesced"
	case publicprofile.ActionPublishNow:
		return s.publishUsageNow(ctx, user, snap, theme)
	default:
		return "coalesced"
	}
}

func (s *server) publishUsageNow(ctx context.Context, user User, snap publicprofile.Snapshot, theme bool) string {
	kind := "usage"
	if theme {
		kind = "theme"
	}
	palette := s.usagePalette(ctx, user.ID)
	row, _ := s.opts.Store.Projection(ctx, user.ID)
	_ = s.opts.Store.SetPublicationIntent(ctx, user.ID, snap.SnapshotID, "pending", kind, "", sql.NullTime{Time: s.now(), Valid: true}, row.PublishRetryCount)
	if _, err := s.contentsClient(); err != nil {
		_ = s.opts.Store.SetPublicationIntent(ctx, user.ID, snap.SnapshotID, "failed", kind, "github_unconfigured", sql.NullTime{}, row.PublishRetryCount)
		return "deferred"
	}
	if _, err := s.publishPalette(ctx, user, palette, kind, nil); err != nil {
		s.notePublishFailure(ctx, user.ID, snap.SnapshotID, kind, err)
		return "deferred"
	}
	return "materialized"
}

func (s *server) notePublishFailure(ctx context.Context, userID int64, snapshotID, kind string, err error) {
	row, _ := s.opts.Store.Projection(ctx, userID)
	retry := row.PublishRetryCount + 1
	status := "failed"
	code := readmeErrorCode(err)
	var due sql.NullTime
	switch {
	case errors.Is(err, ErrGitAuth):
		status = "blocked_auth"
		code = "blocked_auth"
		due = sql.NullTime{Time: s.now().Add(6 * time.Hour), Valid: true}
	case errors.Is(err, errPublishBusy):
		status = "pending"
		code = "publish_busy"
		due = sql.NullTime{Time: s.now().Add(30 * time.Second), Valid: true}
	default:
		var rate *GitRateLimitError
		if errors.As(err, &rate) {
			code = "rate_limit"
			wait := rate.Wait
			if wait <= 0 {
				wait = 30 * time.Second
			}
			due = sql.NullTime{Time: s.now().Add(wait), Valid: true}
			break
		}
		if errors.Is(err, ErrGitTransient) || code == "github_write" {
			wait := publishBackoff(retry)
			if wait > 0 {
				due = sql.NullTime{Time: s.now().Add(wait), Valid: true}
			}
		}
	}
	_ = s.opts.Store.SetPublicationIntent(ctx, userID, snapshotID, status, kind, code, due, retry)
}

func publishBackoff(retry int) time.Duration {
	if retry <= 1 {
		return 0
	}
	wait := 30 * time.Second
	for i := 1; i < retry && wait < 15*time.Minute; i++ {
		wait *= 2
	}
	if wait > 15*time.Minute {
		return 15 * time.Minute
	}
	return wait
}

func (s *server) usagePalette(ctx context.Context, userID int64) string {
	palette := publicprofile.DefaultPalette
	pres, err := s.opts.Store.Presentation(ctx, userID)
	if err == nil && publicprofile.KnownPalette(pres.Palette) && len(pres.PreviewLight) > 0 {
		palette = pres.Palette
	}
	return palette
}

func (s *server) verifiedWall(ctx context.Context, userID int64) publicprofile.VerifiedWall {
	pres, err := s.opts.Store.Presentation(ctx, userID)
	if err != nil || pres.ReadmeSnapshotID == "" {
		return publicprofile.VerifiedWall{}
	}
	wall := publicprofile.VerifiedWall{
		Proven:        true,
		SnapshotID:    pres.ReadmeSnapshotID,
		CacheKey:      pres.ReadmeCacheKey,
		AssetRevision: pres.AssetRevision,
	}
	if pres.ReadmeMaterializedAt.Valid {
		wall.PublishedAt = pres.ReadmeMaterializedAt.Time
	}
	if pres.VerifiedSnapshotID.Valid && pres.VerifiedSnapshotID.String == pres.ReadmeSnapshotID {
		if pres.VerifiedTotal.Valid {
			n := pres.VerifiedTotal.Int64
			wall.Total = &n
		}
		if pres.VerifiedAsOf.Valid {
			wall.AsOfDate = pres.VerifiedAsOf.String
		}
	}
	return wall
}

func (s *server) pendingIntent(ctx context.Context, userID int64) publicprofile.PendingIntent {
	row, err := s.opts.Store.Projection(ctx, userID)
	if err != nil {
		return publicprofile.PendingIntent{}
	}
	intent := publicprofile.PendingIntent{SnapshotID: row.DesiredSnapshotID, Reason: row.PendingReason, Failed: row.ReadmeStatus == "failed" || row.ReadmeStatus == "blocked_auth"}
	if row.PendingDueAt.Valid {
		intent.DueAt = row.PendingDueAt.Time
	}
	return intent
}

// retryFailedReadmes publishes a wall that was accepted but not verified,
// including a usage change that was waiting for the cooldown. It does not
// require a new client PUT.
func (s *server) retryFailedReadmes(ctx context.Context) {
	if s.opts.Store == nil {
		return
	}
	ids, err := s.opts.Store.DuePublicationUserIDs(ctx, s.now())
	if err != nil {
		return
	}
	for _, id := range ids {
		user, err := s.opts.Store.UserByID(ctx, id)
		if err != nil {
			continue
		}
		row, err := s.opts.Store.Projection(ctx, user.ID)
		if err != nil {
			continue
		}
		var snap publicprofile.Snapshot
		if json.Unmarshal(row.SnapshotJSON, &snap) != nil || snap.SnapshotID == "" {
			continue
		}
		s.considerPublication(ctx, user, snap, false, true)
	}
}

func readmeErrorCode(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, ErrGitConflict) {
		return "conflict"
	}
	if errors.Is(err, ErrGitAuth) {
		return "blocked_auth"
	}
	var rate *GitRateLimitError
	if errors.As(err, &rate) {
		return "rate_limit"
	}
	if errors.Is(err, ErrGitTransient) {
		return "github_write"
	}
	switch err.Error() {
	case "remote content changed":
		return "conflict"
	case "readme verify", "readme cache key missing", "light preview mismatch", "dark preview mismatch":
		return "github_verify"
	default:
		return "github_write"
	}
}

func (s *server) getPublicProfile(w http.ResponseWriter, r *http.Request, login string) {
	user, err := s.opts.Store.UserByLogin(r.Context(), login)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	row, err := s.opts.Store.Projection(r.Context(), user.ID)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if publicprofile.Sensitive(string(row.SnapshotJSON)) {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
		return
	}
	pres, presErr := s.opts.Store.Presentation(r.Context(), user.ID)
	palette := publicprofile.DefaultPalette
	revision := "0"
	asset := ""
	if presErr == nil && publicprofile.KnownPalette(pres.Palette) {
		palette = pres.Palette
		revision = pres.Revision
		asset = pres.AssetRevision
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	payload := map[string]any{
		"schema":         publicprofile.LiveSchema,
		"schema_version": publicprofile.LiveSchemaVersion,
		"freshness": map[string]any{
			"mode":       publicprofile.FreshnessMode,
			"source":     publicprofile.FreshnessHosted,
			"updated_at": row.UpdatedAt.UTC().Format(time.RFC3339),
		},
		"owner":                 map[string]string{"github_login": user.Login},
		"data_revision":         row.SnapshotID,
		"presentation_revision": revision,
		"asset_revision":        asset,
		"presentation": publicprofile.Presentation{
			SchemaVersion: publicprofile.PresentationSchema,
			PublicPalette: palette,
			Revision:      revision,
		},
		"snapshot": json.RawMessage(row.SnapshotJSON),
		"wall":     s.wallJSON(r.Context(), user.ID, row),
	}
	_ = json.NewEncoder(w).Encode(payload)
}

func (s *server) wallJSON(ctx context.Context, userID int64, row projectionRow) map[string]any {
	wall := s.verifiedWall(ctx, userID)
	out := map[string]any{"phase": wallPhase(row, wall)}
	if wall.SnapshotID != "" {
		out["verified_snapshot_id"] = wall.SnapshotID
	}
	if wall.AsOfDate != "" {
		out["verified_as_of_date"] = wall.AsOfDate
	}
	if !wall.PublishedAt.IsZero() {
		out["verified_at"] = wall.PublishedAt.UTC().Format(time.RFC3339)
	}
	if wall.Total != nil {
		out["verified_total"] = *wall.Total
	}
	if row.DesiredSnapshotID != "" && row.DesiredSnapshotID != wall.SnapshotID && (row.ReadmeStatus == "pending" || row.ReadmeStatus == "failed" || row.ReadmeStatus == "blocked_auth") {
		out["pending_snapshot_id"] = row.DesiredSnapshotID
		out["pending_failed"] = row.ReadmeStatus == "failed" || row.ReadmeStatus == "blocked_auth"
		if row.PendingDueAt.Valid {
			out["pending_due_at"] = row.PendingDueAt.Time.UTC().Format(time.RFC3339)
		}
	}
	return out
}

func wallPhase(row projectionRow, wall publicprofile.VerifiedWall) string {
	switch row.ReadmeStatus {
	case "pending":
		return "pending"
	case "failed":
		return "failed"
	case "blocked_auth":
		return "blocked_auth"
	case "blocked_data":
		return "blocked"
	}
	if wall.Proven && wall.SnapshotID != "" && (row.DesiredSnapshotID == "" || row.DesiredSnapshotID == wall.SnapshotID || row.ReadmeStatus == "applied") {
		return "verified"
	}
	if wall.Proven {
		return "pending"
	}
	return "none"
}

func (s *server) getPublicPreview(w http.ResponseWriter, r *http.Request, login, name string) {
	user, err := s.opts.Store.UserByLogin(r.Context(), login)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	pres, err := s.opts.Store.Presentation(r.Context(), user.ID)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	body := pres.PreviewLight
	if name == "preview-dark.svg" {
		body = pres.PreviewDark
	}
	if bytes.Contains(bytes.ToLower(body), []byte("<script")) {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "image/svg+xml; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write(body)
}

func (s *server) getPublicOwner(w http.ResponseWriter, r *http.Request, login string) {
	user, _, ok := s.ownerSession(w, r, login, false)
	if !ok {
		return
	}
	palette := publicprofile.DefaultPalette
	if pres, err := s.opts.Store.Presentation(r.Context(), user.ID); err == nil && publicprofile.KnownPalette(pres.Palette) {
		palette = pres.Palette
	}
	row, rowErr := s.opts.Store.Projection(r.Context(), user.ID)
	phase := "idle"
	var wall map[string]any
	if rowErr == nil {
		wall = s.wallJSON(r.Context(), user.ID, row)
		if p, ok := wall["phase"].(string); ok && p != "" {
			phase = p
		}
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"owner":             true,
		"login":             user.Login,
		"published_palette": palette,
		"profile_url":       "https://github.com/" + user.Login,
		"phase":             phase,
		"wall":              wall,
	})
}

func (s *server) postPublicPublish(w http.ResponseWriter, r *http.Request, login string) {
	if !s.limit(r, "public-profile-publish", 10) {
		http.Error(w, "rate limited", http.StatusTooManyRequests)
		return
	}
	user, _, ok := s.ownerSession(w, r, login, true)
	if !ok {
		return
	}
	palette, ok := s.publishBody(w, r)
	if !ok {
		return
	}
	if row, err := s.opts.Store.Projection(r.Context(), user.ID); err == nil {
		var snap publicprofile.Snapshot
		if json.Unmarshal(row.SnapshotJSON, &snap) == nil {
			decision := publicprofile.DecidePublication(publicprofile.PublicationInput{
				Candidate:   snap,
				Accepted:    &snap,
				Verified:    s.verifiedWall(r.Context(), user.ID),
				Pending:     s.pendingIntent(r.Context(), user.ID),
				Now:         s.now(),
				ThemeChange: true,
			})
			if decision.Action != publicprofile.ActionPublishNow {
				http.Error(w, "snapshot is not publishable", http.StatusConflict)
				return
			}
		}
	}
	job, err := s.publishPalette(r.Context(), user, palette, "theme", nil)
	if err != nil {
		if row, rerr := s.opts.Store.Projection(r.Context(), user.ID); rerr == nil {
			s.notePublishFailure(r.Context(), user.ID, row.SnapshotID, "theme", err)
		}
	}
	s.writeJob(w, job, err)
}

func (s *server) postPublicRetry(w http.ResponseWriter, r *http.Request, login, id string) {
	user, _, ok := s.ownerSession(w, r, login, true)
	if !ok {
		return
	}
	if !s.confirmOnly(w, r) {
		return
	}
	job, err := s.opts.Store.Job(r.Context(), user.ID, id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if job.Phase != phasePartialFailure && job.Phase != phaseConflict {
		http.Error(w, "not retryable", http.StatusConflict)
		return
	}
	next, err := s.publishPalette(r.Context(), user, job.Palette, job.Kind, &job)
	s.writeJob(w, next, err)
}

func (s *server) getPublicJob(w http.ResponseWriter, r *http.Request, login, id string) {
	user, _, ok := s.ownerSession(w, r, login, false)
	if !ok {
		return
	}
	job, err := s.opts.Store.Job(r.Context(), user.ID, id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(job.browserJSON())
}

type publishRequest struct {
	Palette string `json:"palette"`
	Confirm bool   `json:"confirm"`
	Path    string `json:"path"`
	Repo    string `json:"repo"`
	Branch  string `json:"branch"`
}

func (s *server) readPublishRequest(w http.ResponseWriter, r *http.Request) (publishRequest, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	var body publishRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return publishRequest{}, false
	}
	if strings.TrimSpace(body.Path) != "" || strings.TrimSpace(body.Repo) != "" || strings.TrimSpace(body.Branch) != "" {
		http.Error(w, "path is not accepted", http.StatusBadRequest)
		return publishRequest{}, false
	}
	if !body.Confirm {
		http.Error(w, "confirmation required", http.StatusBadRequest)
		return publishRequest{}, false
	}
	return body, true
}

func (s *server) publishBody(w http.ResponseWriter, r *http.Request) (string, bool) {
	body, ok := s.readPublishRequest(w, r)
	if !ok {
		return "", false
	}
	if err := publicprofile.ValidatePalette(body.Palette); err != nil {
		http.Error(w, "unknown palette", http.StatusBadRequest)
		return "", false
	}
	return body.Palette, true
}

func (s *server) confirmOnly(w http.ResponseWriter, r *http.Request) bool {
	_, ok := s.readPublishRequest(w, r)
	return ok
}

func (s *server) ownerSession(w http.ResponseWriter, r *http.Request, login string, mutate bool) (User, Session, bool) {
	user, sess, err := s.profileUser(r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return User{}, Session{}, false
	}
	if mutate && !s.requireCSRF(w, r, sess) {
		return User{}, Session{}, false
	}
	owner, err := s.opts.Store.UserByLogin(r.Context(), login)
	if err != nil || owner.ID != user.ID || !strings.EqualFold(owner.Login, user.Login) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return User{}, Session{}, false
	}
	return user, sess, true
}

func (s *server) profileUser(r *http.Request) (User, Session, error) {
	h := strings.TrimSpace(r.Header.Get("Authorization"))
	if !strings.HasPrefix(h, "Bearer ") || s.opts.Store == nil {
		return User{}, Session{}, ErrUnauthorized
	}
	sess, err := s.opts.Store.LookupProfileSession(r.Context(), strings.TrimPrefix(h, "Bearer "))
	if err != nil {
		return User{}, Session{}, ErrUnauthorized
	}
	user, err := s.opts.Store.UserByID(r.Context(), sess.UserID)
	if err != nil {
		return User{}, Session{}, ErrUnauthorized
	}
	return user, sess, nil
}

func (s *server) exchangeProfileSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.limit(r, "public-profile-session", 20) {
		http.Error(w, "rate limited", http.StatusTooManyRequests)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	var body struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.Code) == "" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	userID, err := s.opts.Store.ConsumeExchangeCode(r.Context(), body.Code)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	user, err := s.opts.Store.UserByID(r.Context(), userID)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	token, csrf, exp, err := s.opts.Store.CreateProfileSession(r.Context(), user.ID)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"token":      token,
		"csrf":       csrf,
		"login":      user.Login,
		"expires_at": exp.UTC().Format(time.RFC3339),
	})
}

func (s *server) writeJob(w http.ResponseWriter, job publishJob, err error) {
	if job.ID == "" && err != nil {
		http.Error(w, "publish failed", http.StatusBadGateway)
		return
	}
	status := http.StatusOK
	if job.Phase == phaseFailed || job.Phase == phaseConflict || job.Phase == phasePartialFailure {
		status = http.StatusConflict
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(job.browserJSON())
}

func (s *server) returnAllowed(raw string) bool {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Scheme != "https" || u.User != nil || u.Host == "" || u.RawQuery != "" || u.Fragment != "" {
		return false
	}
	if strings.TrimRight(u.Path, "/") != "/whereToken/profile" {
		return false
	}
	origin := u.Scheme + "://" + u.Host
	origins := s.opts.Config.Publish.Origins
	if len(origins) == 0 {
		origins = []string{"https://rainhuang0220.github.io"}
	}
	for _, allowed := range origins {
		if origin == strings.TrimRight(strings.TrimSpace(allowed), "/") {
			return true
		}
	}
	return false
}

func readLimited(r *http.Request) ([]byte, error) {
	var buf bytes.Buffer
	_, err := buf.ReadFrom(r.Body)
	return buf.Bytes(), err
}
