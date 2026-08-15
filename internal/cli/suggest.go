package cli

import (
	"strings"

	"github.com/rainhuang0220/whereToken/internal/fuzzy"
)

func suggestKnown(want string, ids []string) string {
	want = strings.ToLower(strings.TrimSpace(want))
	if want == "" {
		return ""
	}
	compactWant := compactID(want)
	for _, id := range ids {
		if strings.HasPrefix(id, want) || strings.HasPrefix(compactID(id), compactWant) {
			return id
		}
		if strings.Contains(id, want) {
			return id
		}
	}
	return fuzzy.Closest(want, ids, 2)
}

func compactID(s string) string {
	var b []rune
	for _, r := range strings.ToLower(strings.TrimSpace(s)) {
		if r == ' ' || r == '-' || r == '_' {
			continue
		}
		b = append(b, r)
	}
	return string(b)
}

func unknownName(kind, got, suggestion string, known []string) error {
	msg := "unknown " + kind + " " + quote(got)
	if suggestion != "" {
		msg += " (did you mean " + quote(suggestion) + "?)"
	}
	if len(known) > 0 {
		msg += " (known: " + strings.Join(known, ", ") + ")"
	}
	return usageError{msg: msg}
}

func quote(s string) string {
	return `"` + s + `"`
}
