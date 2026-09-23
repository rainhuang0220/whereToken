package hosted

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

var (
	ErrGitConflict     = errors.New("remote content changed")
	ErrGitMissing      = errors.New("remote file missing")
	ErrGitUnconfigured = errors.New("github app unconfigured")
)

// ContentFile is one allowlisted repository file.
type ContentFile struct {
	SHA     string
	Content []byte
}

// ContentsClient reads and writes exact paths. Callers supply the repo,
// path, and expected SHA. The browser never does.
type ContentsClient interface {
	GetFile(ctx context.Context, repo, path, ref string) (ContentFile, error)
	PutFile(ctx context.Context, repo, path, branch, message string, content []byte, expectedSHA string) (string, error)
}

type appContents struct {
	appID          string
	installationID string
	key            *rsa.PrivateKey
	api            string
	client         *http.Client
	now            func() time.Time

	mu     sync.Mutex
	token  string
	expiry time.Time
}

func parseAppKey(pemBytes []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, errors.New("github app key")
	}
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	key, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("github app key")
	}
	return key, nil
}

func newAppContents(cfg ProfilePublish, now func() time.Time) (*appContents, error) {
	if strings.TrimSpace(cfg.AppID) == "" || strings.TrimSpace(cfg.InstallationID) == "" || len(cfg.PrivateKeyPEM) == 0 || strings.TrimSpace(cfg.Repo) == "" {
		return nil, ErrGitUnconfigured
	}
	key, err := parseAppKey(cfg.PrivateKeyPEM)
	if err != nil {
		return nil, err
	}
	if now == nil {
		now = time.Now
	}
	return &appContents{
		appID:          cfg.AppID,
		installationID: cfg.InstallationID,
		key:            key,
		api:            "https://api.github.com",
		client:         &http.Client{Timeout: 15 * time.Second},
		now:            now,
	}, nil
}

func signAppJWT(appID string, key *rsa.PrivateKey, now time.Time) (string, error) {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256","typ":"JWT"}`))
	claims := fmt.Sprintf(`{"iat":%d,"exp":%d,"iss":%q}`, now.Add(-60*time.Second).Unix(), now.Add(8*time.Minute).Unix(), appID)
	payload := base64.RawURLEncoding.EncodeToString([]byte(claims))
	signing := header + "." + payload
	sum := sha256.Sum256([]byte(signing))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, sum[:])
	if err != nil {
		return "", err
	}
	return signing + "." + base64.RawURLEncoding.EncodeToString(sig), nil
}

func (c *appContents) installationToken(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.token != "" && c.now().Add(time.Minute).Before(c.expiry) {
		return c.token, nil
	}
	jwt, err := signAppJWT(c.appID, c.key, c.now())
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.api+"/app/installations/"+c.installationID+"/access_tokens", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+jwt)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "whereToken-hosted")
	res, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 1<<16))
	if err != nil {
		return "", err
	}
	if res.StatusCode >= 300 {
		return "", fmt.Errorf("github installation token: http %d", res.StatusCode)
	}
	var out struct {
		Token     string    `json:"token"`
		ExpiresAt time.Time `json:"expires_at"`
	}
	if err := json.Unmarshal(body, &out); err != nil || out.Token == "" {
		return "", errors.New("github installation token")
	}
	c.token = out.Token
	c.expiry = out.ExpiresAt
	return c.token, nil
}

func (c *appContents) GetFile(ctx context.Context, repo, path, ref string) (ContentFile, error) {
	tok, err := c.installationToken(ctx)
	if err != nil {
		return ContentFile{}, err
	}
	u := c.api + "/repos/" + repo + "/contents/" + path
	if ref != "" {
		u += "?ref=" + ref
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return ContentFile{}, err
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "whereToken-hosted")
	res, err := c.client.Do(req)
	if err != nil {
		return ContentFile{}, err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	if err != nil {
		return ContentFile{}, err
	}
	if res.StatusCode == http.StatusNotFound {
		return ContentFile{}, ErrGitMissing
	}
	if res.StatusCode >= 300 {
		return ContentFile{}, fmt.Errorf("github get: http %d", res.StatusCode)
	}
	var out struct {
		SHA      string `json:"sha"`
		Content  string `json:"content"`
		Encoding string `json:"encoding"`
	}
	if err := json.Unmarshal(body, &out); err != nil || out.SHA == "" {
		return ContentFile{}, errors.New("github get")
	}
	if out.Encoding != "base64" && out.Encoding != "" {
		return ContentFile{}, errors.New("github encoding")
	}
	raw, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(out.Content, "\n", ""))
	if err != nil {
		return ContentFile{}, err
	}
	return ContentFile{SHA: out.SHA, Content: raw}, nil
}

func (c *appContents) PutFile(ctx context.Context, repo, path, branch, message string, content []byte, expectedSHA string) (string, error) {
	tok, err := c.installationToken(ctx)
	if err != nil {
		return "", err
	}
	payload := map[string]string{
		"message": message,
		"content": base64.StdEncoding.EncodeToString(content),
		"branch":  branch,
	}
	if expectedSHA != "" {
		payload["sha"] = expectedSHA
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, c.api+"/repos/"+repo+"/contents/"+path, strings.NewReader(string(raw)))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "whereToken-hosted")
	res, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return "", err
	}
	if res.StatusCode == http.StatusConflict || (res.StatusCode == http.StatusUnprocessableEntity && strings.Contains(strings.ToLower(string(body)), "sha")) {
		return "", ErrGitConflict
	}
	if res.StatusCode >= 300 {
		return "", fmt.Errorf("github put: http %d", res.StatusCode)
	}
	var out struct {
		Content struct {
			SHA string `json:"sha"`
		} `json:"content"`
	}
	if err := json.Unmarshal(body, &out); err != nil || out.Content.SHA == "" {
		return "", errors.New("github put")
	}
	return out.Content.SHA, nil
}
