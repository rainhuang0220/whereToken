package cli

import (
	"errors"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/rainhuang0220/whereToken/internal/publicprofile"
)

const profileRefreshLabel = "io.github.rainhuang0220.wheretoken.profile-refresh"

func (a *App) userHome() (string, error) {
	if a.UserHome != nil {
		return a.UserHome()
	}
	return os.UserHomeDir()
}

func (a *App) euid() int {
	if a.Euid != nil {
		return a.Euid()
	}
	return os.Geteuid()
}

func (a *App) uid() int {
	if a.Uid != nil {
		return a.Uid()
	}
	return os.Getuid()
}

func (a *App) launchctl(args ...string) (string, error) {
	if a.RunCmdOutput != nil {
		return a.RunCmdOutput("launchctl", args...)
	}
	if a.RunCmd != nil {
		return "", a.RunCmd("launchctl", args...)
	}
	return a.execLaunchctl(args...)
}

func (a *App) profileRefreshPaths() (plistPath, logPath string, err error) {
	home, err := a.userHome()
	if err != nil || home == "" {
		return "", "", errors.New("profile refresh agent is incomplete")
	}
	plistPath = filepath.Join(home, "Library", "LaunchAgents", profileRefreshLabel+".plist")
	logPath = filepath.Join(home, "Library", "Logs", "wheretoken", "profile-refresh.log")
	return plistPath, logPath, nil
}

func (a *App) refreshProgramPath() (string, error) {
	exe, err := a.exePath()
	if err != nil || exe == "" {
		return "", errors.New("profile refresh agent is incomplete")
	}
	if !brewManaged(exe) {
		return exe, nil
	}
	home, err := a.userHome()
	var candidates []string
	if err == nil && home != "" {
		candidates = append(candidates, filepath.Join(home, ".local", "bin", "wheretoken"))
	}
	candidates = append(candidates, "/opt/homebrew/bin/wheretoken", "/usr/local/bin/wheretoken")
	return chooseRefreshProgram(exe, candidates)
}

// chooseRefreshProgram keeps a Homebrew Cellar inode out of the plist.
// candidates are the stable symlinks, in order.
func chooseRefreshProgram(resolved string, candidates []string) (string, error) {
	if !brewManaged(resolved) {
		if resolved == "" {
			return "", errors.New("profile refresh agent is incomplete")
		}
		return resolved, nil
	}
	for _, c := range candidates {
		fi, err := os.Lstat(c)
		if err != nil || fi.IsDir() {
			continue
		}
		return c, nil
	}
	return "", errors.New("Homebrew install has no stable wheretoken command on PATH")
}

// profileRefreshPlist is the user agent definition. It has no
// EnvironmentVariables, no StartInterval, and no --home.
func profileRefreshPlist(program, workDir, logPath string) ([]byte, error) {
	if program == "" || workDir == "" || logPath == "" {
		return nil, errors.New("profile refresh agent is incomplete")
	}
	esc := html.EscapeString
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	b.WriteString(`<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">` + "\n")
	b.WriteString(`<plist version="1.0">` + "\n<dict>\n")
	writePlistString(&b, "Label", profileRefreshLabel)
	b.WriteString("\t<key>ProgramArguments</key>\n\t<array>\n")
	for _, arg := range []string{program, "profile", "refresh", "watch", "--quiet"} {
		fmt.Fprintf(&b, "\t\t<string>%s</string>\n", esc(arg))
	}
	b.WriteString("\t</array>\n")
	b.WriteString("\t<key>RunAtLoad</key>\n\t<true/>\n")
	b.WriteString("\t<key>KeepAlive</key>\n\t<dict>\n")
	b.WriteString("\t\t<key>SuccessfulExit</key>\n\t\t<false/>\n")
	b.WriteString("\t</dict>\n")
	b.WriteString("\t<key>ThrottleInterval</key>\n\t<integer>60</integer>\n")
	b.WriteString("\t<key>ProcessType</key>\n\t<string>Background</string>\n")
	writePlistString(&b, "WorkingDirectory", workDir)
	writePlistString(&b, "StandardOutPath", logPath)
	writePlistString(&b, "StandardErrorPath", logPath)
	b.WriteString("</dict>\n</plist>\n")
	return []byte(b.String()), nil
}

func writePlistString(b *strings.Builder, key, value string) {
	fmt.Fprintf(b, "\t<key>%s</key>\n\t<string>%s</string>\n", html.EscapeString(key), html.EscapeString(value))
}

func (a *App) installProfileRefreshAgent(program string) error {
	plistPath, logPath, err := a.profileRefreshPaths()
	if err != nil {
		return err
	}
	home, err := a.userHome()
	if err != nil || home == "" {
		return errors.New("profile refresh agent is incomplete")
	}
	if err := os.MkdirAll(filepath.Dir(plistPath), 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		return err
	}
	body, err := profileRefreshPlist(program, home, logPath)
	if err != nil {
		return err
	}
	if err := writeAtomicFile(plistPath, body, 0o644); err != nil {
		return err
	}
	domain := fmt.Sprintf("gui/%d", a.uid())
	_, _ = a.launchctl("bootout", domain, plistPath)
	if _, err := a.launchctl("bootstrap", domain, plistPath); err != nil {
		return err
	}
	return nil
}

func (a *App) stopProfileRefreshAgent() {
	if a.GOOS != "darwin" {
		return
	}
	plistPath, _, err := a.profileRefreshPaths()
	if err != nil {
		return
	}
	if _, err := os.Stat(plistPath); err != nil {
		return
	}
	domain := fmt.Sprintf("gui/%d", a.uid())
	_, _ = a.launchctl("bootout", domain, plistPath)
	_ = os.Remove(plistPath)
}

func (a *App) rebootstrapProfileRefresh() {
	if a.GOOS != "darwin" {
		return
	}
	home := a.resolveHome("")
	on, err := publicprofile.LoadRefreshSwitch(publicprofile.RefreshConfigPath(home))
	if err != nil || !on {
		return
	}
	program, err := a.refreshProgramPath()
	if err != nil || program == "" {
		return
	}
	_ = a.installProfileRefreshAgent(program)
}

func (a *App) schedulerStatus(enabled bool) string {
	if a.GOOS != "darwin" {
		return "unsupported"
	}
	if !enabled {
		return "off"
	}
	out, err := a.launchctl("print", fmt.Sprintf("gui/%d/%s", a.uid(), profileRefreshLabel))
	if err == nil && launchctlHasPID(out) {
		return "running"
	}
	return "dead"
}

func launchctlHasPID(out string) bool {
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "pid =") {
			continue
		}
		rest := strings.TrimSpace(strings.TrimPrefix(line, "pid ="))
		n, err := strconv.Atoi(rest)
		if err == nil && n > 0 {
			return true
		}
	}
	return false
}

func (a *App) maybeReexecProfileWatch() {
	if a.GOOS != "darwin" {
		return
	}
	plistPath, _, err := a.profileRefreshPaths()
	if err != nil {
		return
	}
	program, err := plistProgram(plistPath)
	if err != nil || program == "" {
		return
	}
	self, err := a.exePath()
	if err != nil || self == "" {
		return
	}
	same, err := sameExecutable(program, self)
	if err != nil || same {
		return
	}
	argv := append([]string{program}, a.Args...)
	a.reexec(program, argv)
}

func plistProgram(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	text := string(raw)
	i := strings.Index(text, "<key>ProgramArguments</key>")
	if i < 0 {
		return "", errors.New("profile refresh agent is incomplete")
	}
	rest := text[i:]
	open := strings.Index(rest, "<string>")
	close := strings.Index(rest, "</string>")
	if open < 0 || close < open {
		return "", errors.New("profile refresh agent is incomplete")
	}
	return html.UnescapeString(rest[open+len("<string>") : close]), nil
}

func sameExecutable(program, self string) (bool, error) {
	a, err := os.Stat(program)
	if err != nil {
		return false, err
	}
	b, err := os.Stat(self)
	if err != nil {
		return false, err
	}
	return os.SameFile(a, b), nil
}

func writeAtomicFile(path string, body []byte, mode os.FileMode) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, body, mode); err != nil {
		return err
	}
	if err := os.Chmod(tmp, mode); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, path)
}
