package hosted

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rainhuang0220/whereToken/internal/event"
	"github.com/rainhuang0220/whereToken/internal/publicprofile"
)

const publishReadme = `<p align="center">
  <a href="https://rainhuang0220.github.io/whereToken/profile/">
    <picture>
      <source media="(prefers-color-scheme: dark)" srcset="https://rainhuang0220.github.io/whereToken/profile/preview-dark.svg?v=15c96d356fc90d8073524036f29bef83ef236f76f7b02a11e862b9cbb4c52e62-2f221b6893380bf9ebaecd0cce4fe8bf2abea777805c8dac1c88d9d34c4245cc">
      <source media="(prefers-color-scheme: light)" srcset="https://rainhuang0220.github.io/whereToken/profile/preview-light.svg?v=15c96d356fc90d8073524036f29bef83ef236f76f7b02a11e862b9cbb4c52e62-2f221b6893380bf9ebaecd0cce4fe8bf2abea777805c8dac1c88d9d34c4245cc">
      <img src="https://rainhuang0220.github.io/whereToken/profile/preview-light.svg?v=15c96d356fc90d8073524036f29bef83ef236f76f7b02a11e862b9cbb4c52e62-2f221b6893380bf9ebaecd0cce4fe8bf2abea777805c8dac1c88d9d34c4245cc" width="800" alt="Coding activity snapshot">
    </picture>
  </a>
</p>
keep-this-line
<img src="https://github.com/rainhuang0220.png" alt="portrait">
`

type memGit struct {
	mu         sync.Mutex
	files      map[string]memGitFile
	puts       int
	failReadme int
	conflict   int
	failPath   string
	paths      []string
}

type memGitFile struct {
	SHA  string
	Body []byte
}

func newMemGit(readme string) *memGit {
	g := &memGit{files: map[string]memGitFile{}}
	body := []byte(readme)
	g.files["rainhuang0220/rainhuang0220\nREADME.md"] = memGitFile{SHA: memDigest(body), Body: body}
	return g
}

func memDigest(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func (g *memGit) GetFile(_ context.Context, repo, path, _ string) (ContentFile, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	file, ok := g.files[repo+"\n"+path]
	if !ok {
		return ContentFile{}, ErrGitMissing
	}
	return ContentFile{SHA: file.SHA, Content: append([]byte(nil), file.Body...)}, nil
}

func (g *memGit) PutFile(_ context.Context, repo, path, branch, message string, content []byte, expectedSHA string) (string, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if branch != "main" {
		return "", ErrGitConflict
	}
	if strings.Contains(message, "force") {
		return "", ErrGitConflict
	}
	g.puts++
	g.paths = append(g.paths, path)
	key := repo + "\n" + path
	cur, ok := g.files[key]
	if expectedSHA == "" {
		if ok {
			return "", ErrGitConflict
		}
	} else if !ok || cur.SHA != expectedSHA {
		return "", ErrGitConflict
	}
	if path == "README.md" && g.conflict > 0 {
		g.conflict--
		return "", ErrGitConflict
	}
	if path == "README.md" && g.failReadme > 0 {
		g.failReadme--
		return "", io.EOF
	}
	if g.failPath != "" && path == g.failPath {
		g.failPath = ""
		return "", io.EOF
	}
	if repo != "rainhuang0220/rainhuang0220" || (path != "README.md" && path != "wheretoken/preview-light.svg" && path != "wheretoken/preview-dark.svg") {
		return "", ErrGitConflict
	}
	next := memGitFile{SHA: memDigest(content), Body: append([]byte(nil), content...)}
	g.files[key] = next
	return next.SHA, nil
}

func (g *memGit) readme() string {
	g.mu.Lock()
	defer g.mu.Unlock()
	return string(g.files["rainhuang0220/rainhuang0220\nREADME.md"].Body)
}

func publishMux(t *testing.T, git ContentsClient) http.Handler {
	t.Helper()
	st := readyStore(t)
	return NewMux(MuxOptions{
		Version: "0.7.4",
		Store:   st,
		Config: Config{
			Listen:             "127.0.0.1:0",
			GitHubClientID:     "cid",
			GitHubClientSecret: "csecret",
			PublicURL:          "https://wheretoken.plainlist.space",
			CookieSecure:       false,
			Publish: ProfilePublish{
				AppID:          "1",
				InstallationID: "1",
				PrivateKeyPEM:  []byte("configured"),
				Repo:           "rainhuang0220/rainhuang0220",
				Branch:         "main",
				ReadmePath:     "README.md",
				PreviewDir:     "wheretoken",
				Origins:        []string{"https://rainhuang0220.github.io"},
			},
		},
		Contents: git,
		Log:      log.New(io.Discard, "", 0),
	})
}

func seedOwner(t *testing.T) (string, string, string) {
	t.Helper()
	st := readyStore(t)
	ctx := context.Background()
	user, err := st.UpsertGitHubUser(ctx, GitHubIdentity{ID: 4242, Login: "rainhuang0220", AvatarURL: "https://avatars.githubusercontent.com/u/4242"})
	if err != nil {
		t.Fatal(err)
	}
	_, deviceToken, err := st.InsertDevice(ctx, user.ID, DeviceMeta{OS: "darwin", Arch: "arm64", ClientVersion: "test", Label: "test"})
	if err != nil {
		t.Fatal(err)
	}
	session, csrf, _, err := st.CreateProfileSession(ctx, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	return deviceToken, session, csrf
}

func testProjection(t *testing.T) []byte {
	t.Helper()
	snap, err := publicprofile.Build(publicprofile.Input{Now: time.Date(2026, 9, 15, 9, 33, 58, 0, time.UTC), Loc: time.UTC, Version: "test"})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := publicprofile.Marshal(snap)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestPublicProfileSyncPublishAndRetry(t *testing.T) {
	git := newMemGit(publishReadme)
	h := publishMux(t, git)
	device, session, csrf := seedOwner(t)
	raw := testProjection(t)
	var snap publicprofile.Snapshot
	if err := json.Unmarshal(raw, &snap); err != nil {
		t.Fatal(err)
	}

	bad := httptest.NewRequest(http.MethodPut, "/api/v1/sync/public-profile", bytes.NewReader([]byte(`{"access_token":"secret","schema":"wheretoken.public-profile"}`)))
	bad.Header.Set("Authorization", "Bearer "+device)
	badRec := httptest.NewRecorder()
	h.ServeHTTP(badRec, bad)
	if badRec.Code != http.StatusBadRequest {
		t.Fatalf("private upload %d %s", badRec.Code, badRec.Body.String())
	}

	anon := httptest.NewRequest(http.MethodPut, "/api/v1/sync/public-profile", bytes.NewReader(raw))
	anonRec := httptest.NewRecorder()
	h.ServeHTTP(anonRec, anon)
	if anonRec.Code != http.StatusUnauthorized {
		t.Fatalf("anon sync %d", anonRec.Code)
	}

	put := httptest.NewRequest(http.MethodPut, "/api/v1/sync/public-profile", bytes.NewReader(raw))
	put.Header.Set("Authorization", "Bearer "+device)
	putRec := httptest.NewRecorder()
	h.ServeHTTP(putRec, put)
	if putRec.Code != http.StatusOK {
		t.Fatalf("sync %d %s", putRec.Code, putRec.Body.String())
	}
	var synced map[string]any
	_ = json.Unmarshal(putRec.Body.Bytes(), &synced)
	if synced["snapshot_id"] != snap.SnapshotID {
		t.Fatalf("synced id %+v", synced)
	}

	got := httptest.NewRequest(http.MethodGet, "/api/v1/public-profile/rainhuang0220", nil)
	gotRec := httptest.NewRecorder()
	h.ServeHTTP(gotRec, got)
	if gotRec.Code != http.StatusOK {
		t.Fatalf("get %d %s", gotRec.Code, gotRec.Body.String())
	}
	body := gotRec.Body.String()
	for _, forbidden := range []string{"access_token", "device_token", "/Users/", "source_hmac", "wtd_1", "README.md", "rainhuang0220/rainhuang0220"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("public body contains %q", forbidden)
		}
	}
	var envelope map[string]any
	if err := json.Unmarshal(gotRec.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope["schema"] != publicprofile.LiveSchema || envelope["data_revision"] != snap.SnapshotID {
		t.Fatalf("envelope %+v", envelope)
	}
	fresh := envelope["freshness"].(map[string]any)
	if fresh["mode"] != publicprofile.FreshnessMode || fresh["source"] != "hosted" {
		t.Fatalf("freshness %+v", fresh)
	}

	putsBeforeReject := git.puts
	noCSRF := publishRequestTo(t, h, session, "", "cobalt", true, "")
	if noCSRF.Code != http.StatusForbidden {
		t.Fatalf("csrf %d %s", noCSRF.Code, noCSRF.Body.String())
	}
	withPath := publishRequestTo(t, h, session, csrf, "cobalt", true, `,"path":"README.md","repo":"evil/evil","branch":"dev"}`)
	if withPath.Code != http.StatusBadRequest {
		t.Fatalf("path %d %s", withPath.Code, withPath.Body.String())
	}
	if git.puts != putsBeforeReject {
		t.Fatalf("rejected request wrote files %d -> %d", putsBeforeReject, git.puts)
	}

	st := readyStore(t)
	otherUser, err := st.UpsertGitHubUser(context.Background(), GitHubIdentity{ID: 777001, Login: "otherperson"})
	if err != nil {
		t.Fatal(err)
	}
	otherToken, otherCSRF, _, err := st.CreateProfileSession(context.Background(), otherUser.ID)
	if err != nil {
		t.Fatal(err)
	}
	denied := publishRequestTo(t, h, otherToken, otherCSRF, "magenta", true, "")
	if denied.Code != http.StatusForbidden {
		t.Fatalf("non-owner %d %s", denied.Code, denied.Body.String())
	}
	if git.puts != putsBeforeReject {
		t.Fatal("non-owner wrote")
	}

	git.failReadme = 1
	first := publishRequestTo(t, h, session, csrf, "cobalt", true, "")
	if first.Code != http.StatusConflict {
		t.Fatalf("partial %d %s", first.Code, first.Body.String())
	}
	var job map[string]any
	_ = json.Unmarshal(first.Body.Bytes(), &job)
	if job["phase"] != phasePartialFailure || job["retry_readme"] != true {
		t.Fatalf("job %+v", job)
	}
	if job["snapshot_id"] != snap.SnapshotID {
		t.Fatalf("theme changed snapshot %v", job["snapshot_id"])
	}
	if strings.Contains(git.readme(), "cobalt") && !strings.Contains(git.readme(), "keep-this-line") {
		t.Fatal("partial write lost the readme")
	}

	retryReq := httptest.NewRequest(http.MethodPost, "/api/v1/public-profile/rainhuang0220/jobs/"+job["id"].(string)+"/retry", strings.NewReader(`{"confirm":true}`))
	retryReq.Header.Set("Authorization", "Bearer "+session)
	retryReq.Header.Set("X-CSRF-Token", csrf)
	retryRec := httptest.NewRecorder()
	h.ServeHTTP(retryRec, retryReq)
	if retryRec.Code != http.StatusOK {
		t.Fatalf("retry %d %s", retryRec.Code, retryRec.Body.String())
	}
	var retried map[string]any
	_ = json.Unmarshal(retryRec.Body.Bytes(), &retried)
	if retried["result"] != "published" || retried["snapshot_id"] != snap.SnapshotID {
		t.Fatalf("retry job %+v", retried)
	}
	readme := git.readme()
	if !strings.Contains(readme, "keep-this-line") || !strings.Contains(readme, "rainhuang0220.png") {
		t.Fatalf("readme lost content\n%s", readme)
	}
	if !strings.Contains(readme, "raw.githubusercontent.com/rainhuang0220/rainhuang0220/main/wheretoken/preview-light.svg?v=") {
		t.Fatalf("readme preview\n%s", readme)
	}
	alt, err := publicprofile.ReadmeAlt(publicprofile.ReadmeFacts{
		TotalDisplay: snap.Periods.All.Totals.Total.Display,
		DataStatus:   snap.DataStatus,
		AsOfDate:     snap.AsOfDate,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(readme, `alt="`+alt+`"`) || !strings.Contains(readme, `alt="portrait"`) {
		t.Fatalf("readme alt\n%s", readme)
	}
	if strings.Contains(readme, "0 measured") || strings.Contains(readme, "0.00") {
		t.Fatal("missing usage was written as zero")
	}
	puts := git.puts
	again := publishRequestTo(t, h, session, csrf, "cobalt", true, "")
	if again.Code != http.StatusOK {
		t.Fatalf("again %d %s", again.Code, again.Body.String())
	}
	var idem map[string]any
	_ = json.Unmarshal(again.Body.Bytes(), &idem)
	if idem["result"] != resultAlreadyPublished {
		t.Fatalf("idempotent %+v", idem)
	}
	if git.puts != puts {
		t.Fatalf("already published wrote files %d -> %d", puts, git.puts)
	}

	stored, err := readyStore(t).UserByLogin(context.Background(), "rainhuang0220")
	if err != nil {
		t.Fatal(err)
	}
	proj, err := readyStore(t).Projection(context.Background(), stored.ID)
	if err != nil {
		t.Fatal(err)
	}
	if proj.SnapshotID != snap.SnapshotID {
		t.Fatalf("projection id changed %s", proj.SnapshotID)
	}
	for _, path := range git.paths {
		if path != "README.md" && path != "wheretoken/preview-light.svg" && path != "wheretoken/preview-dark.svg" {
			t.Fatalf("unexpected path %s", path)
		}
	}
}

func TestPublishedSurfacesShareOneSnapshot(t *testing.T) {
	git := newMemGit(publishReadme)
	h := publishMux(t, git)
	device, session, csrf := seedOwner(t)
	now := time.Date(2026, 9, 24, 3, 0, 0, 0, time.UTC)
	proj := mustProjection(t, publicprofile.Input{
		Now: now, Loc: time.UTC, Version: "test",
		Events: []event.UsageEvent{
			authEvent("claude", "anthropic", now.Add(-2*time.Hour), 1000),
			authEvent("cursor", "cursor", now.Add(-time.Hour), 5000),
		},
	})
	putProjection(t, h, device, proj.raw)
	beforeReadme := git.readme()
	puts := git.puts

	git.failPath = "wheretoken/preview-light.svg"
	failed := publishRequestTo(t, h, session, csrf, "cobalt", true, "")
	if failed.Code != http.StatusConflict {
		t.Fatalf("svg failure status %d %s", failed.Code, failed.Body.String())
	}
	var failedJob map[string]any
	if err := json.Unmarshal(failed.Body.Bytes(), &failedJob); err != nil {
		t.Fatal(err)
	}
	if failedJob["phase"] == phasePublished || failedJob["result"] == "published" || failedJob["result"] == resultAlreadyPublished {
		t.Fatalf("svg failure reported success: %+v", failedJob)
	}
	if git.readme() != beforeReadme {
		t.Fatal("failed svg write updated the README")
	}
	if len(git.file("wheretoken/preview-light.svg")) != 0 || len(git.file("wheretoken/preview-dark.svg")) != 0 {
		t.Fatal("a failed preview write left an svg committed")
	}

	retryReq := httptest.NewRequest(http.MethodPost, "/api/v1/public-profile/rainhuang0220/jobs/"+failedJob["id"].(string)+"/retry", strings.NewReader(`{"confirm":true}`))
	retryReq.Header.Set("Authorization", "Bearer "+session)
	retryReq.Header.Set("X-CSRF-Token", csrf)
	retryRec := httptest.NewRecorder()
	h.ServeHTTP(retryRec, retryReq)
	if retryRec.Code != http.StatusOK {
		t.Fatalf("retry %d %s", retryRec.Code, retryRec.Body.String())
	}
	var published map[string]any
	if err := json.Unmarshal(retryRec.Body.Bytes(), &published); err != nil {
		t.Fatal(err)
	}
	if published["result"] != "published" || published["snapshot_id"] != proj.snap.SnapshotID {
		t.Fatalf("retry job %+v", published)
	}

	puts = git.puts
	againRetry := httptest.NewRequest(http.MethodPost, "/api/v1/public-profile/rainhuang0220/jobs/"+failedJob["id"].(string)+"/retry", strings.NewReader(`{"confirm":true}`))
	againRetry.Header.Set("Authorization", "Bearer "+session)
	againRetry.Header.Set("X-CSRF-Token", csrf)
	againRec := httptest.NewRecorder()
	h.ServeHTTP(againRec, againRetry)
	if againRec.Code != http.StatusConflict || !strings.Contains(againRec.Body.String(), "not retryable") {
		t.Fatalf("second retry %d %s", againRec.Code, againRec.Body.String())
	}
	if git.puts != puts {
		t.Fatal("retry of a finished job wrote files")
	}

	puts = git.puts
	second := publishRequestTo(t, h, session, csrf, "cobalt", true, "")
	if second.Code != http.StatusOK {
		t.Fatalf("second publish %d %s", second.Code, second.Body.String())
	}
	var idem map[string]any
	if err := json.Unmarshal(second.Body.Bytes(), &idem); err != nil {
		t.Fatal(err)
	}
	if idem["result"] != resultAlreadyPublished || idem["snapshot_id"] != proj.snap.SnapshotID {
		t.Fatalf("second publish %+v", idem)
	}
	if git.puts != puts {
		t.Fatalf("already published wrote %d files", git.puts-puts)
	}

	bad := httptest.NewRequest(http.MethodPut, "/api/v1/sync/public-profile", strings.NewReader(`{"schema":"wheretoken.public-profile"}`))
	bad.Header.Set("Authorization", "Bearer "+device)
	badRec := httptest.NewRecorder()
	h.ServeHTTP(badRec, bad)
	if badRec.Code != http.StatusBadRequest {
		t.Fatalf("bad sync %d", badRec.Code)
	}
	if publicSnapshotID(t, h, "rainhuang0220") != proj.snap.SnapshotID {
		t.Fatal("failed sync replaced the last good snapshot")
	}

	gotReq := httptest.NewRequest(http.MethodGet, "/api/v1/public-profile/rainhuang0220", nil)
	gotRec := httptest.NewRecorder()
	h.ServeHTTP(gotRec, gotReq)
	var envelope struct {
		DataRevision string                 `json:"data_revision"`
		Asset        string                 `json:"asset_revision"`
		Snapshot     publicprofile.Snapshot `json:"snapshot"`
	}
	if err := json.Unmarshal(gotRec.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.DataRevision != proj.snap.SnapshotID || envelope.Snapshot.SnapshotID != proj.snap.SnapshotID {
		t.Fatalf("api snapshot %s data %s", envelope.Snapshot.SnapshotID, envelope.DataRevision)
	}
	if envelope.Snapshot.GeneratedAt != proj.snap.GeneratedAt || envelope.Snapshot.AsOfDate != proj.snap.AsOfDate {
		t.Fatalf("timestamp generated %s/%s as_of %s/%s", envelope.Snapshot.GeneratedAt, proj.snap.GeneratedAt, envelope.Snapshot.AsOfDate, proj.snap.AsOfDate)
	}
	if envelope.Snapshot.Periods.All.Totals.Total.Display != proj.snap.Periods.All.Totals.Total.Display {
		t.Fatal("api total changed")
	}
	light := getPreview(t, h, "preview-light.svg")
	dark := getPreview(t, h, "preview-dark.svg")
	if !bytes.Equal(light, git.file("wheretoken/preview-light.svg")) || !bytes.Equal(dark, git.file("wheretoken/preview-dark.svg")) {
		t.Fatal("api previews differ from the files written to git")
	}
	for _, svg := range [][]byte{light, dark} {
		if !bytes.Contains(svg, []byte("<metadata id=\"wheretoken-snapshot-id\">"+proj.snap.SnapshotID+"</metadata>")) {
			t.Fatal("preview metadata snapshot differs")
		}
		if !bytes.Contains(svg, []byte("through "+proj.snap.AsOfDate)) {
			t.Fatal("preview date differs from the snapshot")
		}
	}
	key, err := publicprofile.CacheKey(proj.snap.SnapshotID, envelope.Asset)
	if err != nil {
		t.Fatal(err)
	}
	readme := git.readme()
	if strings.Count(readme, "?v="+key) != 3 {
		t.Fatalf("readme cache key count for %s\n%s", key, readme)
	}
	if strings.Contains(readme, "15c96d356fc90d8073524036f29bef83ef236f76f7b02a11e862b9cbb4c52e62-2f221b6893380bf9ebaecd0cce4fe8bf2abea777805c8dac1c88d9d34c4245cc") {
		t.Fatal("readme kept the previous cache key")
	}
	alt, err := publicprofile.ReadmeAlt(publicprofile.ReadmeFacts{
		TotalDisplay: proj.snap.Periods.All.Totals.Total.Display,
		DataStatus:   proj.snap.DataStatus,
		AsOfDate:     proj.snap.AsOfDate,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(readme, `alt="`+alt+`"`) || !strings.Contains(alt, proj.snap.Periods.All.Totals.Total.Display) {
		t.Fatalf("readme total/date alt %q\n%s", alt, readme)
	}
}

func getPreview(t *testing.T, h http.Handler, name string) []byte {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/public-profile/rainhuang0220/"+name, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("preview %s %d", name, rec.Code)
	}
	return rec.Body.Bytes()
}

func TestPublicProfileRemoteConflictDoesNotOverwrite(t *testing.T) {
	git := newMemGit(publishReadme)
	git.conflict = 1
	h := publishMux(t, git)
	device, session, csrf := seedOwner(t)
	raw := testProjection(t)
	put := httptest.NewRequest(http.MethodPut, "/api/v1/sync/public-profile", bytes.NewReader(raw))
	put.Header.Set("Authorization", "Bearer "+device)
	putRec := httptest.NewRecorder()
	h.ServeHTTP(putRec, put)
	if putRec.Code != http.StatusOK {
		t.Fatalf("sync %d %s", putRec.Code, putRec.Body.String())
	}
	before := git.readme()
	rec := publishRequestTo(t, h, session, csrf, "magenta", true, "")
	if rec.Code != http.StatusConflict {
		t.Fatalf("conflict %d %s", rec.Code, rec.Body.String())
	}
	var job map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &job)
	if job["phase"] != phaseConflict {
		t.Fatalf("phase %+v", job)
	}
	if git.readme() != before {
		t.Fatal("conflict overwrote the readme")
	}
}

func TestOAuthReturnToProfileDropsGitHubToken(t *testing.T) {
	gh := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "access_token") {
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"access_token":"gho_SHOULD_NOT_LEAK","token_type":"bearer"}`)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"id":4242,"login":"rainhuang0220","avatar_url":"https://avatars.githubusercontent.com/u/4242"}`)
	})
	oauth := testMux(t, gh)
	start := httptest.NewRequest(http.MethodGet, "/api/v1/auth/github?return_to="+url.QueryEscape("https://rainhuang0220.github.io/whereToken/profile/"), nil)
	startRec := httptest.NewRecorder()
	oauth.ServeHTTP(startRec, start)
	if startRec.Code != http.StatusFound {
		t.Fatalf("start %d %s", startRec.Code, startRec.Body.String())
	}
	loc := startRec.Header().Get("Location")
	if strings.Contains(loc, "scope=") && !strings.Contains(loc, "scope=&") && strings.Contains(loc, "scope=repo") {
		t.Fatalf("oauth requested repo scope: %s", loc)
	}
	u, err := url.Parse(loc)
	if err != nil {
		t.Fatal(err)
	}
	var stateCookie *http.Cookie
	for _, c := range startRec.Result().Cookies() {
		if c.Name == cookieOAuthState {
			stateCookie = c
		}
	}
	cb := httptest.NewRequest(http.MethodGet, "/api/v1/auth/github/callback?code=ok&state="+u.Query().Get("state"), nil)
	cb.AddCookie(stateCookie)
	for _, c := range startRec.Result().Cookies() {
		if c.Name == "wt_return" {
			cb.AddCookie(c)
		}
	}
	cbRec := httptest.NewRecorder()
	oauth.ServeHTTP(cbRec, cb)
	if cbRec.Code != http.StatusFound {
		t.Fatalf("callback %d %s", cbRec.Code, cbRec.Body.String())
	}
	next := cbRec.Header().Get("Location")
	if !strings.HasPrefix(next, "https://rainhuang0220.github.io/whereToken/profile/#wt_code=") {
		t.Fatalf("return %s", next)
	}
	if strings.Contains(next, "gho_") || strings.Contains(cbRec.Body.String(), "gho_") {
		t.Fatal("github token leaked")
	}
	code := strings.TrimPrefix(next, "https://rainhuang0220.github.io/whereToken/profile/#wt_code=")
	ex := httptest.NewRequest(http.MethodPost, "/api/v1/public-profile/session", strings.NewReader(`{"code":"`+code+`"}`))
	exRec := httptest.NewRecorder()
	oauth.ServeHTTP(exRec, ex)
	if exRec.Code != http.StatusOK {
		t.Fatalf("exchange %d %s", exRec.Code, exRec.Body.String())
	}
	if strings.Contains(exRec.Body.String(), "gho_") {
		t.Fatal("exchange returned a github token")
	}
	var sess map[string]string
	_ = json.Unmarshal(exRec.Body.Bytes(), &sess)
	if sess["login"] != "rainhuang0220" || !strings.HasPrefix(sess["token"], "wtp_1.") || sess["csrf"] == "" {
		t.Fatalf("session %+v", sess)
	}
	again := httptest.NewRequest(http.MethodPost, "/api/v1/public-profile/session", strings.NewReader(`{"code":"`+code+`"}`))
	againRec := httptest.NewRecorder()
	oauth.ServeHTTP(againRec, again)
	if againRec.Code != http.StatusUnauthorized {
		t.Fatalf("replay %d", againRec.Code)
	}
	bad := httptest.NewRequest(http.MethodGet, "/api/v1/auth/github?return_to=https://evil.example/whereToken/profile/", nil)
	badRec := httptest.NewRecorder()
	oauth.ServeHTTP(badRec, bad)
	if badRec.Code != http.StatusBadRequest {
		t.Fatalf("open redirect %d", badRec.Code)
	}
}

func publishRequestTo(t *testing.T, h http.Handler, session, csrf, palette string, confirm bool, extra string) *httptest.ResponseRecorder {
	t.Helper()
	body := `{"palette":"` + palette + `","confirm":` + boolJSON(confirm) + `}`
	if extra != "" {
		body = `{"palette":"` + palette + `","confirm":` + boolJSON(confirm) + extra
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/public-profile/rainhuang0220/publish", strings.NewReader(body))
	if session != "" {
		req.Header.Set("Authorization", "Bearer "+session)
	}
	if csrf != "" {
		req.Header.Set("X-CSRF-Token", csrf)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestPublicProjectionRejectsWeakerSnapshot(t *testing.T) {
	h := publishMux(t, newMemGit(publishReadme))
	token := seedLogin(t, 777001, "snapshot-guard")
	now := time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC)
	complete := mustProjection(t, publicprofile.Input{
		Now: now, Loc: time.UTC, Version: "test",
		Events: []event.UsageEvent{
			authEvent("claude", "anthropic", now.Add(-2*time.Hour), 1000),
			authEvent("cursor", "cursor", now.Add(-time.Hour), 5000),
		},
	})
	updated := mustProjection(t, publicprofile.Input{
		Now: now.Add(time.Minute), Loc: time.UTC, Version: "test",
		Events: []event.UsageEvent{
			authEvent("claude", "anthropic", now.Add(-2*time.Hour), 1100),
			authEvent("cursor", "cursor", now.Add(-time.Hour), 5200),
		},
	})
	putProjection(t, h, token, complete.raw)
	if got := publicSnapshotID(t, h, "snapshot-guard"); got != complete.snap.SnapshotID {
		t.Fatalf("stored %s", got)
	}
	putProjection(t, h, token, updated.raw)
	if got := publicSnapshotID(t, h, "snapshot-guard"); got != updated.snap.SnapshotID {
		t.Fatal("complete update did not replace the public snapshot")
	}
	partial := mustProjection(t, publicprofile.Input{
		Now: now.Add(2 * time.Minute), Loc: time.UTC, Version: "test",
		Events: []event.UsageEvent{
			{Source: "claude", Vendor: "anthropic", Timestamp: now.Add(-2 * time.Hour), Miss: 1000, Quality: event.QualityDegraded},
		},
	})
	if partial.snap.DataStatus != publicprofile.StatusPartial {
		t.Fatalf("partial status %s", partial.snap.DataStatus)
	}
	kept := putProjection(t, h, token, partial.raw)
	if kept["kept"] != "previous" || kept["snapshot_id"] != updated.snap.SnapshotID {
		t.Fatalf("downgrade response %+v", kept)
	}
	if got := publicSnapshotID(t, h, "snapshot-guard"); got != updated.snap.SnapshotID {
		t.Fatal("partial snapshot replaced a complete public projection")
	}
}

func TestPublicProjectionRestoresCoverageAndKeepsProviderHistory(t *testing.T) {
	h := publishMux(t, newMemGit(publishReadme))
	token := seedLogin(t, 777002, "coverage-guard")
	now := time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC)
	partial := mustProjection(t, publicprofile.Input{
		Now: now, Loc: time.UTC, Version: "test",
		Events: []event.UsageEvent{
			{Source: "claude", Vendor: "anthropic", Timestamp: now.Add(-time.Hour), Miss: 40, Quality: event.QualityDegraded},
		},
	})
	complete := mustProjection(t, publicprofile.Input{
		Now: now.Add(time.Minute), Loc: time.UTC, Version: "test",
		Events: []event.UsageEvent{
			authEvent("claude", "anthropic", now.Add(-time.Hour), 40),
			authEvent("cursor", "cursor", now.Add(-time.Hour), 80),
		},
	})
	putProjection(t, h, token, partial.raw)
	putProjection(t, h, token, complete.raw)
	if got := publicSnapshotID(t, h, "coverage-guard"); got != complete.snap.SnapshotID {
		t.Fatal("complete snapshot did not replace a partial projection")
	}

	history := mustProjection(t, publicprofile.Input{
		Now: now, Loc: time.UTC, Version: "test",
		Events: []event.UsageEvent{
			authEvent("claude", "anthropic", now.Add(-2*time.Hour), 100),
			authEvent("cursor", "cursor", now.Add(-time.Hour), 900),
		},
	})
	failed := mustProjection(t, publicprofile.Input{
		Now: now.Add(time.Minute), Loc: time.UTC, Version: "test",
		Errors: []string{"cursor: upstream"},
		Events: []event.UsageEvent{
			authEvent("claude", "anthropic", now.Add(-2*time.Hour), 100),
			{
				Source: "cursor", Vendor: "cursor", RequestID: "cursor-local",
				Timestamp: now.Add(-time.Hour),
				Quality:   event.QualityDegraded, Derivation: event.DeriveRaw,
			},
		},
	})
	if failed.snap.DataStatus != publicprofile.StatusPartial {
		t.Fatalf("provider failure status %s", failed.snap.DataStatus)
	}
	cursor, ok := agentByID(failed.snap, "cursor")
	if !ok || cursor.Coverage.Reason != publicprofile.ReasonAPIFailed || cursor.Coverage.Tokens != publicprofile.StatusUnavailable {
		t.Fatalf("cursor failure was not visible: %+v", cursor.Coverage)
	}
	alt, err := publicprofile.ReadmeAlt(publicprofile.ReadmeFacts{
		TotalDisplay: failed.snap.Periods.All.Totals.Total.Display,
		DataStatus:   failed.snap.DataStatus,
		AsOfDate:     failed.snap.AsOfDate,
	})
	if err != nil || !strings.Contains(alt, "partial coverage") || !strings.Contains(alt, "September 23, 2026") {
		t.Fatalf("readme alt %q err=%v", alt, err)
	}
	token2 := seedLogin(t, 777003, "cursor-guard")
	putProjection(t, h, token2, history.raw)
	kept := putProjection(t, h, token2, failed.raw)
	if kept["kept"] != "previous" || publicSnapshotID(t, h, "cursor-guard") != history.snap.SnapshotID {
		t.Fatal("cursor API failure replaced measured public history")
	}
}

func TestPublishNewsprintAfterMagentaMatchesCommittedFiles(t *testing.T) {
	git := newMemGit(publishReadme)
	h := publishMux(t, git)
	_, session, csrf := seedOwner(t)
	proj := mustProjection(t, publicprofile.Input{
		Now: time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC), Loc: time.UTC, Version: "test",
		Events: []event.UsageEvent{
			authEvent("claude", "anthropic", time.Date(2026, 9, 23, 10, 0, 0, 0, time.UTC), 250),
		},
	})
	device := seedLogin(t, 4242, "rainhuang0220")
	putProjection(t, h, device, proj.raw)

	magenta := publishRequestTo(t, h, session, csrf, "magenta", true, "")
	if magenta.Code != http.StatusOK {
		t.Fatalf("magenta %d %s", magenta.Code, magenta.Body.String())
	}
	light := git.file("wheretoken/preview-light.svg")
	dark := git.file("wheretoken/preview-dark.svg")
	if bytes.Contains(light, []byte("newsprint-ink-")) || bytes.Contains(dark, []byte("newsprint-ink-")) {
		t.Fatal("magenta preview contained newsprint ink")
	}
	news := publishRequestTo(t, h, session, csrf, "newsprint", true, "")
	if news.Code != http.StatusOK {
		t.Fatalf("newsprint %d %s", news.Code, news.Body.String())
	}
	light = git.file("wheretoken/preview-light.svg")
	dark = git.file("wheretoken/preview-dark.svg")
	readme := git.readme()
	if !bytes.Contains(light, []byte("newsprint-ink-")) || !bytes.Contains(dark, []byte("newsprint-ink-")) {
		t.Fatal("newsprint publish did not commit newsprint ink")
	}
	if !strings.Contains(readme, "raw.githubusercontent.com/rainhuang0220/rainhuang0220/main/wheretoken/preview-light.svg?v=") {
		t.Fatalf("readme did not point at the newsprint revision\n%s", readme)
	}
	puts := git.puts
	fake := []byte(`<svg xmlns="http://www.w3.org/2000/svg"><text>magenta</text></svg>`)
	git.replace("wheretoken/preview-light.svg", fake)
	git.replace("wheretoken/preview-dark.svg", fake)
	again := publishRequestTo(t, h, session, csrf, "newsprint", true, "")
	if again.Code != http.StatusOK {
		t.Fatalf("repair %d %s", again.Code, again.Body.String())
	}
	if git.puts == puts {
		t.Fatal("already-published skipped a magenta file while the request was newsprint")
	}
	if !bytes.Contains(git.file("wheretoken/preview-light.svg"), []byte("newsprint-ink-")) || !bytes.Contains(git.file("wheretoken/preview-dark.svg"), []byte("newsprint-ink-")) {
		t.Fatal("repair left a non-newsprint preview committed")
	}
}

func authEvent(source, vendor string, at time.Time, miss int64) event.UsageEvent {
	return event.UsageEvent{Source: source, Vendor: vendor, Timestamp: at, Miss: miss, Quality: event.QualityAuthoritative}
}

type builtProjection struct {
	raw  []byte
	snap publicprofile.Snapshot
}

func mustProjection(t *testing.T, in publicprofile.Input) builtProjection {
	t.Helper()
	snap, err := publicprofile.Build(in)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := publicprofile.Marshal(snap)
	if err != nil {
		t.Fatal(err)
	}
	return builtProjection{raw: raw, snap: snap}
}

func seedLogin(t *testing.T, id int64, login string) string {
	t.Helper()
	st := readyStore(t)
	user, err := st.UpsertGitHubUser(context.Background(), GitHubIdentity{ID: id, Login: login, AvatarURL: "https://avatars.githubusercontent.com/u/1"})
	if err != nil {
		t.Fatal(err)
	}
	_, token, err := st.InsertDevice(context.Background(), user.ID, DeviceMeta{OS: "darwin", Arch: "arm64", ClientVersion: "test", Label: "test"})
	if err != nil {
		t.Fatal(err)
	}
	return token
}

func putProjection(t *testing.T, h http.Handler, token string, raw []byte) map[string]any {
	t.Helper()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/sync/public-profile", bytes.NewReader(raw))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("sync %d %s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	return body
}

func publicSnapshotID(t *testing.T, h http.Handler, login string) string {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/public-profile/"+login, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("get %d %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Snapshot struct {
			SnapshotID string `json:"snapshot_id"`
		} `json:"snapshot"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	return body.Snapshot.SnapshotID
}

func agentByID(snap publicprofile.Snapshot, id string) (publicprofile.Breakdown, bool) {
	for _, row := range snap.Periods.All.ByAgent {
		if row.ID == id {
			return row, true
		}
	}
	return publicprofile.Breakdown{}, false
}

func (g *memGit) file(path string) []byte {
	g.mu.Lock()
	defer g.mu.Unlock()
	return append([]byte(nil), g.files["rainhuang0220/rainhuang0220\n"+path].Body...)
}

func (g *memGit) replace(path string, body []byte) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.files["rainhuang0220/rainhuang0220\n"+path] = memGitFile{SHA: memDigest(body), Body: append([]byte(nil), body...)}
}

func boolJSON(v bool) string {
	if v {
		return "true"
	}
	return "false"
}
