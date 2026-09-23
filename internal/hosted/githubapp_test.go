package hosted

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestAppJWTSignsWithTheAppKeyAndDoesNotUseRepoScope(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC)
	token, err := signAppJWT("12345", key, now)
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("jwt %s", token)
	}
	claimsRaw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(claimsRaw), "repo") {
		t.Fatalf("claims request a scope: %s", claimsRaw)
	}
	var claims struct {
		Iss string `json:"iss"`
		Exp int64  `json:"exp"`
	}
	if err := json.Unmarshal(claimsRaw, &claims); err != nil {
		t.Fatal(err)
	}
	if claims.Iss != "12345" || claims.Exp != now.Add(8*time.Minute).Unix() {
		t.Fatalf("claims %+v", claims)
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if err := rsa.VerifyPKCS1v15(&key.PublicKey, crypto.SHA256, sum[:], sig); err != nil {
		t.Fatal(err)
	}
}

func TestInstallationTokenIsCachedAndPutHonorsExpectedSHA(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	var mints int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/access_tokens"):
			if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
				t.Errorf("auth %s", r.Header.Get("Authorization"))
			}
			mints++
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"token":"ghs_test","expires_at":"2030-01-01T00:00:00Z"}`)
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/contents/README.md"):
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"sha":"abc","encoding":"base64","content":"aGVsbG8="}`)
		case r.Method == http.MethodPut:
			var body map[string]string
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body["sha"] != "abc" {
				w.WriteHeader(http.StatusConflict)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"content":{"sha":"def"}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	c := &appContents{
		appID:          "12345",
		installationID: "99",
		key:            key,
		api:            srv.URL,
		client:         srv.Client(),
		now:            time.Now,
	}
	ctx := context.Background()
	file, err := c.GetFile(ctx, "rainhuang0220/rainhuang0220", "README.md", "main")
	if err != nil {
		t.Fatal(err)
	}
	if string(file.Content) != "hello" || file.SHA != "abc" {
		t.Fatalf("%s %q", file.SHA, file.Content)
	}
	if _, err := c.GetFile(ctx, "rainhuang0220/rainhuang0220", "README.md", "main"); err != nil {
		t.Fatal(err)
	}
	if mints != 1 {
		t.Fatalf("installation token minted %d times", mints)
	}
	if _, err := c.PutFile(ctx, "rainhuang0220/rainhuang0220", "README.md", "main", "whereToken: update public profile preview", []byte("hello"), "stale"); !errorsIsConflict(err) {
		t.Fatalf("stale sha: %v", err)
	}
	sha, err := c.PutFile(ctx, "rainhuang0220/rainhuang0220", "README.md", "main", "whereToken: update public profile preview", []byte("hello"), "abc")
	if err != nil || sha != "def" {
		t.Fatalf("put %s %v", sha, err)
	}
}

func errorsIsConflict(err error) bool {
	return err == ErrGitConflict
}
