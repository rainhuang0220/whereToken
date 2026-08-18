package scan

import (
	"strings"

	"github.com/rainhuang0220/whereToken/internal/event"
	"github.com/rainhuang0220/whereToken/internal/metric"
)

// AgentStatus is one row of doctor / sources, derived from a completed scan.
type AgentStatus struct {
	ID       string
	Label    string
	Detected bool
	Usage    bool
	Quality  event.Quality
	Path     string
	Error    string
}

func Diagnose(res Result) []AgentStatus {
	byID := map[string]metric.Slice{}
	for _, s := range res.Summary.BySource {
		byID[s.ID] = s
	}
	paths := map[string][]string{}
	for _, r := range res.Roots {
		paths[r.ID] = append(paths[r.ID], r.Path)
	}
	errs := map[string]string{}
	for _, e := range res.Errors {
		id, msg, ok := strings.Cut(e, ":")
		if !ok {
			continue
		}
		id = strings.TrimSpace(id)
		if _, exists := errs[id]; !exists {
			errs[id] = strings.TrimSpace(msg)
		}
	}

	var out []AgentStatus
	for _, id := range metric.KnownSourceIDs() {
		st := AgentStatus{ID: id, Label: metric.SourceLabel(id)}
		if p := paths[id]; len(p) > 0 {
			st.Detected = true
			st.Path = strings.Join(p, ", ")
		}
		if s, ok := byID[id]; ok {
			st.Quality = s.Quality
			st.Usage = s.Total() > 0 || s.Requests > 0 || s.UserTurns > 0
		} else if st.Detected {
			st.Quality = event.QualityAbsent
		}
		st.Error = errs[id]
		out = append(out, st)
	}
	return out
}
