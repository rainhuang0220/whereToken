package profilewebembed

import (
	"encoding/json"
	"regexp"
	"sort"
	"strings"
	"testing"
)

func TestMessageCatalogsMatch(t *testing.T) {
	raw, err := Read("assets/messages.js")
	if err != nil {
		t.Fatal(err)
	}
	const marker = "window.WT_MESSAGES = "
	s := string(raw)
	i := strings.Index(s, marker)
	if i < 0 {
		t.Fatal("message catalog assignment missing")
	}
	rest := s[i+len(marker):]
	end := strings.Index(rest, ";\n")
	if end < 0 {
		t.Fatal("message catalog terminator missing")
	}
	var catalogs map[string]map[string]string
	if err := json.Unmarshal([]byte(rest[:end]), &catalogs); err != nil {
		t.Fatal(err)
	}
	en, zh := catalogs["en"], catalogs["zh-CN"]
	if len(en) == 0 || len(zh) == 0 {
		t.Fatal("both catalogs are required")
	}
	if len(en) != len(zh) {
		t.Fatalf("catalog size en=%d zh-CN=%d", len(en), len(zh))
	}
	placeholder := regexp.MustCompile(`\{[a-zA-Z]+\}`)
	var missing []string
	for key, value := range en {
		other, ok := zh[key]
		if !ok {
			missing = append(missing, key)
			continue
		}
		if strings.Join(placeholder.FindAllString(value, -1), ",") != strings.Join(placeholder.FindAllString(other, -1), ",") {
			t.Errorf("placeholder mismatch %s en=%q zh=%q", key, value, other)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		t.Fatalf("zh-CN missing %s", strings.Join(missing, ", "))
	}
	used := messageKeys(t)
	for _, key := range used {
		if _, ok := en[key]; !ok {
			t.Errorf("missing catalog key %s", key)
		}
	}
}

func TestPublicPageDefaultsToEnglishWithoutALocalhostPublishLink(t *testing.T) {
	html, err := Read("index.html")
	if err != nil {
		t.Fatal(err)
	}
	page := string(html)
	if !strings.Contains(page, `<html lang="en">`) {
		t.Fatal("document language must start as English")
	}
	for _, bad := range []string{"intent=publish", "本机发布", `href="http://127.0.0.1`, "navigator.language", "Accept-Language"} {
		if strings.Contains(page, bad) {
			t.Fatalf("index.html contains %q", bad)
		}
	}
	for _, name := range []string{"assets/lang.js", "assets/messages.js", "assets/profile.js"} {
		body, err := Read(name)
		if err != nil {
			t.Fatal(err)
		}
		s := string(body)
		for _, bad := range []string{"navigator.language", "navigator.languages", "Accept-Language", "intent=publish"} {
			if strings.Contains(s, bad) {
				t.Fatalf("%s contains %q", name, bad)
			}
		}
	}
	js, err := Read("assets/profile.js")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(js), `prev.get("lang")`) {
		t.Fatal("filter updates must keep the lang parameter")
	}
	css, err := Read("assets/profile.css")
	if err != nil {
		t.Fatal(err)
	}
	sheet := string(css)
	for _, want := range []string{".legend-ramp", "min-width: max-content", "gap: 10px", `html[data-i18n-pending] body`} {
		if !strings.Contains(sheet, want) {
			t.Fatalf("stylesheet missing %q", want)
		}
	}
}

func messageKeys(t *testing.T) []string {
	t.Helper()
	html, err := Read("index.html")
	if err != nil {
		t.Fatal(err)
	}
	js, err := Read("assets/profile.js")
	if err != nil {
		t.Fatal(err)
	}
	text := string(html) + "\n" + string(js)
	re := regexp.MustCompile(`(?:data-i18n|data-i18n-aria)="([^"]+)"|(?:^|[^A-Za-z0-9_])t\("([A-Za-z0-9]+(?:\.[A-Za-z0-9]+)*)"`)
	seen := map[string]struct{}{}
	for _, match := range re.FindAllStringSubmatch(text, -1) {
		key := match[1]
		if key == "" {
			key = match[2]
		}
		seen[key] = struct{}{}
	}
	if len(seen) < 20 {
		t.Fatalf("only found %d message keys", len(seen))
	}
	keys := make([]string, 0, len(seen))
	for key := range seen {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
