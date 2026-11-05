package cli

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"strings"
	"time"
)

const (
	profileHostedName      = "wheretoken.plainlist.space"
	macProfileResolverFile = "/etc/resolver/" + profileHostedName
)

// scopedProfileRefreshTransport honors the macOS per-domain resolver for the
// whereToken hosted API even in CGO_ENABLED=0 release binaries.
//
// Pure Go resolution on macOS does not consult /etc/resolver. In a TUN/Fake-IP
// environment it can connect to the synthetic 198.18.0.0/15 address instead
// of the public server, despite macOS native clients working normally.
//
// Only the exact official HTTPS hostname uses the scoped resolver. All other
// hosts (including redirects) keep the normal dialer, and TLS still verifies
// the original URL hostname. No raw IP or proxy setting is hard-coded.
func scopedProfileRefreshTransport(base string) *http.Transport {
	tr := http.DefaultTransport.(*http.Transport).Clone()
	if runtime.GOOS != "darwin" {
		return tr
	}
	u, err := url.Parse(base)
	if err != nil || u.Scheme != "https" || !strings.EqualFold(u.Hostname(), profileHostedName) {
		return tr
	}
	dnsIP, err := readScopedProfileNameserver(macProfileResolverFile)
	if err != nil {
		return tr
	}
	resolver := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, _ string) (net.Conn, error) {
			return (&net.Dialer{Timeout: 4 * time.Second}).DialContext(ctx, network, net.JoinHostPort(dnsIP, "53"))
		},
	}
	standard := &net.Dialer{Timeout: 30 * time.Second, KeepAlive: 30 * time.Second}
	scoped := &net.Dialer{Timeout: 30 * time.Second, KeepAlive: 30 * time.Second, Resolver: resolver}
	tr.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		host, _, err := net.SplitHostPort(address)
		if err == nil && strings.EqualFold(host, profileHostedName) {
			return scoped.DialContext(ctx, network, address)
		}
		return standard.DialContext(ctx, network, address)
	}
	return tr
}

func readScopedProfileNameserver(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(b), "\n") {
		line, _, _ = strings.Cut(line, "#")
		fields := strings.Fields(line)
		if len(fields) != 2 || fields[0] != "nameserver" {
			continue
		}
		ip := net.ParseIP(fields[1])
		if ip != nil && ip.IsGlobalUnicast() {
			return ip.String(), nil
		}
	}
	return "", errors.New("no usable nameserver in per-domain resolver")
}
