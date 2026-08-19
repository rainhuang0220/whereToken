package cli

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	defaultRepo      = "rainhuang0220/whereToken"
	maxReleaseBody   = 64 << 20
	releaseUserAgent = "wheretoken-update"
)

func (a *App) runUpdate(quiet bool) int {
	exe, err := a.exePath()
	if err != nil {
		fmt.Fprintln(a.Stderr, err.Error())
		return ExitFail
	}
	if brewManaged(exe) {
		if !quiet {
			fmt.Fprintln(a.Stderr, "Homebrew install; running brew upgrade wheretoken")
		}
		if err := a.runCmd("brew", "upgrade", "wheretoken"); err != nil {
			fmt.Fprintln(a.Stderr, err.Error())
			return ExitFail
		}
		return ExitOK
	}
	asset := releaseAssetName(a.GOOS, a.GOARCH)
	base := a.releaseBase()
	if !quiet {
		fmt.Fprintf(a.Stderr, "wheretoken: downloading %s/%s\n", base, asset)
	}
	raw, err := a.httpGet(base + "/" + asset)
	if err != nil {
		fmt.Fprintln(a.Stderr, "wheretoken: download failed: "+err.Error())
		return ExitFail
	}
	sums, err := a.httpGet(base + "/checksums.txt")
	if err != nil {
		fmt.Fprintln(a.Stderr, "wheretoken: no checksums.txt; refusing to install")
		return ExitFail
	}
	if err := verifyChecksum(sums, asset, raw); err != nil {
		fmt.Fprintln(a.Stderr, err.Error())
		return ExitFail
	}
	bin, err := extractBinary(raw, asset, a.GOOS)
	if err != nil {
		fmt.Fprintln(a.Stderr, err.Error())
		return ExitFail
	}
	if err := replaceFile(exe, bin); err != nil {
		fmt.Fprintln(a.Stderr, err.Error())
		return ExitFail
	}
	fmt.Fprintln(a.Stderr, "wheretoken: updated "+exe)
	return ExitOK
}

func (a *App) runUninstall(quiet bool) int {
	exe, err := a.exePath()
	if err != nil {
		fmt.Fprintln(a.Stderr, err.Error())
		return ExitFail
	}
	if brewManaged(exe) {
		if !quiet {
			fmt.Fprintln(a.Stderr, "Homebrew install; running brew uninstall wheretoken")
		}
		if err := a.runCmd("brew", "uninstall", "wheretoken"); err != nil {
			fmt.Fprintln(a.Stderr, err.Error())
			return ExitFail
		}
		return ExitOK
	}
	if err := removeBinary(exe); err != nil {
		fmt.Fprintln(a.Stderr, err.Error())
		return ExitFail
	}
	fmt.Fprintln(a.Stderr, "wheretoken: removed "+exe)
	return ExitOK
}

func (a *App) exePath() (string, error) {
	if a.Executable != nil {
		return a.Executable()
	}
	p, err := os.Executable()
	if err != nil {
		return "", err
	}
	if real, err := filepath.EvalSymlinks(p); err == nil {
		return real, nil
	}
	return p, nil
}

func (a *App) runCmd(name string, args ...string) error {
	if a.RunCmd != nil {
		return a.RunCmd(name, args...)
	}
	cmd := exec.Command(name, args...)
	cmd.Stdout = a.Stdout
	cmd.Stderr = a.Stderr
	return cmd.Run()
}

func (a *App) httpGet(url string) ([]byte, error) {
	if a.HTTPGet != nil {
		return a.HTTPGet(url)
	}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", releaseUserAgent)
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, maxReleaseBody))
}

func (a *App) releaseBase() string {
	env := a.LookupEnv
	if env == nil {
		env = os.Getenv
	}
	if v := strings.TrimSpace(env("WHERETOKEN_RELEASE_URL")); v != "" {
		return strings.TrimRight(v, "/")
	}
	repo := strings.TrimSpace(env("WHERETOKEN_REPO"))
	if repo == "" {
		repo = defaultRepo
	}
	if ver := strings.TrimPrefix(strings.TrimSpace(env("WHERETOKEN_VERSION")), "v"); ver != "" {
		return "https://github.com/" + repo + "/releases/download/v" + ver
	}
	return "https://github.com/" + repo + "/releases/latest/download"
}

func releaseAssetName(goos, arch string) string {
	if goos == "" {
		goos = runtime.GOOS
	}
	if arch == "" {
		arch = runtime.GOARCH
	}
	switch arch {
	case "x86_64":
		arch = "amd64"
	case "aarch64":
		arch = "arm64"
	}
	if goos == "windows" {
		return "wheretoken_windows_" + arch + ".zip"
	}
	return "wheretoken_" + goos + "_" + arch + ".tar.gz"
}

func brewManaged(exe string) bool {
	slash := filepath.ToSlash(exe)
	return strings.Contains(slash, "/Cellar/wheretoken/")
}

func verifyChecksum(sums []byte, asset string, raw []byte) error {
	want := checksumFor(sums, asset)
	if want == "" {
		return fmt.Errorf("wheretoken: checksums.txt has no %s", asset)
	}
	sum := sha256.Sum256(raw)
	got := hex.EncodeToString(sum[:])
	if !strings.EqualFold(got, want) {
		return fmt.Errorf("wheretoken: checksum mismatch for %s", asset)
	}
	return nil
}

func checksumFor(sums []byte, asset string) string {
	for _, line := range strings.Split(string(sums), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		name := fields[len(fields)-1]
		name = strings.TrimPrefix(name, "*")
		if filepath.Base(name) == asset {
			return fields[0]
		}
	}
	return ""
}

func extractBinary(raw []byte, asset, goos string) ([]byte, error) {
	if strings.HasSuffix(asset, ".zip") {
		return unzipBinary(raw, goos)
	}
	return untarBinary(raw, goos)
}

func untarBinary(raw []byte, goos string) ([]byte, error) {
	gr, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	defer gr.Close()
	tr := tar.NewReader(gr)
	want := "wheretoken"
	if goos == "windows" {
		want = "wheretoken.exe"
	}
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if h.FileInfo().IsDir() {
			continue
		}
		if filepath.Base(h.Name) != want {
			continue
		}
		return io.ReadAll(io.LimitReader(tr, maxReleaseBody))
	}
	return nil, fmt.Errorf("wheretoken: archive had no binary")
}

func unzipBinary(raw []byte, goos string) ([]byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return nil, err
	}
	want := "wheretoken"
	if goos == "windows" {
		want = "wheretoken.exe"
	}
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		if filepath.Base(f.Name) != want {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		b, err := io.ReadAll(io.LimitReader(rc, maxReleaseBody))
		rc.Close()
		return b, err
	}
	return nil, fmt.Errorf("wheretoken: archive had no binary")
}

func replaceFile(dest string, payload []byte) error {
	dir := filepath.Dir(dest)
	tmp, err := os.CreateTemp(dir, ".wheretoken-new-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	ok := false
	defer func() {
		if !ok {
			os.Remove(tmpName)
		}
	}()
	if _, err := tmp.Write(payload); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o755); err != nil && runtime.GOOS != "windows" {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	old := dest + ".old"
	_ = os.Remove(old)
	if err := os.Rename(dest, old); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := os.Rename(tmpName, dest); err != nil {
		_ = os.Rename(old, dest)
		return err
	}
	ok = true
	if err := os.Remove(old); err != nil && !os.IsNotExist(err) && runtime.GOOS == "windows" {
		scheduleWindowsDelete(old)
	}
	return nil
}

func removeBinary(exe string) error {
	if err := os.Remove(exe); err == nil || os.IsNotExist(err) {
		return nil
	} else if runtime.GOOS != "windows" {
		return err
	}
	old := exe + ".old"
	_ = os.Remove(old)
	if err := os.Rename(exe, old); err != nil {
		return err
	}
	if err := os.Remove(old); err == nil || os.IsNotExist(err) {
		return nil
	}
	scheduleWindowsDelete(old)
	return nil
}

func scheduleWindowsDelete(path string) {
	cmd := exec.Command("cmd", "/C", "ping 127.0.0.1 -n 2 >nul & del /f /q "+strconvQuote(path))
	cmd.Stdout = nil
	cmd.Stderr = nil
	_ = cmd.Start()
}

func strconvQuote(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}
