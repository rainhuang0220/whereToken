package report

import (
	"regexp"
)

var (
	jwtRE             = regexp.MustCompile(`eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+`)
	bearerRE          = regexp.MustCompile(`(?i)bearer\s+\S+`)
	skRE              = regexp.MustCompile(`\bsk-[A-Za-z0-9]{10,}`)
	hexRE             = regexp.MustCompile(`\b[A-Fa-f0-9]{40,}\b`)
	authRE            = regexp.MustCompile(`(?i)authorization:\s*\S+`)
	xAPIKeyRE         = regexp.MustCompile(`(?i)x-api-key:\s*\S+`)
	openaiAPIKeyRE    = regexp.MustCompile(`(?i)openai-api-key:\s*\S+`)
	anthropicAPIKeyRE = regexp.MustCompile(`(?i)anthropic-api-key:\s*\S+`)
	googAPIKeyRE      = regexp.MustCompile(`(?i)x-goog-api-key:\s*\S+`)
	tokenParamRE      = regexp.MustCompile(`(?i)((?:access|refresh|id)_token|api[_-]?key)=[^&\s]+`)
	cookieRE          = regexp.MustCompile(`(?i)((?:set-)?cookie):\s*.+`)
	cloudIDEJWTRE     = regexp.MustCompile(`(?i)cloud-ide-jwt\s+\S+`)
)

func Redact(s string) string {
	s = cookieRE.ReplaceAllString(s, "${1}: [redacted]")
	s = cloudIDEJWTRE.ReplaceAllString(s, "Cloud-IDE-JWT [redacted]")
	s = jwtRE.ReplaceAllString(s, "[redacted]")
	s = bearerRE.ReplaceAllString(s, "bearer [redacted]")
	s = authRE.ReplaceAllString(s, "authorization: [redacted]")
	s = xAPIKeyRE.ReplaceAllString(s, "x-api-key: [redacted]")
	s = openaiAPIKeyRE.ReplaceAllString(s, "openai-api-key: [redacted]")
	s = anthropicAPIKeyRE.ReplaceAllString(s, "anthropic-api-key: [redacted]")
	s = googAPIKeyRE.ReplaceAllString(s, "x-goog-api-key: [redacted]")
	s = skRE.ReplaceAllString(s, "[redacted]")
	s = hexRE.ReplaceAllString(s, "[redacted]")
	s = tokenParamRE.ReplaceAllString(s, "${1}=[redacted]")
	return s
}
