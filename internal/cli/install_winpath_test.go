package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"unicode/utf16"
)

// windowsUserPathMu serializes tests that write HKCU User Path.
var windowsUserPathMu sync.Mutex

func TestInstallCMDSameShellFindsBinary(t *testing.T) {
	fx := newWindowsInstallFixture(t, windowsInstallFixtureOpts{})
	out := fx.runCMD(t, cmdSameShellProbe(fx.binDir))
	assertSameShellSuccess(t, out, fx.binDir)
}

func TestInstallPS1SameShellFindsBinary(t *testing.T) {
	fx := newWindowsInstallFixture(t, windowsInstallFixtureOpts{})
	out := fx.runPS1(t, psSameShellProbe(fx.binDir))
	assertSameShellSuccess(t, out, fx.binDir)
}

func TestInstallCMDIdempotentUserPath(t *testing.T) {
	fx := newWindowsInstallFixture(t, windowsInstallFixtureOpts{})
	first := fx.runCMD(t, cmdSameShellProbe(fx.binDir))
	assertSameShellSuccess(t, first, fx.binDir)
	second := fx.runCMD(t, cmdSameShellProbe(fx.binDir))
	assertSameShellSuccess(t, second, fx.binDir)
	if n := countPathEntries(fx.userPath(t), fx.binDir); n != 1 {
		t.Fatalf("User Path has %d copies of %s", n, fx.binDir)
	}
}

func TestInstallPS1IdempotentUserPath(t *testing.T) {
	fx := newWindowsInstallFixture(t, windowsInstallFixtureOpts{})
	first := fx.runPS1(t, psSameShellProbe(fx.binDir))
	assertSameShellSuccess(t, first, fx.binDir)
	second := fx.runPS1(t, psSameShellProbe(fx.binDir))
	assertSameShellSuccess(t, second, fx.binDir)
	if n := countPathEntries(fx.userPath(t), fx.binDir); n != 1 {
		t.Fatalf("User Path has %d copies of %s", n, fx.binDir)
	}
}

func TestInstallCMDRejectsChecksumMismatch(t *testing.T) {
	fx := newWindowsInstallFixture(t, windowsInstallFixtureOpts{badChecksum: true})
	out, err := fx.runCMDAllowFail(t, cmdSameShellProbe(fx.binDir))
	if err == nil {
		t.Fatalf("expected checksum failure\n%s", out)
	}
	if _, statErr := os.Stat(filepath.Join(fx.binDir, "wheretoken.exe")); !os.IsNotExist(statErr) {
		t.Fatalf("checksum mismatch must not leave a binary: %v\n%s", statErr, out)
	}
	if !strings.Contains(out, "SHA256 mismatch") && !strings.Contains(out, "checksum") {
		t.Fatalf("expected checksum error\n%s", out)
	}
}

func TestInstallPS1RejectsChecksumMismatch(t *testing.T) {
	fx := newWindowsInstallFixture(t, windowsInstallFixtureOpts{badChecksum: true})
	out, err := fx.runPS1AllowFail(t, psSameShellProbe(fx.binDir))
	if err == nil {
		t.Fatalf("expected checksum failure\n%s", out)
	}
	if _, statErr := os.Stat(filepath.Join(fx.binDir, "wheretoken.exe")); !os.IsNotExist(statErr) {
		t.Fatalf("checksum mismatch must not leave a binary: %v\n%s", statErr, out)
	}
	if !strings.Contains(strings.ToLower(out), "sha256") && !strings.Contains(strings.ToLower(out), "checksum") {
		t.Fatalf("expected checksum error\n%s", out)
	}
}

func TestInstallCMDUnsupportedArch(t *testing.T) {
	fx := newWindowsInstallFixture(t, windowsInstallFixtureOpts{})
	script := filepath.Join(fx.root, "scripts", "install.cmd")
	wrapper := filepath.Join(t.TempDir(), "arch.cmd")
	body := fmt.Sprintf("@echo off\r\nset PROCESSOR_ARCHITEW6432=\r\nset PROCESSOR_ARCHITECTURE=IA64\r\ncall \"%s\"\r\n", script)
	if err := os.WriteFile(wrapper, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := runCmdFile(fx.cmdEnv(), wrapper)
	if err == nil {
		t.Fatalf("expected unsupported arch\n%s", out)
	}
	if !strings.Contains(out, "unsupported") || !strings.Contains(out, "IA64") {
		t.Fatalf("expected a clear unsupported-arch error\n%s", out)
	}
	if _, statErr := os.Stat(filepath.Join(fx.binDir, "wheretoken.exe")); !os.IsNotExist(statErr) {
		t.Fatalf("unsupported arch must not install a binary\n%s", out)
	}
}

func TestInstallPS1UnsupportedArch(t *testing.T) {
	fx := newWindowsInstallFixture(t, windowsInstallFixtureOpts{})
	script := filepath.Join(fx.root, "scripts", "install.ps1")
	ps := fmt.Sprintf(`$ErrorActionPreference = 'Stop'
Remove-Item Env:PROCESSOR_ARCHITEW6432 -ErrorAction SilentlyContinue
$env:PROCESSOR_ARCHITECTURE = 'IA64'
Get-Content -Raw -LiteralPath '%s' | Invoke-Expression
`, powershellSingleQuote(script))
	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", ps)
	cmd.Env = fx.psEnv()
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected unsupported arch\n%s", out)
	}
	got := string(out)
	if !strings.Contains(got, "unsupported") || !strings.Contains(got, "IA64") {
		t.Fatalf("expected a clear unsupported-arch error\n%s", got)
	}
}

func TestInstallCMDMissingAsset(t *testing.T) {
	fx := newWindowsInstallFixture(t, windowsInstallFixtureOpts{omitAsset: true})
	out, err := fx.runCMDAllowFail(t, cmdSameShellProbe(fx.binDir))
	if err == nil {
		t.Fatalf("expected missing-asset failure\n%s", out)
	}
	if !strings.Contains(out, "download failed") && !strings.Contains(out, fx.asset) {
		t.Fatalf("expected download/asset error naming the URL or zip\n%s", out)
	}
}

func TestInstallPS1MissingAsset(t *testing.T) {
	fx := newWindowsInstallFixture(t, windowsInstallFixtureOpts{omitAsset: true})
	out, err := fx.runPS1AllowFail(t, psSameShellProbe(fx.binDir))
	if err == nil {
		t.Fatalf("expected missing-asset failure\n%s", out)
	}
	if !strings.Contains(out, "download failed") && !strings.Contains(out, fx.asset) {
		t.Fatalf("expected download/asset error naming the URL or zip\n%s", out)
	}
}

func TestInstallCMDPreservesBangInUserPath(t *testing.T) {
	testInstallCMDPreservesPathEntry(t, `C:\Tools!Special\bin`, `C:\Bang!Dir\bin`)
}

func TestInstallCMDPreservesAmpInUserPath(t *testing.T) {
	testInstallCMDPreservesPathEntry(t, `C:\Amp&Dir\bin`, `C:\Amp&Dir\bin`)
}

func testInstallCMDPreservesPathEntry(t *testing.T, userEntry, processEntry string) {
	t.Helper()
	fx := newWindowsInstallFixture(t, windowsInstallFixtureOpts{processPathPrefix: processEntry})
	fx.prependUserPath(t, userEntry)
	out := fx.runCMD(t, cmdSameShellProbe(fx.binDir))
	assertSameShellSuccess(t, out, fx.binDir)
	got := fx.userPath(t)
	if !pathHasExactEntry(got, userEntry) {
		t.Fatalf("User Path lost %q:\n%s", userEntry, got)
	}
	stripped := strings.ReplaceAll(userEntry, "!", "")
	if stripped != userEntry && strings.Contains(got, stripped) && !pathHasExactEntry(got, userEntry) {
		t.Fatalf("delayed expansion stripped ! from User Path:\n%s", got)
	}
	if n := countPathEntries(got, fx.binDir); n != 1 {
		t.Fatalf("User Path has %d copies of %s\n%s", n, fx.binDir, got)
	}
	if strings.Contains(out, "is not recognized") {
		t.Fatalf("installer executed PATH content:\n%s", out)
	}
}

func TestCIWindowsInstallerSmokeStaysOneShell(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	body, err := os.ReadFile(filepath.Join(filepath.Dir(file), "..", "..", "ci", "github-workflows", "ci.yml"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(body)
	if !strings.Contains(s, "Windows installer same-shell smoke") {
		t.Fatal("ci.yml must have a named Windows installer same-shell smoke step")
	}
	if !strings.Contains(s, `if: runner.os == 'Windows'`) {
		t.Fatal("installer smoke must run on windows-latest")
	}
	if !strings.Contains(s, `-run "TestInstallCMD|TestInstallPS1"`) {
		t.Fatal("installer smoke must invoke the same-shell Go tests, not split install and where across GHA steps")
	}
}

type windowsInstallFixtureOpts struct {
	badChecksum       bool
	omitAsset         bool
	processPathPrefix string
}

type windowsInstallFixture struct {
	root              string
	binDir            string
	asset             string
	processPathPrefix string
	restore           func()
}

func newWindowsInstallFixture(t *testing.T, opts windowsInstallFixtureOpts) *windowsInstallFixture {
	t.Helper()
	if runtime.GOOS != "windows" {
		t.Skip("windows installer")
	}
	windowsUserPathMu.Lock()
	t.Cleanup(windowsUserPathMu.Unlock)

	needWindowsTools(t)
	restore := snapshotUserPath(t)
	t.Cleanup(restore)

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	root := filepath.Join(filepath.Dir(file), "..", "..")
	binDir := filepath.Join(t.TempDir(), "Some User", "whereToken", "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}

	stub := buildWhereTokenStub(t)
	payload, err := os.ReadFile(stub)
	if err != nil {
		t.Fatal(err)
	}
	archive := zipNamed(t, "wheretoken.exe", payload)
	sum := sha256.Sum256(archive)
	want := hex.EncodeToString(sum[:])
	if opts.badChecksum {
		want = strings.Repeat("ab", 32)
	}
	asset := "wheretoken_windows_amd64.zip"
	if arch := windowsTestArch(); arch != "" {
		asset = "wheretoken_windows_" + arch + ".zip"
	}
	checksums := want + "  " + asset + "\n"

	mux := http.NewServeMux()
	if !opts.omitAsset {
		mux.HandleFunc("/"+asset, func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write(archive)
		})
	}
	mux.HandleFunc("/checksums.txt", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, checksums)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	fx := &windowsInstallFixture{
		root:              root,
		binDir:            binDir,
		asset:             asset,
		processPathPrefix: opts.processPathPrefix,
		restore:           restore,
	}
	t.Setenv("WHERETOKEN_RELEASE_URL", srv.URL)
	t.Setenv("BIN_DIR", binDir)
	t.Setenv("PREFIX", filepath.Dir(binDir))
	return fx
}

func (fx *windowsInstallFixture) cmdEnv(extra ...string) []string {
	env := os.Environ()
	env = append(env,
		"WHERETOKEN_RELEASE_URL="+os.Getenv("WHERETOKEN_RELEASE_URL"),
		"BIN_DIR="+fx.binDir,
		"PREFIX="+filepath.Dir(fx.binDir),
	)
	if fx.processPathPrefix != "" {
		env = append(env, "PATH="+fx.processPathPrefix+";"+os.Getenv("PATH"))
	}
	env = append(env, extra...)
	return env
}

func (fx *windowsInstallFixture) prependUserPath(t *testing.T, entry string) {
	t.Helper()
	value, typ, exists := readUserPath(t)
	if typ == "" {
		typ = "REG_EXPAND_SZ"
	}
	next := entry
	if exists && strings.TrimSpace(value) != "" {
		next = entry + ";" + value
	}
	cmd := exec.Command("reg.exe", "add", `HKCU\Environment`, "/v", "Path", "/t", typ, "/d", next, "/f")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("prepend User Path %q: %v\n%s", entry, err, out)
	}
}

func pathHasExactEntry(path, dir string) bool {
	return countPathEntries(path, dir) == 1
}

func (fx *windowsInstallFixture) psEnv(extra ...string) []string {
	return fx.cmdEnv(extra...)
}

func (fx *windowsInstallFixture) runCMD(t *testing.T, probe string) string {
	t.Helper()
	out, err := fx.runCMDAllowFail(t, probe)
	if err != nil {
		t.Fatalf("install.cmd same-shell: %v\n%s", err, out)
	}
	return out
}

func (fx *windowsInstallFixture) runCMDAllowFail(t *testing.T, probe string) (string, error) {
	t.Helper()
	script := filepath.Join(fx.root, "scripts", "install.cmd")
	raw, err := os.ReadFile(script)
	if err != nil {
		t.Fatal(err)
	}
	spacedTemp := filepath.Join(t.TempDir(), "Some User", "Temp")
	if err := os.MkdirAll(spacedTemp, 0o755); err != nil {
		t.Fatal(err)
	}
	downloaded := filepath.Join(spacedTemp, "wt-install.cmd")
	if err := os.WriteFile(downloaded, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	wrapper := filepath.Join(t.TempDir(), "smoke.cmd")
	body := fmt.Sprintf("@echo off\r\ncall \"%s\"\r\nif errorlevel 1 exit /b 1\r\n%s\r\n", downloaded, probe)
	if err := os.WriteFile(wrapper, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return runCmdFile(fx.cmdEnv("TEMP="+spacedTemp, "TMP="+spacedTemp), wrapper)
}

func runCmdFile(env []string, bat string) (string, error) {
	cmd := exec.Command("cmd.exe", "/D", "/E:ON", "/V:OFF", "/C", bat)
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func (fx *windowsInstallFixture) runPS1(t *testing.T, probe string) string {
	t.Helper()
	out, err := fx.runPS1AllowFail(t, probe)
	if err != nil {
		t.Fatalf("install.ps1 same-shell: %v\n%s", err, out)
	}
	return out
}

func (fx *windowsInstallFixture) runPS1AllowFail(t *testing.T, probe string) (string, error) {
	t.Helper()
	script := filepath.Join(fx.root, "scripts", "install.ps1")
	ps := fmt.Sprintf(`$ErrorActionPreference = 'Stop'
Get-Content -Raw -LiteralPath '%s' | Invoke-Expression
%s
`, powershellSingleQuote(script), probe)
	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", ps)
	cmd.Env = fx.psEnv()
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func (fx *windowsInstallFixture) userPath(t *testing.T) string {
	t.Helper()
	value, _, _ := readUserPath(t)
	return value
}

func cmdSameShellProbe(binDir string) string {
	exe := filepath.Join(binDir, "wheretoken.exe")
	return fmt.Sprintf("where wheretoken\r\nif errorlevel 1 exit /b 1\r\nwheretoken --version\r\nif errorlevel 1 exit /b 1\r\nwheretoken\r\nif errorlevel 1 exit /b 1\r\nif not exist \"%s\" exit /b 1\r\n", exe)
}

func psSameShellProbe(binDir string) string {
	exe := filepath.Join(binDir, "wheretoken.exe")
	return fmt.Sprintf(`$cmd = Get-Command wheretoken -CommandType Application -ErrorAction Stop
Write-Output ("resolved=" + $cmd.Source)
if ($cmd.Source -ne '%s') { throw "Get-Command resolved $($cmd.Source)" }
wheretoken --version
if ($LASTEXITCODE -ne 0 -and $null -ne $LASTEXITCODE) { throw "wheretoken --version exit $LASTEXITCODE" }
wheretoken
if ($LASTEXITCODE -ne 0 -and $null -ne $LASTEXITCODE) { throw "wheretoken exit $LASTEXITCODE" }
if (-not (Test-Path -LiteralPath '%s')) { throw 'exe missing' }
`, powershellSingleQuote(exe), powershellSingleQuote(exe))
}

func assertSameShellSuccess(t *testing.T, out, binDir string) {
	t.Helper()
	exe := filepath.Join(binDir, "wheretoken.exe")
	if _, err := os.Stat(exe); err != nil {
		t.Fatalf("binary missing: %v\n%s", err, out)
	}
	if !strings.Contains(strings.ToLower(out), strings.ToLower(binDir)) {
		t.Fatalf("same-shell resolution did not mention bindir %s:\n%s", binDir, out)
	}
	if !strings.Contains(out, "0.0.0-test") {
		t.Fatalf("same-shell wheretoken --version missing:\n%s", out)
	}
	if !strings.Contains(out, "wheretoken test stub ok") {
		t.Fatalf("same-shell wheretoken missing:\n%s", out)
	}
}

func needWindowsTools(t *testing.T) {
	t.Helper()
	for _, name := range []string{"cmd.exe", "powershell.exe", "curl.exe", "tar.exe", "certutil.exe", "reg.exe"} {
		if _, err := exec.LookPath(name); err != nil {
			t.Skip(name)
		}
	}
}

func buildWhereTokenStub(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	src := filepath.Join(dir, "main.go")
	code := `package main
import (
	"fmt"
	"os"
)
func main() {
	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "-V") {
		fmt.Println("wheretoken 0.0.0-test")
		return
	}
	fmt.Println("wheretoken test stub ok")
}
`
	if err := os.WriteFile(src, []byte(code), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "wheretoken.exe")
	cmd := exec.Command("go", "build", "-o", out, src)
	cmd.Env = append(os.Environ(), "GOOS=windows")
	got, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go build stub: %v\n%s", err, got)
	}
	return out
}

func windowsTestArch() string {
	if v := os.Getenv("PROCESSOR_ARCHITEW6432"); v != "" {
		return windowsGoArch(v)
	}
	return windowsGoArch(os.Getenv("PROCESSOR_ARCHITECTURE"))
}

func windowsGoArch(arch string) string {
	switch strings.ToUpper(arch) {
	case "AMD64":
		return "amd64"
	case "ARM64":
		return "arm64"
	default:
		return "amd64"
	}
}

func countPathEntries(path, dir string) int {
	n := 0
	want := strings.TrimRight(strings.ToLower(filepath.Clean(dir)), `\`)
	for _, part := range strings.Split(path, ";") {
		got := strings.TrimRight(strings.ToLower(filepath.Clean(part)), `\`)
		if got == want {
			n++
		}
	}
	return n
}

func powershellSingleQuote(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}

func snapshotUserPath(t *testing.T) func() {
	t.Helper()
	value, typ, exists := readUserPath(t)
	return func() {
		if !exists {
			cmd := exec.Command("reg.exe", "delete", `HKCU\Environment`, "/v", "Path", "/f")
			_ = cmd.Run()
			return
		}
		if typ == "" {
			typ = "REG_EXPAND_SZ"
		}
		cmd := exec.Command("reg.exe", "add", `HKCU\Environment`, "/v", "Path", "/t", typ, "/d", value, "/f")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Errorf("restore User Path: %v\n%s", err, out)
		}
	}
}

func readUserPath(t *testing.T) (value, typ string, exists bool) {
	t.Helper()
	cmd := exec.Command("reg.exe", "query", `HKCU\Environment`, "/v", "Path")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", "", false
	}
	text := decodeRegOutput(out)
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 3 || !strings.EqualFold(fields[0], "Path") {
			continue
		}
		typ = fields[1]
		value = strings.TrimSpace(strings.TrimPrefix(line, fields[0]))
		value = strings.TrimSpace(strings.TrimPrefix(value, typ))
		return value, typ, true
	}
	t.Fatalf("reg query Path succeeded but could not parse:\n%s", text)
	return "", "", false
}

func decodeRegOutput(b []byte) string {
	if len(b) >= 2 && b[0] == 0xFF && b[1] == 0xFE {
		n := (len(b) - 2) / 2
		u16 := make([]uint16, n)
		for i := 0; i < n; i++ {
			u16[i] = uint16(b[2+2*i]) | uint16(b[3+2*i])<<8
		}
		return strings.TrimRight(string(utf16.Decode(u16)), "\x00")
	}
	return string(b)
}
