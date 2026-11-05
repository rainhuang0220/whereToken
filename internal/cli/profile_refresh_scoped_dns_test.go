package cli

import (
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestReadScopedProfileNameserver(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{"single", "nameserver 223.5.5.5\n", "223.5.5.5"},
		{"comments", "# owned by user\nnameserver 223.5.5.5 # direct DNS\n", "223.5.5.5"},
		{"ipv6", "nameserver 2606:4700:4700::1111\n", "2606:4700:4700::1111"},
		{"invalid IP", "nameserver 198.18.0.0.0\n", ""},
		{"loopback", "nameserver 127.0.0.1\n", ""},
		{"unknown", "domain wheretoken.plainlist.space\n", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "resolver")
			if err := os.WriteFile(p, []byte(c.body), 0600); err != nil {
				t.Fatal(err)
			}
			got, err := readScopedProfileNameserver(p)
			if c.want == "" {
				if err == nil {
					t.Fatalf("expected resolver rejection, got %q", got)
				}
				return
			}
			if err != nil || got != c.want {
				t.Fatalf("got %q, %v; want %q", got, err, c.want)
			}
		})
	}
}

func TestScopedProfileRefreshLiveDarwinStaticGo(t *testing.T) {
	if runtime.GOOS != "darwin" || os.Getenv("WT_TEST_LIVE_PROFILE_DNS") != "1" {
		t.Skip("run explicitly with CGO_ENABLED=0 WT_TEST_LIVE_PROFILE_DNS=1 on macOS")
	}
	// This test proves statically linked Go can use the owner's native
	// per-domain /etc/resolver rather than the TUN's synthetic Fake-IP DNS.
	resolverIP, err := readScopedProfileNameserver(macProfileResolverFile)
	if err != nil {
		t.Skipf("no scoped DNS resolver: %v", err)
	}
	t.Logf("scoped resolver present (public IP): %s", resolverIP)
	c := &http.Client{Timeout: 12 * time.Second, Transport: scopedProfileRefreshTransport("https://" + profileHostedName)}
	resp, err := c.Get("https://" + profileHostedName + "/api/health")
	if err != nil {
		t.Fatalf("scoped HTTPS request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("scoped HTTPS status = %d", resp.StatusCode)
	}
	if got := resp.Header.Get("Content-Type"); !strings.Contains(got, "json") {
		t.Fatalf("unexpected content-type %q", got)
	}
}
