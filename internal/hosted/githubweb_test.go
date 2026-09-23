package hosted

import (
	"context"
	"errors"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestGitHubDialTargetsFailoverOnlyForWeb(t *testing.T) {
	web := githubDialTargets("github.com", "443")
	if len(web) != 1+len(githubWebFallbacks) || web[0] != "github.com:443" {
		t.Fatalf("web targets %v", web)
	}
	api := githubDialTargets("api.github.com", "443")
	if len(api) != 1 || api[0] != "api.github.com:443" {
		t.Fatalf("api targets %v", api)
	}
	_, network, err := net.ParseCIDR("140.82.112.0/20")
	if err != nil {
		t.Fatal(err)
	}
	for _, ip := range githubWebFallbacks {
		if !network.Contains(net.ParseIP(ip)) {
			t.Fatalf("fallback %s is outside GitHub's published web range", ip)
		}
	}
}

func TestGitHubOAuthClientKeepsVerification(t *testing.T) {
	client := githubOAuthClient()
	if client.Timeout < 15*time.Second {
		t.Fatal(client.Timeout)
	}
	transport, ok := client.Transport.(*http.Transport)
	if !ok || transport.DialTLSContext == nil {
		t.Fatal("oauth client has no verified dial")
	}
	if transport.TLSClientConfig != nil && transport.TLSClientConfig.InsecureSkipVerify {
		t.Fatal("tls verification disabled")
	}
}

func TestOAuthTLSConfigKeepsVerification(t *testing.T) {
	cfg := oauthTLSConfig("github.com")
	if cfg.InsecureSkipVerify || cfg.ServerName != "github.com" || cfg.RootCAs != nil {
		t.Fatalf("tls config %+v", cfg)
	}
}

func TestConnectAddressesSkipsDeadGitHubAddress(t *testing.T) {
	var mu sync.Mutex
	var tried []string
	var names []string
	conn, err := connectAddresses(context.Background(), []string{"203.0.113.9:443", "203.0.113.10:443"}, "github.com",
		func(ctx context.Context, network, address string) (net.Conn, error) {
			mu.Lock()
			tried = append(tried, address)
			mu.Unlock()
			if address == "203.0.113.9:443" {
				return nil, context.DeadlineExceeded
			}
			client, server := net.Pipe()
			_ = server.Close()
			return client, nil
		},
		func(raw net.Conn, serverName string) (net.Conn, error) {
			mu.Lock()
			names = append(names, serverName)
			mu.Unlock()
			if serverName != "github.com" {
				return nil, errors.New("server name")
			}
			return raw, nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	_ = conn.Close()
	if len(tried) == 0 || tried[len(tried)-1] == "" {
		t.Fatalf("dialed %v", tried)
	}
	seen := map[string]bool{}
	for _, address := range tried {
		seen[address] = true
	}
	if !seen["203.0.113.10:443"] {
		t.Fatalf("successful address was not dialed: %v", tried)
	}
	if len(names) != 1 || names[0] != "github.com" {
		t.Fatalf("server names %v", names)
	}
}

func TestConnectAddressesTriesNextAfterHandshakeFailure(t *testing.T) {
	var calls atomic.Int32
	conn, err := connectAddresses(context.Background(), []string{"203.0.113.9:443", "203.0.113.10:443"}, "github.com",
		func(ctx context.Context, network, address string) (net.Conn, error) {
			client, server := net.Pipe()
			_ = server.Close()
			return client, nil
		},
		func(raw net.Conn, serverName string) (net.Conn, error) {
			if calls.Add(1) == 1 {
				return nil, errors.New("handshake")
			}
			return raw, nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	_ = conn.Close()
	deadline := time.Now().Add(time.Second)
	for calls.Load() < 2 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if calls.Load() < 2 {
		t.Fatalf("handshakes %d", calls.Load())
	}
}

func TestConnectAddressesDoesNotWaitForBlackhole(t *testing.T) {
	start := time.Now()
	conn, err := connectAddresses(context.Background(), []string{"203.0.113.9:443", "203.0.113.10:443"}, "github.com",
		func(ctx context.Context, network, address string) (net.Conn, error) {
			if address == "203.0.113.9:443" {
				<-ctx.Done()
				return nil, ctx.Err()
			}
			client, server := net.Pipe()
			_ = server.Close()
			return client, nil
		},
		func(raw net.Conn, serverName string) (net.Conn, error) {
			if serverName != "github.com" {
				return nil, errors.New("server name")
			}
			return raw, nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	_ = conn.Close()
	if time.Since(start) > 2*time.Second {
		t.Fatalf("blackhole delayed the verified address by %s", time.Since(start))
	}
}
