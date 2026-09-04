package hosted

import "testing"

func TestParseListenRejectsPublicBind(t *testing.T) {
	t.Parallel()
	for _, addr := range []string{"0.0.0.0:3400", ":3400", "[::]:3400", "1.2.3.4:3400"} {
		if _, err := ParseListen(addr); err == nil {
			t.Fatalf("ParseListen(%q) accepted public bind", addr)
		}
	}
}

func TestParseListenAllowsLoopback(t *testing.T) {
	t.Parallel()
	for _, addr := range []string{"127.0.0.1:3400", "localhost:3400", "127.0.0.1:0"} {
		got, err := ParseListen(addr)
		if err != nil {
			t.Fatalf("ParseListen(%q): %v", addr, err)
		}
		if got != addr {
			t.Fatalf("ParseListen(%q)=%q", addr, got)
		}
	}
}

func TestConfigFromEnv(t *testing.T) {
	env := func(k string) string {
		switch k {
		case "WHERETOKEN_LISTEN":
			return "127.0.0.1:3400"
		case "WHERETOKEN_MYSQL_DSN":
			return "wheretoken:secret@tcp(127.0.0.1:3306)/wheretoken"
		case "WHERETOKEN_GITHUB_CLIENT_ID":
			return "cid"
		case "WHERETOKEN_GITHUB_CLIENT_SECRET":
			return "csecret"
		case "WHERETOKEN_PUBLIC_URL":
			return "https://wheretoken.plainlist.space"
		default:
			return ""
		}
	}
	cfg, err := ConfigFromEnv(env)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Listen != "127.0.0.1:3400" || cfg.PublicURL != "https://wheretoken.plainlist.space" {
		t.Fatalf("%+v", cfg)
	}
	if cfg.GitHubClientSecret == "" || cfg.MySQLDSN == "" {
		t.Fatal("missing secrets")
	}
}

func TestConfigFromEnvRejectsEmptyDSN(t *testing.T) {
	_, err := ConfigFromEnv(func(string) string { return "" })
	if err == nil {
		t.Fatal("accepted empty env")
	}
}
