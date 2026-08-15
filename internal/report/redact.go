package report

import (
	"regexp"
)

var (
	jwtRE        = regexp.MustCompile(`eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+`)
	bearerRE     = regexp.MustCompile(`(?i)bearer\s+\S+`)
	skRE         = regexp.MustCompile(`\bsk-[A-Za-z0-9]{10,}`)
	hexRE        = regexp.MustCompile(`\b[A-Fa-f0-9]{40,}\b`)
	tokenParamRE = regexp.MustCompile(`(?i)((?:access|refresh|id)_token|api[_-]?key)=[^&\s]+`)
)

func Redact(s string) string {
	s = jwtRE.ReplaceAllString(s, "[redacted]")
	s = bearerRE.ReplaceAllString(s, "bearer [redacted]")
	s = skRE.ReplaceAllString(s, "[redacted]")
	s = hexRE.ReplaceAllString(s, "[redacted]")
	s = tokenParamRE.ReplaceAllString(s, "${1}=[redacted]")
	return s
}
