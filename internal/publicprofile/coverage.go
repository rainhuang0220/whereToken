package publicprofile

import (
	"strings"
	"time"

	"github.com/rainhuang0220/whereToken/internal/adapter"
	"github.com/rainhuang0220/whereToken/internal/event"
	"github.com/rainhuang0220/whereToken/internal/metric"
)

func tokensUnavailable(s metric.Slice) bool {
	if s.Total() > 0 {
		return false
	}
	// Measured zero is authoritative. Degraded/absent/empty 0 is missing usage.
	return s.Quality != event.QualityAuthoritative
}

func tokenStatusOf(s metric.Slice) string {
	if tokensUnavailable(s) {
		return StatusUnavailable
	}
	if s.Quality == event.QualityDegraded || s.Quality == event.QualityEstimated {
		return StatusPartial
	}
	return StatusAvailable
}

func cloudSource(id string) bool {
	for _, t := range adapter.Catalog() {
		if t.ID == id {
			return t.Cloud
		}
	}
	return false
}

func coverageFromSlice(id string, s metric.Slice, asSource bool, in Input, loc *time.Location) Coverage {
	if asSource {
		return coverageFor(id, s, in, loc)
	}
	cov := Coverage{
		Tokens:      tokenStatusOf(s),
		Requests:    StatusAvailable,
		TokenSource: TokenSourceUnknown,
	}
	if !tokensUnavailable(s) {
		cov.TokenSource = TokenSourceLocal
	}
	if cov.Tokens == StatusUnavailable {
		cov.Reason = ReasonLocalTokensMissing
	}
	return cov
}

func coverageFor(id string, s metric.Slice, in Input, loc *time.Location) Coverage {
	cov := Coverage{
		Tokens:      tokenStatusOf(s),
		Requests:    StatusAvailable,
		TokenSource: tokenSourceOf(id, in.Events),
	}
	if cov.TokenSource == TokenSourceAccountAPI {
		cov.TokenWindow = tokenWindowOf(id, in, loc)
	}
	if cov.Tokens != StatusAvailable {
		cov.Reason = tokenReason(id, in)
	}
	return cov
}

func tokenSourceOf(id string, events []event.UsageEvent) string {
	var hasAPI, hasDerived, hasLocal bool
	for _, e := range events {
		if publicID(e.Source, true) != id {
			continue
		}
		switch e.Derivation {
		case event.DeriveProviderAPI:
			hasAPI = true
		case event.DeriveDerived:
			hasDerived = true
		default:
			hasLocal = true
		}
	}
	switch {
	case hasAPI:
		return TokenSourceAccountAPI
	case hasDerived && !hasLocal:
		return TokenSourceDerived
	case hasLocal:
		return TokenSourceLocal
	default:
		return TokenSourceUnknown
	}
}

func publicID(raw string, asSource bool) string {
	if asSource {
		id, _ := publicSource(raw)
		return id
	}
	id, _ := publicVendor(raw)
	return id
}

func tokenWindowOf(id string, in Input, loc *time.Location) *DateRange {
	if id == "cursor" {
		now := in.Now
		if now.IsZero() {
			now = time.Now()
		}
		if loc == nil {
			loc = time.UTC
		}
		from := now.In(loc).AddDate(0, 0, -53*7).Format("2006-01-02")
		return &DateRange{From: &from, To: now.In(loc).Format("2006-01-02"), Label: "53w account usage"}
	}
	var from, to string
	for _, e := range in.Events {
		if publicID(e.Source, true) != id || e.Derivation != event.DeriveProviderAPI || e.Timestamp.IsZero() {
			continue
		}
		if loc == nil {
			loc = e.Timestamp.Location()
		}
		d := e.Timestamp.In(loc).Format("2006-01-02")
		if from == "" || d < from {
			from = d
		}
		if to == "" || d > to {
			to = d
		}
	}
	if from == "" {
		return nil
	}
	fromCopy := from
	return &DateRange{From: &fromCopy, To: to, Label: "account usage"}
}

func tokenReason(id string, in Input) string {
	if in.Offline && cloudSource(id) {
		return ReasonAccountAPISkipped
	}
	msg := sourceError(id, in.Errors)
	switch {
	case strings.Contains(msg, "未找到本机登录态") || strings.Contains(strings.ToLower(msg), "auth"):
		return ReasonAuthMissing
	case msg != "":
		return ReasonAPIFailed
	case cloudSource(id):
		return ReasonAccountAPISkipped
	default:
		return ReasonLocalTokensMissing
	}
}

func sourceError(id string, errs []string) string {
	prefix := id + ": "
	for _, msg := range errs {
		if strings.HasPrefix(msg, prefix) {
			return strings.TrimPrefix(msg, prefix)
		}
	}
	return ""
}
