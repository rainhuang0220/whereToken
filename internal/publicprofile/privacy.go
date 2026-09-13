package publicprofile

import "strings"

var denylist = []string{
	"/Users/",
	"/home/",
	`C:\`,
	"access_token",
	"refresh_token",
	"Bearer ",
	"Authorization",
	"private key",
	"BEGIN RSA",
	"eyJhbGci",
}

func Sensitive(s string) bool {
	low := strings.ToLower(s)
	for _, n := range denylist {
		if strings.Contains(low, strings.ToLower(n)) {
			return true
		}
	}
	if strings.Contains(s, "sk-") {
		return true
	}
	return false
}
