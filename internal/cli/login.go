package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/rainhuang0220/whereToken/internal/adapter"
	"github.com/rainhuang0220/whereToken/internal/community"
	"github.com/rainhuang0220/whereToken/internal/credstore"
)

func (a *App) hostedBase() string {
	if a.LookupEnv != nil {
		if v := strings.TrimSpace(a.LookupEnv("WHERETOKEN_HOSTED_URL")); v != "" {
			return strings.TrimRight(v, "/")
		}
	}
	return "https://wheretoken.plainlist.space"
}

func (a *App) credStore(home adapter.Home) credstore.Store {
	if a.Creds != nil {
		return a.Creds
	}
	return credstore.Open(filepath.Dir(community.ConfigPath(home)))
}

func (a *App) runLogin(flags Flags, home adapter.Home) int {
	base := a.hostedBase()
	label := defaultDeviceLabel(a.GOOS, a.GOARCH)
	body, _ := json.Marshal(map[string]string{
		"os":             a.GOOS,
		"arch":           a.GOARCH,
		"client_version": a.Version,
		"label":          label,
	})
	res, err := a.jsonRequest(http.MethodPost, base+"/api/v1/pair/start", body, "")
	if err != nil {
		fmt.Fprintln(a.Stderr, err.Error())
		return ExitFail
	}
	code, _ := res["display_code"].(string)
	secret, _ := res["device_secret"].(string)
	verify, _ := res["verification_url"].(string)
	if code == "" || secret == "" || verify == "" {
		fmt.Fprintln(a.Stderr, "pair start failed")
		return ExitFail
	}
	fmt.Fprintf(a.Stdout, "Open this URL to connect your device:\n\n%s\n\nWaiting for approval...\n", verify)
	if a.OpenURL != nil {
		_ = a.OpenURL(verify)
	} else {
		_ = openBrowser(verify)
	}
	deadline := a.Now().Add(10 * time.Minute)
	for a.Now().Before(deadline) {
		a.sleep(time.Second)
		st, err := a.jsonRequest(http.MethodPost, base+"/api/v1/pair/status", mustJSON(map[string]string{
			"display_code":  code,
			"device_secret": secret,
		}), "")
		if err != nil {
			continue
		}
		switch st["status"] {
		case "pending":
			continue
		case "denied":
			fmt.Fprintln(a.Stderr, "device pairing was rejected")
			return ExitFail
		case "expired":
			fmt.Fprintln(a.Stderr, "pairing code expired")
			return ExitFail
		case "approved":
			tok, _ := st["device_token"].(string)
			login, _ := st["user_login"].(string)
			dev, _ := st["device_id"].(string)
			hmacKey, _ := st["source_hmac_key"].(string)
			cs := a.credStore(home)
			if err := cs.Set(credstore.KeyDeviceToken, tok); err != nil {
				fmt.Fprintln(a.Stderr, err.Error())
				return ExitFail
			}
			_ = cs.Set(credstore.KeyLogin, login)
			_ = cs.Set(credstore.KeyDeviceID, dev)
			_ = cs.Set(credstore.KeyHMAC, hmacKey)
			fmt.Fprintf(a.Stdout, "✓ Signed in as %s\n✓ Device connected\n", login)
			if flags.NoSync {
				fmt.Fprintf(a.Stdout, "\nOpen:\n%s\n", base)
				return ExitOK
			}
			fmt.Fprintln(a.Stdout, "Syncing aggregated usage only — prompts, code and file paths are never uploaded.")
			_, sources, syncErr := a.syncOnce(flags, home)
			if syncErr != nil {
				fmt.Fprintf(a.Stderr, "! Initial sync failed: %s\n\nRun:\n  wheretoken sync\n", syncErr.Error())
				return ExitFail
			}
			fmt.Fprintf(a.Stdout, "✓ Scanned local usage\n✓ Synced %d sources\n\nOpen:\n%s\n", sources, base)
			return ExitOK
		}
	}
	fmt.Fprintln(a.Stderr, "timed out waiting for approval")
	return ExitFail
}

func (a *App) runLogout(flags Flags, home adapter.Home) int {
	cs := a.credStore(home)
	tok, err := cs.Get(credstore.KeyDeviceToken)
	if err != nil {
		fmt.Fprintln(a.Stderr, "not logged in")
		return ExitFail
	}
	base := a.hostedBase()
	req, _ := http.NewRequest(http.MethodPost, base+"/api/v1/devices/self/revoke", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	res, err := a.doHTTP(req)
	if err != nil {
		fmt.Fprintln(a.Stderr, "could not revoke device on server:", err.Error())
		fmt.Fprintln(a.Stderr, "local credentials were not deleted; retry when online")
		return ExitFail
	}
	defer res.Body.Close()
	if res.StatusCode >= 400 && res.StatusCode != http.StatusUnauthorized {
		fmt.Fprintf(a.Stderr, "revoke failed: HTTP %d\n", res.StatusCode)
		return ExitFail
	}
	_ = cs.Delete(credstore.KeyDeviceToken)
	_ = cs.Delete(credstore.KeyHMAC)
	_ = cs.Delete(credstore.KeyDeviceID)
	_ = cs.Delete(credstore.KeyLogin)
	fmt.Fprintln(a.Stdout, "logged out")
	return ExitOK
}

func defaultDeviceLabel(goos, arch string) string {
	switch goos {
	case "darwin":
		return "Mac"
	case "windows":
		return "Windows PC"
	case "linux":
		return "Linux"
	default:
		return goos + " " + arch
	}
}

func (a *App) sleep(d time.Duration) {
	if a.Sleep != nil {
		a.Sleep(d)
		return
	}
	time.Sleep(d)
}

func (a *App) doHTTP(req *http.Request) (*http.Response, error) {
	if a.HTTPDo != nil {
		return a.HTTPDo(req)
	}
	return http.DefaultClient.Do(req)
}

func (a *App) jsonRequest(method, url string, body []byte, bearer string) (map[string]any, error) {
	req, err := http.NewRequest(method, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	res, err := a.doHTTP(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if res.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d", res.StatusCode)
	}
	if len(raw) == 0 {
		return map[string]any{}, nil
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func mustJSON(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}

func openBrowser(u string) error {
	for _, c := range [][]string{{"open", u}, {"xdg-open", u}} {
		if err := exec.Command(c[0], c[1:]...).Start(); err == nil {
			return nil
		}
	}
	return fmt.Errorf("open browser")
}
