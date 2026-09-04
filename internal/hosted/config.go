package hosted

import (
	"fmt"
	"net"
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
	return cfg, nil
}
