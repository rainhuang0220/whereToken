package cursor

import (
	"encoding/base64"
	"encoding/json"
	"strings"

	"github.com/rainhuang0220/whereToken/internal/adapter"
)

func JWTSubject(accessToken string) string {
	parts := strings.Split(accessToken, ".")
	if len(parts) < 2 {
		return ""
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		payload, err = base64.StdEncoding.DecodeString(parts[1])
		if err != nil {
			return ""
		}
	}
	var claims struct {
		Sub string `json:"sub"`
	}
	if json.Unmarshal(payload, &claims) != nil {
		return ""
	}
	return strings.TrimSpace(claims.Sub)
}

func (a Adapter) AccountSubject(home adapter.Home) string {
	a.AllowPrompt = false
	return a.Subject(home)
}

// Subject reads the current Cursor account id. AllowPrompt controls whether
// a sealed session may ask the operating system. The returned value is the
// JWT subject, not the token.
func (a Adapter) Subject(home adapter.Home) string {
	if a.ConsentPath == "" {
		a.ConsentPath = ConsentPath(home)
	}
	for _, root := range a.Discover(home) {
		path := resolveDB(root.Path)
		if path == "" {
			continue
		}
		token, _, err := authTokens(path, false)
		if err != nil || token == "" {
			continue
		}
		opened, err := a.openSealed(token, path)
		if err != nil || !usableSessionToken(opened) {
			continue
		}
		if sub := JWTSubject(opened); sub != "" {
			return sub
		}
	}
	return ""
}

// StoredSessionKind reports jwt, sealed, missing, or unreadable.
// It does not return the stored value and it does not ask the OS.
func StoredSessionKind(home adapter.Home) string {
	roots := Adapter{}.Discover(home)
	if len(roots) == 0 {
		return "missing"
	}
	saw := false
	for _, root := range roots {
		path := resolveDB(root.Path)
		if path == "" {
			continue
		}
		token, _, err := authTokens(path, false)
		if err != nil {
			return "unreadable"
		}
		token = normalizeStoredSession(token)
		if token == "" {
			continue
		}
		saw = true
		switch {
		case strings.HasPrefix(token, sealedStoragePrefix):
			return "sealed"
		case usableSessionToken(token):
			return "jwt"
		default:
			return "unreadable"
		}
	}
	if !saw {
		return "missing"
	}
	return "missing"
}
