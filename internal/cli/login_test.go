package cli

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/rainhuang0220/whereToken/internal/adapter"
	"github.com/rainhuang0220/whereToken/internal/adapter/testhome"
	"github.com/rainhuang0220/whereToken/internal/credstore"
	"github.com/rainhuang0220/whereToken/internal/event"
	"github.com/rainhuang0220/whereToken/internal/metric"
	"github.com/rainhuang0220/whereToken/internal/scan"
)

type memCreds struct {
	m map[string]string
}

func (m *memCreds) Get(k string) (string, error) {
	v, ok := m.m[k]
	if !ok {
		return "", credstore.ErrNotFound
	}
	return v, nil
}
func (m *memCreds) Set(k, v string) error { m.m[k] = v; return nil }
func (m *memCreds) Delete(k string) error { delete(m.m, k); return nil }

func jsonRes(code int, v any) (*http.Response, error) {
	var raw []byte
	if v != nil {
		raw, _ = json.Marshal(v)
	}
	return &http.Response{
		StatusCode: code,
		Body:       io.NopCloser(bytes.NewReader(raw)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}, nil
}

func TestLoginStoresDeviceTokenAndDoesNotPrintSecret(t *testing.T) {
	creds := &memCreds{m: map[string]string{}}
	n := 0
	app, out, errb := testApp([]string{"login"})
	app.Creds = creds
	app.GOOS, app.GOARCH = "darwin", "arm64"
	app.Version = "0.7.0"
	app.LookupEnv = func(k string) string {
		if k == "WHERETOKEN_HOSTED_URL" {
			return "https://wheretoken.plainlist.space"
		}
		return ""
	}
	app.OpenURL = func(string) error { return nil }
	app.Sleep = func(time.Duration) {}
	app.HTTPDo = func(req *http.Request) (*http.Response, error) {
		switch {
		case strings.HasSuffix(req.URL.Path, "/pair/start"):
			return jsonRes(200, map[string]any{
				"display_code":     "ABCD-EFGH",
				"device_secret":    "sekrit",
				"verification_url": "https://wheretoken.plainlist.space/pair/ABCD-EFGH",
			})
		case strings.HasSuffix(req.URL.Path, "/pair/status"):
			n++
			if n == 1 {
				return jsonRes(200, map[string]any{"status": "pending"})
			}
			return jsonRes(200, map[string]any{
				"status":          "approved",
				"device_token":    "wtd_1.tok",
				"user_login":      "rainhuang0220",
				"device_id":       "dev-uuid",
				"source_hmac_key": base64.RawURLEncoding.EncodeToString(make([]byte, 32)),
			})
		default:
			return jsonRes(404, nil)
		}
	}
	if code := app.Run(); code != 0 {
		t.Fatalf("exit %d err=%s", code, errb.String())
	}
	if creds.m[credstore.KeyDeviceToken] != "wtd_1.tok" {
		t.Fatalf("token %v", creds.m)
	}
	s := out.String()
	if !strings.Contains(s, "https://wheretoken.plainlist.space/pair/ABCD-EFGH") {
		t.Fatalf("url: %s", s)
	}
	if strings.Contains(s, "sekrit") || strings.Contains(s, "wtd_1.tok") {
		t.Fatalf("secret printed: %s", s)
	}
	if !strings.Contains(s, "rainhuang0220") {
		t.Fatalf("login: %s", s)
	}
}

func TestLogoutRequiresRemoteRevokeBeforeDeleting(t *testing.T) {
	creds := &memCreds{m: map[string]string{credstore.KeyDeviceToken: "wtd_1.tok"}}
	app, _, errb := testApp([]string{"logout"})
	app.Creds = creds
	app.LookupEnv = func(string) string { return "https://example.test" }
	app.HTTPDo = func(req *http.Request) (*http.Response, error) {
		return nil, io.EOF
	}
	if app.Run() == 0 {
		t.Fatal("logout succeeded while offline")
	}
	if creds.m[credstore.KeyDeviceToken] != "wtd_1.tok" {
		t.Fatal("deleted local token after network failure")
	}
	if !strings.Contains(errb.String(), "not deleted") {
		t.Fatalf("%s", errb.String())
	}
}

func TestSyncUploadsAllowlistPayload(t *testing.T) {
	hmacKey := make([]byte, 32)
	creds := &memCreds{m: map[string]string{
		credstore.KeyDeviceToken: "wtd_1.tok",
		credstore.KeyDeviceID:    "dev-1",
		credstore.KeyHMAC:        base64.RawURLEncoding.EncodeToString(hmacKey),
	}}
	var gotBody []byte
	app, _, errb := testApp([]string{"sync"})
	app.Creds = creds
	app.Home = testhome.New(t.TempDir())
	app.LookupEnv = func(k string) string {
		if k == "WHERETOKEN_HOSTED_URL" {
			return "https://wheretoken.plainlist.space"
		}
		return ""
	}
	app.Scan = func(adapter.Home) scan.Result {
		ev := event.UsageEvent{
			Source: "claude", Vendor: "anthropic", Model: "claude-opus-4.6",
			SourceRoot: "/Users/rainhuang/.claude", Workspace: "/Users/rainhuang/proj",
			SessionID: "sess", Timestamp: time.Date(2026, 9, 3, 10, 0, 0, 0, time.UTC),
			Miss: 1000, Quality: event.QualityAuthoritative, Derivation: event.DeriveRaw,
		}
		return scan.Result{Events: []event.UsageEvent{ev}, Summary: metric.Aggregate([]event.UsageEvent{ev}, nil)}
	}
	app.HTTPDo = func(req *http.Request) (*http.Response, error) {
		if req.Method == http.MethodPut && strings.HasSuffix(req.URL.Path, "/sync/batch") {
			gotBody, _ = io.ReadAll(req.Body)
			return jsonRes(200, map[string]any{"ok": true})
		}
		return jsonRes(404, nil)
	}
	if code := app.Run(); code != 0 {
		t.Fatalf("exit %d %s", code, errb.String())
	}
	low := strings.ToLower(string(gotBody))
	for _, bad := range []string{"prompt", "workspace", "/users/rainhuang", "sess", "source_root"} {
		if strings.Contains(low, bad) {
			t.Fatalf("payload contains %q: %s", bad, gotBody)
		}
	}
}

func TestReportDoesNotRequireLogin(t *testing.T) {
	app, _, errb := testApp(nil)
	app.Scan = func(adapter.Home) scan.Result {
		return scan.Result{Errors: []string{}}
	}
	if code := app.Run(); code != 0 {
		t.Fatalf("report exit %d %s", code, errb.String())
	}
}
