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
	for _, root := range a.Discover(home) {
		path := resolveDB(root.Path)
		if path == "" {
			continue
		}
		token, _, err := authTokens(path, false)
		if err != nil || token == "" {
			continue
		}
		if sub := JWTSubject(token); sub != "" {
			return sub
		}
	}
	return ""
}
