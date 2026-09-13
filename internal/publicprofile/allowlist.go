package publicprofile

import (
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/rainhuang0220/whereToken/internal/metric"
	"github.com/rainhuang0220/whereToken/internal/price"
	"github.com/rainhuang0220/whereToken/internal/vendor"
)

func publicSource(id string) (string, string) {
	if canonical, ok := metric.LookupSource(id); ok {
		return canonical, metric.SourceLabel(canonical)
	}
	return AgentOtherID, AgentOtherLabel
}

func publicVendor(id string) (string, string) {
	n := strings.ToLower(strings.TrimSpace(id))
	for _, known := range vendor.KnownIDs() {
		if known == "unknown" {
			continue
		}
		if n == known || n == strings.ToLower(vendor.Label(known)) {
			return known, vendor.Label(known)
		}
	}
	return VendorOtherID, VendorOtherLabel
}

func publicModel(raw, vend string) (string, string) {
	rate, norm, ok := price.Resolve(vend, raw, time.Time{})
	_ = rate
	if !ok || norm == "" {
		return ModelOtherID, ModelOtherLabel
	}
	if strings.ContainsAny(norm, "/\\<>\"'") || strings.Contains(norm, "..") {
		return ModelOtherID, ModelOtherLabel
	}
	return norm, norm
}

var githubLoginRe = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9-]{0,37}[A-Za-z0-9])?$`)

func sanitizeOwner(in *Owner) (*Owner, error) {
	if in == nil {
		return nil, nil
	}
	out := &Owner{}
	if n := strings.TrimSpace(in.DisplayName); n != "" {
		if utf8.RuneCountInString(n) > 80 {
			n = string([]rune(n)[:80])
		}
		n = strings.Map(func(r rune) rune {
			if r < 32 || r == 127 {
				return -1
			}
			return r
		}, n)
		out.DisplayName = n
	}
	if login := strings.TrimSpace(in.GitHubLogin); login != "" {
		if !githubLoginRe.MatchString(login) {
			return nil, errOwner("github_login")
		}
		out.GitHubLogin = login
	}
	if u := strings.TrimSpace(in.AvatarURL); u != "" {
		if err := requireHTTPSGitHub(u); err != nil {
			return nil, err
		}
		out.AvatarURL = u
	}
	if u := strings.TrimSpace(in.ProfileURL); u != "" {
		if err := requireHTTPSGitHubProfile(u, out.GitHubLogin); err != nil {
			return nil, err
		}
		out.ProfileURL = u
	}
	if out.DisplayName == "" && out.GitHubLogin == "" && out.AvatarURL == "" && out.ProfileURL == "" {
		return nil, nil
	}
	return out, nil
}

type ownerError struct{ field string }

func errOwner(field string) error { return ownerError{field: field} }

func (e ownerError) Error() string { return "publicprofile: invalid owner." + e.field }

func requireHTTPSGitHub(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return errOwner("url")
	}
	host := strings.ToLower(u.Host)
	if host != "github.com" && !strings.HasSuffix(host, ".github.com") &&
		host != "avatars.githubusercontent.com" && !strings.HasSuffix(host, ".githubusercontent.com") {
		return errOwner("url")
	}
	return nil
}

func requireHTTPSGitHubProfile(raw, login string) error {
	if err := requireHTTPSGitHub(raw); err != nil {
		return err
	}
	u, _ := url.Parse(raw)
	if strings.ToLower(u.Host) != "github.com" {
		return errOwner("profile_url")
	}
	if login != "" && !strings.EqualFold(strings.Trim(u.Path, "/"), login) {
		return errOwner("profile_url")
	}
	return nil
}
