package report

import (
	"strings"

	"github.com/rainhuang0220/whereToken/internal/event"
	"github.com/rainhuang0220/whereToken/internal/fuzzy"
)

func suggestModel(want string, events []event.UsageEvent) string {
	want = strings.ToLower(strings.TrimSpace(want))
	if want == "" {
		return ""
	}
	seen := map[string]struct{}{}
	var names []string
	var lowers []string
	for _, e := range events {
		m := strings.TrimSpace(e.Model)
		if m == "" {
			continue
		}
		if _, ok := seen[m]; ok {
			continue
		}
		seen[m] = struct{}{}
		names = append(names, m)
		lowers = append(lowers, strings.ToLower(m))
	}
	for i, n := range lowers {
		if strings.Contains(n, want) {
			return names[i]
		}
	}
	hit := fuzzy.Closest(want, lowers, 2)
	if hit == "" {
		return ""
	}
	for i, n := range lowers {
		if n == hit {
			return names[i]
		}
	}
	return ""
}
