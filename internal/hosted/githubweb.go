package hosted

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"strings"
	"time"
)

// githubWebFallbacks are addresses inside GitHub's published web range
// 140.82.112.0/20. The production host's DNS answer for github.com is a
// single address that has failed to complete TCP while these addresses
// still presented a certificate for github.com. They are tried only after
// that DNS address fails. The TLS ServerName stays the URL host.
var githubWebFallbacks = []string{
	"140.82.112.4",
	"140.82.113.4",
	"140.82.114.4",
}

func githubOAuthClient() *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	dialer := &net.Dialer{Timeout: 4 * time.Second, KeepAlive: 30 * time.Second}
	transport.DialTLSContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, err
		}
		return connectAddresses(ctx, githubDialTargets(host, port), host, dialer.DialContext, productionWrap)
	}
	return &http.Client{Timeout: 20 * time.Second, Transport: transport}
}

func githubWebHost(host string) bool {
	host = strings.TrimSuffix(strings.ToLower(host), ".")
	return host == "github.com" || host == "www.github.com"
}

func githubDialTargets(host, port string) []string {
	first := net.JoinHostPort(host, port)
	if !githubWebHost(host) {
		return []string{first}
	}
	out := []string{first}
	seen := map[string]bool{first: true}
	for _, ip := range githubWebFallbacks {
		addr := net.JoinHostPort(ip, port)
		if seen[addr] {
			continue
		}
		seen[addr] = true
		out = append(out, addr)
	}
	return out
}

func oauthTLSConfig(serverName string) *tls.Config {
	return &tls.Config{ServerName: serverName}
}

type rawDial func(ctx context.Context, network, address string) (net.Conn, error)
type tlsWrap func(raw net.Conn, serverName string) (net.Conn, error)

func connectAddresses(ctx context.Context, addresses []string, serverName string, dial rawDial, wrap tlsWrap) (net.Conn, error) {
	var last error
	for _, address := range addresses {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		attempt, cancel := context.WithTimeout(ctx, 4*time.Second)
		raw, err := dial(attempt, "tcp", address)
		if err != nil {
			cancel()
			last = err
			continue
		}
		secured, err := wrap(raw, serverName)
		cancel()
		if err != nil {
			_ = raw.Close()
			last = err
			continue
		}
		return secured, nil
	}
	if last == nil {
		last = errors.New("github dial")
	}
	return nil, last
}

func productionWrap(raw net.Conn, serverName string) (net.Conn, error) {
	cfg := oauthTLSConfig(serverName)
	if cfg.InsecureSkipVerify {
		_ = raw.Close()
		return nil, errors.New("tls verification disabled")
	}
	conn := tls.Client(raw, cfg)
	_ = raw.SetDeadline(time.Now().Add(5 * time.Second))
	err := conn.Handshake()
	_ = raw.SetDeadline(time.Time{})
	if err != nil {
		_ = raw.Close()
		return nil, err
	}
	if len(conn.ConnectionState().PeerCertificates) == 0 {
		_ = raw.Close()
		return nil, errors.New("tls certificate")
	}
	return conn, nil
}
