package hosted

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"strings"
	"sync"
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
	if len(addresses) == 0 {
		return nil, errors.New("github dial")
	}
	ctx, cancel := context.WithTimeout(ctx, 12*time.Second)
	type result struct {
		conn net.Conn
		err  error
	}
	ch := make(chan result, len(addresses))
	var wg sync.WaitGroup
	for _, address := range addresses {
		wg.Add(1)
		go func(address string) {
			defer wg.Done()
			attempt, attemptCancel := context.WithTimeout(ctx, 8*time.Second)
			defer attemptCancel()
			raw, err := dial(attempt, "tcp", address)
			if err != nil {
				ch <- result{err: err}
				return
			}
			secured, err := wrap(raw, serverName)
			if err != nil {
				_ = raw.Close()
				ch <- result{err: err}
				return
			}
			ch <- result{conn: secured}
		}(address)
	}
	var last error
	received := 0
	for received < len(addresses) {
		res := <-ch
		received++
		if res.err != nil {
			last = res.err
			continue
		}
		cancel()
		rest := len(addresses) - received
		go func() {
			for i := 0; i < rest; i++ {
				extra := <-ch
				if extra.conn != nil {
					_ = extra.conn.Close()
				}
			}
			wg.Wait()
		}()
		return res.conn, nil
	}
	cancel()
	wg.Wait()
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
	_ = raw.SetDeadline(time.Now().Add(8 * time.Second))
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
