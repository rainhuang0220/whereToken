package cli

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

func TestInstallScriptMentionsReleaseAssets(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	body, err := os.ReadFile(filepath.Join(filepath.Dir(file), "..", "..", "scripts", "install.sh"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(body)
	for _, want := range []string{
		"wheretoken_${os}_${arch}.tar.gz",
		"github.com/rainhuang0220/whereToken",
		"darwin", "linux", "amd64", "arm64",
		"checksums.txt", "sha256",
		"WHERETOKEN_RELEASE_URL",
		"releases/latest/download",
		"export PATH=",
		"$HOME/.local/bin",
		"/usr/local/bin",
		"${BIN_DIR}/wheretoken",
		".zshrc",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("install.sh missing %q", want)
		}
	}
	if strings.Contains(s, "GOPATH") || strings.Contains(s, "go env GOPATH") {
		t.Fatal("install.sh must not print a GOPATH lecture")
	}
	if strings.Contains(s, "eyJ") {
		t.Fatal("install.sh must not contain JWT material")
	}
	if strings.Contains(s, "next: wheretoken") {
		t.Fatal("install.sh must not tell this shell to type a bare wheretoken")
	}
}

func TestInstallPS1MentionsWindowsZip(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	body, err := os.ReadFile(filepath.Join(filepath.Dir(file), "..", "..", "scripts", "install.ps1"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(body)
	for _, want := range []string{
		"wheretoken_windows_${goarch}.zip",
		"wheretoken.exe",
		"github.com/rainhuang0220/whereToken",
		"amd64", "arm64",
		"checksums.txt",
		"SHA256",
		"releases/latest/download",
		"SetEnvironmentVariable",
		"wheretoken.exe",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("install.ps1 missing %q", want)
		}
	}
	if strings.Contains(s, "GOPATH") {
		t.Fatal("install.ps1 must not print a GOPATH lecture")
	}
	if strings.Contains(s, "eyJ") {
		t.Fatal("install.ps1 must not contain JWT material")
	}
}

func TestInstallCMDUsesCurlTarCertutil(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	body, err := os.ReadFile(filepath.Join(filepath.Dir(file), "..", "..", "scripts", "install.cmd"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(body)
	for _, want := range []string{
		"curl.exe",
		"tar.exe",
		"certutil",
		"checksums.txt",
		"wheretoken_windows_%GOARCH%.zip",
		"wheretoken.exe",
		"LOCALAPPDATA",
		"update",
		"uninstall",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("install.cmd missing %q", want)
		}
	}
	if strings.Contains(s, "eyJ") {
		t.Fatal("install.cmd must not contain JWT material")
	}
}

func TestGoreleaserShipsManCompletionsAndLicense(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	body, err := os.ReadFile(filepath.Join(filepath.Dir(file), "..", "..", ".goreleaser.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(body)
	for _, want := range []string{"LICENSE", "README.md", "CHANGELOG.md", "docs/wheretoken.1", "docs/cli-json.schema.json", "completions/*", "nfpms:", "deb", "rpm", `isEnvSet "MACOS_SIGN_P12"`, "MACOS_NOTARY_KEY", "MACOS_NOTARY_KEY_ID", "MACOS_NOTARY_ISSUER_ID"} {
		if !strings.Contains(s, want) {
			t.Errorf("goreleaser missing %q", want)
		}
	}
	i := strings.Index(s, "archives:")
	j := strings.Index(s, "checksum:")
	if i < 0 || j <= i {
		t.Fatal("goreleaser archives block missing")
	}
	if !strings.Contains(s[i:j], "docs/wheretoken.1") {
		t.Fatal("GitHub Release tarball must include the man page, not only the .deb/.rpm")
	}
}

func TestGitHubWorkflowsPinActionSHAs(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	root := filepath.Join(filepath.Dir(file), "..", "..", "ci", "github-workflows")
	sha := regexp.MustCompile(`^[0-9a-f]{40}$`)
	uses := regexp.MustCompile(`uses:\s+(\S+)`)
	for _, name := range []string{"ci.yml", "release.yml"} {
		body, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range uses.FindAllStringSubmatch(string(body), -1) {
			ref := m[1]
			_, at, ok := strings.Cut(ref, "@")
			if !ok {
				t.Errorf("%s: uses %s has no ref", name, ref)
				continue
			}
			if !sha.MatchString(at) {
				t.Errorf("%s: unpinned action %s", name, ref)
			}
		}
	}
}

func TestCIRunsGovulncheck(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	root := filepath.Join(filepath.Dir(file), "..", "..")
	for _, rel := range []string{
		filepath.Join(".github", "workflows", "ci.yml"),
		filepath.Join("ci", "github-workflows", "ci.yml"),
	} {
		body, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatal(err)
		}
		s := string(body)
		if !strings.Contains(s, "scripts/govulncheck.sh") {
			t.Fatalf("%s must run scripts/govulncheck.sh", rel)
		}
		if !strings.Contains(s, "go vet ./...") {
			t.Fatalf("%s must run go vet like make ci", rel)
		}
	}
}

func TestGitHubActionsWorkflowsAreInstalled(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	root := filepath.Join(filepath.Dir(file), "..", "..")
	for _, name := range []string{"ci.yml", "release.yml"} {
		want, err := os.ReadFile(filepath.Join(root, "ci", "github-workflows", name))
		if err != nil {
			t.Fatal(err)
		}
		got, err := os.ReadFile(filepath.Join(root, ".github", "workflows", name))
		if err != nil {
			t.Fatalf("GitHub Actions only runs files under .github/workflows (%s): %v", name, err)
		}
		if string(got) != string(want) {
			t.Fatalf(".github/workflows/%s drifted from ci/github-workflows/%s", name, name)
		}
	}
}

func TestCIRunsGofmt(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	body, err := os.ReadFile(filepath.Join(filepath.Dir(file), "..", "..", "ci", "github-workflows", "ci.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "gofmt -l") {
		t.Fatal("ci.yml should fail the build when gofmt -l is non-empty")
	}
}

func TestInstallDocsDoNotClaimLiveNpmPackage(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	root := filepath.Join(filepath.Dir(file), "..", "..")
	for _, rel := range []string{"README.md", "scripts/install.sh", "scripts/install.ps1", "docs/wheretoken.1"} {
		body, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatal(err)
		}
		s := string(body)
		if strings.Contains(s, "npm install -g wheretoken") || strings.Contains(s, "npx wheretoken") {
			t.Errorf("%s advertises unpublished npm package", rel)
		}
	}
	readme, err := os.ReadFile(filepath.Join(root, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(readme), "not on the npm registry") {
		t.Fatal("README should say the npm package is not on the registry yet")
	}
	curl := "curl -fsSL https://raw.githubusercontent.com/rainhuang0220/whereToken/main/scripts/install.sh | bash"
	irm := "irm https://raw.githubusercontent.com/rainhuang0220/whereToken/main/scripts/install.ps1 | iex"
	goInstall := "go install github.com/rainhuang0220/whereToken/cmd/wheretoken@latest"
	rs := string(readme)
	if !strings.Contains(rs, curl) {
		t.Fatal("README must show the curl | bash one-liner")
	}
	if !strings.Contains(rs, irm) {
		t.Fatal("README must show the PowerShell irm | iex one-liner")
	}
	cmd := `curl.exe -fsSL -o %TEMP%\wt-install.cmd https://raw.githubusercontent.com/rainhuang0220/whereToken/main/scripts/install.cmd && %TEMP%\wt-install.cmd`
	if !strings.Contains(rs, cmd) {
		t.Fatal("README must show the Command Prompt curl.exe install.cmd one-liner")
	}
	ci := strings.Index(rs, curl)
	gi := strings.Index(rs, goInstall)
	if gi >= 0 && ci > gi {
		t.Fatal("README should lead with curl | bash, then go install as an alternative")
	}
	if strings.Contains(rs, "GOPATH") {
		t.Fatal("README must not lecture GOPATH; binary install is the default")
	}
	if strings.Contains(rs, "Pushing those files needs a GitHub token") {
		t.Fatal("README must not tell strangers that Actions is blocked on workflow scope")
	}
	for _, want := range []string{"unsigned", "brew tap rainhuang0220/wheretoken", "not on the npm registry", "signed in"} {
		if !strings.Contains(strings.ToLower(rs), strings.ToLower(want)) && !strings.Contains(rs, want) {
			t.Errorf("README missing honest not-yet %q", want)
		}
	}
	if !strings.Contains(rs, "unsigned") {
		t.Fatal("README should say GitHub binaries are unsigned")
	}
	if !strings.Contains(rs, "brew tap rainhuang0220/wheretoken") || !strings.Contains(rs, "brew install wheretoken") {
		t.Fatal("README should show brew tap rainhuang0220/wheretoken then brew install wheretoken")
	}
	if strings.Contains(rs, "no Homebrew tap") || strings.Contains(rs, "There is **no Homebrew tap**") {
		t.Fatal("README must not claim there is no Homebrew tap after the tap exists")
	}
	if !strings.Contains(rs, "signed in") && !strings.Contains(rs, "已登录") {
		t.Fatal("README should say Trae/Cursor token columns need those apps signed in")
	}
}

func TestHomebrewFormulaIsHeadBuild(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	body, err := os.ReadFile(filepath.Join(filepath.Dir(file), "..", "..", "Formula", "wheretoken.rb"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(body)
	for _, want := range []string{`class Wheretoken`, `head "https://github.com/rainhuang0220/whereToken.git"`, "./cmd/wheretoken", "docs/wheretoken.1", "bash_completion", "zsh_completion", "fish_completion"} {
		if !strings.Contains(s, want) {
			t.Errorf("formula missing %q", want)
		}
	}
	if !strings.Contains(s, "sha256") || !strings.Contains(s, "archive/refs/tags/v") {
		t.Fatal("formula should pin a GitHub tag tarball SHA256 so brew install ./Formula works without --HEAD")
	}
}

func TestManPageMentionsJSONSchema(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	body, err := os.ReadFile(filepath.Join(filepath.Dir(file), "..", "..", "docs", "wheretoken.1"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(body)
	for _, want := range []string{"cli-json.schema.json", "JWT", "127.0.0.1", "--offline", "--width", "--version", "install.sh", "go install", "--port", "--today --kimi", "unsigned", "brew tap rainhuang0220/wheretoken", "COLUMNS", "FORCE_COLOR", "刷新", "xai", "Community Rank", "DO_NOT_TRACK", "Does not upload", "估价", "uploaded"} {
		if !strings.Contains(s, want) {
			t.Errorf("man page missing %q", want)
		}
	}
	if strings.Contains(s, "GOPATH") {
		t.Fatal("man page must not lecture GOPATH")
	}
}

func TestCommunityFileDocsNameWindowsLayout(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	root := filepath.Join(filepath.Dir(file), "..", "..")
	for _, rel := range []string{"docs/community.md", "docs/wheretoken.1"} {
		body, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatal(err)
		}
		s := string(body)
		if !strings.Contains(s, "~/.config/wheretoken") {
			t.Errorf("%s must name the Unix community.json directory", rel)
		}
		if !strings.Contains(s, "%APPDATA%") {
			t.Errorf("%s must name the Windows community.json directory", rel)
		}
	}
}

func TestReleaseWorkflowInstallsNodeBeforeGoreleaser(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	body, err := os.ReadFile(filepath.Join(filepath.Dir(file), "..", "..", "ci", "github-workflows", "release.yml"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(body)
	if !strings.Contains(s, "setup-node") {
		t.Fatal("release.yml must setup Node so goreleaser can build web/")
	}
	if !strings.Contains(s, "49933ea5288caeca8642d1e84afbd3f7d6820020") {
		t.Fatal("release.yml should pin setup-node to the same SHA as ci.yml")
	}
	for _, want := range []string{
		"secrets.MACOS_SIGN_P12",
		"secrets.MACOS_SIGN_PASSWORD",
		"secrets.MACOS_NOTARY_KEY",
		"secrets.MACOS_NOTARY_KEY_ID",
		"secrets.MACOS_NOTARY_ISSUER_ID",
		"publishing unsigned binaries",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("release.yml missing skip-if-missing signing %q", want)
		}
	}
}

func TestMacOSSigningDocListsSecretNamesOnly(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	body, err := os.ReadFile(filepath.Join(filepath.Dir(file), "..", "..", "docs", "macos-signing.md"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(body)
	for _, want := range []string{
		"MACOS_SIGN_P12",
		"MACOS_SIGN_PASSWORD",
		"MACOS_NOTARY_KEY",
		"MACOS_NOTARY_KEY_ID",
		"MACOS_NOTARY_ISSUER_ID",
		"Never paste",
		"skips signing",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("macos-signing.md missing %q", want)
		}
	}
	if strings.Contains(s, "BEGIN CERTIFICATE") || strings.Contains(s, "-----BEGIN") {
		t.Fatal("macos-signing.md must not contain certificate material")
	}
}

func TestPackageReleaseScriptNamesChecksums(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	body, err := os.ReadFile(filepath.Join(filepath.Dir(file), "..", "..", "scripts", "package-release.sh"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(body)
	for _, want := range []string{
		"wheretoken_${goos}_${goarch}.tar.gz",
		"wheretoken_windows_${goarch}.zip",
		"checksums.txt",
		"main.version",
		"darwin", "linux", "windows", "amd64", "arm64",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("package-release.sh missing %q", want)
		}
	}
}

func TestInstallShDownloadsVerifiedRelease(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("bash installer")
	}
	if _, err := exec.LookPath("curl"); err != nil {
		t.Skip("curl")
	}
	if _, err := exec.LookPath("tar"); err != nil {
		t.Skip("tar")
	}

	osName := runtime.GOOS
	arch := runtime.GOARCH
	asset := fmt.Sprintf("wheretoken_%s_%s.tar.gz", osName, arch)

	payload := []byte("#!/bin/sh\necho wheretoken 0.1.0\n")
	var tarbuf bytes.Buffer
	gz := gzip.NewWriter(&tarbuf)
	tw := tar.NewWriter(gz)
	hdr := &tar.Header{Name: "wheretoken", Mode: 0755, Size: int64(len(payload))}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(payload); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	archive := tarbuf.Bytes()
	sum := sha256.Sum256(archive)
	checksums := fmt.Sprintf("%x  %s\n", sum, asset)

	mux := http.NewServeMux()
	mux.HandleFunc("/"+asset, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(archive)
	})
	mux.HandleFunc("/checksums.txt", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, checksums)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	script := filepath.Join(filepath.Dir(file), "..", "..", "scripts", "install.sh")
	prefix := t.TempDir()
	home := t.TempDir()
	cmd := exec.Command("bash", script)
	cmd.Env = append(os.Environ(),
		"PREFIX="+prefix,
		"HOME="+home,
		"WHERETOKEN_RELEASE_URL="+srv.URL,
		"PATH=/usr/bin:/bin:/usr/sbin:/sbin",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("install.sh: %v\n%s", err, out)
	}
	got := string(out)
	if strings.Contains(got, "GOPATH") {
		t.Fatalf("GOPATH lecture:\n%s", got)
	}
	bin := filepath.Join(prefix, "bin", "wheretoken")
	st, err := os.Stat(bin)
	if err != nil {
		t.Fatalf("binary missing: %v\n%s", err, got)
	}
	if st.Mode()&0o111 == 0 {
		t.Fatalf("binary not executable: %v", st.Mode())
	}
	if !strings.Contains(got, bin) {
		t.Fatalf("should print the one command that runs the binary:\n%s", got)
	}
	if strings.Contains(got, "next: wheretoken") {
		t.Fatalf("do not ask for a second PATH step:\n%s", got)
	}
	rc, err := os.ReadFile(filepath.Join(home, ".zshrc"))
	if err != nil {
		t.Fatalf("expected PATH written to fake HOME/.zshrc: %v\n%s", err, got)
	}
	if !strings.Contains(string(rc), filepath.Join(prefix, "bin")) {
		t.Fatalf("zshrc missing bindir:\n%s", rc)
	}
}

func TestInstallShRejectsChecksumMismatch(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("bash installer")
	}
	if _, err := exec.LookPath("curl"); err != nil {
		t.Skip("curl")
	}

	osName := runtime.GOOS
	arch := runtime.GOARCH
	asset := fmt.Sprintf("wheretoken_%s_%s.tar.gz", osName, arch)
	archive := []byte("not-a-real-tarball")
	checksums := fmt.Sprintf("%x  %s\n", sha256.Sum256([]byte("other")), asset)

	mux := http.NewServeMux()
	mux.HandleFunc("/"+asset, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(archive)
	})
	mux.HandleFunc("/checksums.txt", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, checksums)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	script := filepath.Join(filepath.Dir(file), "..", "..", "scripts", "install.sh")
	cmd := exec.Command("bash", script)
	cmd.Env = append(os.Environ(),
		"PREFIX="+t.TempDir(),
		"WHERETOKEN_RELEASE_URL="+srv.URL,
		"PATH=/usr/bin:/bin:/usr/sbin:/sbin",
	)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected checksum failure\n%s", out)
	}
	if strings.Contains(string(out), "GOPATH") {
		t.Fatalf("GOPATH lecture on checksum failure:\n%s", out)
	}
}
