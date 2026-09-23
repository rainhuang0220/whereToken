package hosted

import (
	"context"
	"errors"
	"net"
	"net/http"
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
	var tried []string
	var names []string
	conn, err := connectAddresses(context.Background(), []string{"203.0.113.9:443", "203.0.113.10:443"}, "github.com",
		func(ctx context.Context, network, address string) (net.Conn, error) {
			tried = append(tried, address)
			if address == "203.0.113.9:443" {
				return nil, context.DeadlineExceeded
			}
			client, server := net.Pipe()
			_ = server.Close()
			return client, nil
		},
		func(raw net.Conn, serverName string) (net.Conn, error) {
			names = append(names, serverName)
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
	if len(tried) != 2 || tried[0] != "203.0.113.9:443" || tried[1] != "203.0.113.10:443" {
		t.Fatalf("dialed %v", tried)
	}
	if len(names) != 1 || names[0] != "github.com" {
		t.Fatalf("server names %v", names)
	}
}

func TestConnectAddressesTriesNextAfterHandshakeFailure(t *testing.T) {
	calls := 0
	conn, err := connectAddresses(context.Background(), []string{"203.0.113.9:443", "203.0.113.10:443"}, "github.com",
		func(ctx context.Context, network, address string) (net.Conn, error) {
			client, server := net.Pipe()
			_ = server.Close()
			return client, nil
		},
		func(raw net.Conn, serverName string) (net.Conn, error) {
			calls++
			if calls == 1 {
				return nil, errors.New("handshake")
			}
			return raw, nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	_ = conn.Close()
	if calls != 2 {
		t.Fatalf("handshakes %d", calls)
	}
}
