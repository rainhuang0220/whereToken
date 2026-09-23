package hosted

import (
	"fmt"
	"net"
	"os"
	"strings"
)

type Config struct {
	Listen             string
	MySQLDSN           string
	GitHubClientID     string
	GitHubClientSecret string
	PublicURL          string
	CookieSecure       bool
	Version            string
	Publish            ProfilePublish
}

// ProfilePublish is the server-side GitHub App used to materialize a profile
// README. Empty AppID leaves publishing unavailable. The browser cannot
// supply a repository, branch, or path.
type ProfilePublish struct {
	AppID          string
	InstallationID string
	PrivateKeyPEM  []byte
	Repo           string
	Branch         string
	ReadmePath     string
	PreviewDir     string
	Origins        []string
}

func (p ProfilePublish) configured() bool {
	return strings.TrimSpace(p.AppID) != "" &&
		strings.TrimSpace(p.InstallationID) != "" &&
		len(p.PrivateKeyPEM) > 0 &&
		strings.TrimSpace(p.Repo) != ""
}

func ParseListen(addr string) (string, error) {
	host, port, err := net.SplitHostPort(strings.TrimSpace(addr))
	if err != nil {
		return "", fmt.Errorf("listen address: %w", err)
	}
	switch strings.ToLower(host) {
	case "127.0.0.1", "localhost":
		return net.JoinHostPort(host, port), nil
	default:
		return "", fmt.Errorf("refusing non-localhost bind %s", addr)
	}
}

func ConfigFromEnv(getenv func(string) string) (Config, error) {
	if getenv == nil {
		return Config{}, fmt.Errorf("missing env lookup")
	}
	listen := strings.TrimSpace(getenv("WHERETOKEN_LISTEN"))
	if listen == "" {
		listen = "127.0.0.1:3400"
	}
	listen, err := ParseListen(listen)
	if err != nil {
		return Config{}, err
	}
	cfg := Config{
		Listen:             listen,
		MySQLDSN:           strings.TrimSpace(getenv("WHERETOKEN_MYSQL_DSN")),
		GitHubClientID:     strings.TrimSpace(getenv("WHERETOKEN_GITHUB_CLIENT_ID")),
		GitHubClientSecret: strings.TrimSpace(getenv("WHERETOKEN_GITHUB_CLIENT_SECRET")),
		PublicURL:          strings.TrimRight(strings.TrimSpace(getenv("WHERETOKEN_PUBLIC_URL")), "/"),
		CookieSecure:       true,
	}
	if v := strings.TrimSpace(getenv("WHERETOKEN_COOKIE_SECURE")); v == "0" || strings.EqualFold(v, "false") {
		cfg.CookieSecure = false
	}
	if cfg.MySQLDSN == "" || cfg.GitHubClientID == "" || cfg.GitHubClientSecret == "" || cfg.PublicURL == "" {
		return Config{}, fmt.Errorf("WHERETOKEN_MYSQL_DSN, WHERETOKEN_GITHUB_CLIENT_ID, WHERETOKEN_GITHUB_CLIENT_SECRET, and WHERETOKEN_PUBLIC_URL are required")
	}
	publish, err := profilePublishFromEnv(getenv)
	if err != nil {
		return Config{}, err
	}
	cfg.Publish = publish
	return cfg, nil
}

func profilePublishFromEnv(getenv func(string) string) (ProfilePublish, error) {
	p := ProfilePublish{
		AppID:          strings.TrimSpace(getenv("WHERETOKEN_GITHUB_APP_ID")),
		InstallationID: strings.TrimSpace(getenv("WHERETOKEN_GITHUB_APP_INSTALLATION_ID")),
		Repo:           strings.TrimSpace(getenv("WHERETOKEN_PROFILE_REPO")),
		Branch:         strings.TrimSpace(getenv("WHERETOKEN_PROFILE_BRANCH")),
		ReadmePath:     strings.TrimSpace(getenv("WHERETOKEN_PROFILE_README")),
		PreviewDir:     strings.TrimSpace(getenv("WHERETOKEN_PROFILE_PREVIEW_DIR")),
	}
	if p.Branch == "" {
		p.Branch = "main"
	}
	if p.ReadmePath == "" {
		p.ReadmePath = "README.md"
	}
	if p.PreviewDir == "" {
		p.PreviewDir = "wheretoken"
	}
	if raw := strings.TrimSpace(getenv("WHERETOKEN_PUBLIC_PROFILE_ORIGINS")); raw != "" {
		for _, part := range strings.Split(raw, ",") {
			if origin := strings.TrimRight(strings.TrimSpace(part), "/"); origin != "" {
				p.Origins = append(p.Origins, origin)
			}
		}
	}
	if len(p.Origins) == 0 {
		p.Origins = []string{"https://rainhuang0220.github.io"}
	}
	if path := strings.TrimSpace(getenv("WHERETOKEN_GITHUB_APP_PRIVATE_KEY_FILE")); path != "" {
		b, err := os.ReadFile(path)
		if err != nil {
			return ProfilePublish{}, fmt.Errorf("WHERETOKEN_GITHUB_APP_PRIVATE_KEY_FILE: %w", err)
		}
		p.PrivateKeyPEM = b
	}
	return p, nil
}
