package cursor

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/rainhuang0220/whereToken/internal/adapter"
	"github.com/rainhuang0220/whereToken/internal/event"
	"github.com/rainhuang0220/whereToken/internal/vendor"
)

const (
	defaultAPIBase    = "https://api2.cursor.sh"
	cursorOAuthClient = "KbZUR41cY7W6zRSdpSUJ7I7mLYBKOCmB"
	filteredRPC       = "/aiserver.v1.DashboardService/GetFilteredUsageEvents"
	aggregatedRPC     = "/aiserver.v1.DashboardService/GetAggregatedUsageEvents"
	oauthPath         = "/oauth/token"
	maxUsagePages     = 100
	usagePageSize     = 1000
)

func (a Adapter) fetchAccountUsage(sourceRoot, access, refresh string) ([]event.UsageEvent, error) {
	if strings.TrimSpace(access) == "" {
		return nil, errNoLocalAuth
	}
	if !usableSessionToken(access) {
		return nil, errAccountUsageUnavailable
	}
	token := access
	events, err := a.fetchUsageWithToken(sourceRoot, token)
	if err != nil && adapter.IsUnauthorized(err) && compatibleRefreshToken(refresh) && refresh != access {
		if len(events) > 0 {
			// Keep the completed dated windows and retry from their persisted
			// checkpoint on the next run. A second fetch would reset them.
			return events, err
		}
		next, rerr := a.refreshAccessToken(refresh)
		if rerr != nil {
			return events, rerr
		}
		if !usableSessionToken(next) {
			return events, errExpiredAuth
		}
		if subject := JWTSubject(access); subject == "" || JWTSubject(next) != subject {
			return events, errAccountMismatch
		}
		return a.fetchUsageWithToken(sourceRoot, next)
	}
	return events, err
}

type usageSpan struct {
	start, end time.Time
	inclusive  bool
}

func (s usageSpan) coverage() CoverageWindow {
	end := s.end.UTC()
	if !s.inclusive {
		end = end.Add(-time.Millisecond)
	}
	return CoverageWindow{
		From: s.start.UTC().Format("2006-01-02"),
		To:   end.Format("2006-01-02"),
	}
}

func (a Adapter) usageWindows() []usageSpan {
	return a.uncoveredWindows(a.allUsageWindows())
}

func (a Adapter) allUsageWindows() []usageSpan {
	now := a.now().UTC()
	if a.Probe {
		start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
		if !start.Before(now) {
			start = now
		}
		return []usageSpan{{start: start, end: now, inclusive: true}}
	}
	day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	earliest := day.AddDate(0, 0, -53*7)
	cur := time.Date(earliest.Year(), earliest.Month(), 1, 0, 0, 0, 0, time.UTC)
	var out []usageSpan
	for cur.Before(now) {
		next := cur.AddDate(0, 1, 0)
		start := cur
		if start.Before(earliest) {
			start = earliest
		}
		if !start.Before(now) {
			break
		}
		if next.After(now) {
			out = append(out, usageSpan{start: start, end: now, inclusive: true})
		} else {
			out = append(out, usageSpan{start: start, end: next, inclusive: false})
		}
		cur = next
	}
	if len(out) == 0 {
		out = append(out, usageSpan{start: earliest, end: now, inclusive: true})
	}
	return out
}

// uncoveredWindows keeps uncovered history and the open UTC month. The
// existing legacy behavior also refreshes the previous month. Once an
// explicitly linked verified snapshot exists, a fully audited previous
// month can be reused after the first two days of the next UTC month.
func (a Adapter) uncoveredWindows(all []usageSpan) []usageSpan {
	if len(all) == 0 {
		return nil
	}
	var protectFrom time.Time
	if a.ProtectedThrough != "" {
		if last, err := time.Parse("2006-01-02", a.ProtectedThrough); err == nil {
			protectFrom = last.AddDate(0, 0, 1)
		}
	}
	now := a.now().UTC()
	var out []usageSpan
	for i, w := range all {
		// Older windows must still be considered: the legacy endpoint may
		// remain functional and can backfill genuinely unknown historical days.
		// Only the billing-v2 fallback skips older dates after a detected
		// schema change, inside fetchUsageWithToken.
		coverage := w.coverage()
		if !protectFrom.IsZero() && w.start.Before(protectFrom) && w.end.After(protectFrom) {
			coverage.From = protectFrom.Format("2006-01-02")
		}
		current := i == len(all)-1
		// Keep revisiting the just-closed month during the first two UTC
		// days so late-finishing requests around midnight can settle.
		boundaryRecheck := i == len(all)-2 && ((!a.Scheduled && a.ProtectedThrough == "") || now.Day() <= 2)
		if current || boundaryRecheck || !windowCovered(coverage, a.Covered) {
			out = append(out, w)
		}
	}
	if len(out) == 0 {
		out = append(out, all[len(all)-1])
	}
	return out
}

func windowCovered(w CoverageWindow, covered []CoverageWindow) bool {
	if w.From == "" || w.To == "" {
		return false
	}
	for _, c := range covered {
		if c.From != "" && c.To != "" && c.From <= w.From && c.To >= w.To {
			return true
		}
	}
	return false
}

func (a Adapter) fetchUsageWithToken(sourceRoot, token string) ([]event.UsageEvent, error) {
	clearCoverage(sourceRoot)
	now := a.now().UTC()
	a.Now = func() time.Time { return now }
	all := a.allUsageWindows()
	windows := a.uncoveredWindows(all)
	if a.Scheduled {
		windows = scheduledWindows(windows, all, now)
		var cancel context.CancelFunc
		a.collectionContext, cancel = context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()
	}
	if len(all) > 0 {
		setLookback(sourceRoot, CoverageWindow{
			From: all[0].coverage().From,
			To:   all[len(all)-1].coverage().To,
		})
	}
	seen := map[string]struct{}{}
	var out []event.UsageEvent
	var covered []CoverageWindow
	missing := 0
	availabilityOnly := 0
	var availabilityDetails string
	emptyBilling := 0
	var firstMissing, lastMissing string
	recordMissing := func(w usageSpan) {
		missing++
		cov := w.coverage()
		if firstMissing == "" || cov.From < firstMissing {
			firstMissing = cov.From
		}
		if lastMissing == "" || cov.To > lastMissing {
			lastMissing = cov.To
		}
	}
	var failed error
	useBilling := false
	var protectFrom time.Time
	if a.ProtectedThrough != "" {
		if last, err := time.Parse("2006-01-02", a.ProtectedThrough); err == nil {
			protectFrom = last.AddDate(0, 0, 1)
		}
	}
	for _, w := range windows {
		var got filteredFetch
		var err error
		if useBilling {
			if !protectFrom.IsZero() {
				if !w.end.After(protectFrom) {
					// Older dates are intentionally sourced from a retained
					// verified snapshot, not a failed remote window.
					continue
				}
				if w.start.Before(protectFrom) {
					w.start = protectFrom
				}
			}
			got, err = a.fetchBillingWindow(token, w)
		} else {
			got, err = a.fetchFilteredWindow(token, w, 0)
			if errors.Is(err, errSchemaChanged) {
				// The legacy personal endpoint can return HTTP 200 {} even
				// for active users. Only fall back on a verified schema
				// failure, never on 401/403/rate limits/network errors.
				useBilling = true
				if !protectFrom.IsZero() {
					if !w.end.After(protectFrom) {
						// Historical snapshot has priority over the new
						// endpoint's unknown retention window.
						continue
					}
					if w.start.Before(protectFrom) {
						w.start = protectFrom
					}
				}
				got, err = a.fetchBillingWindow(token, w)
			}
		}
		if err == nil && !useBilling {
			got.events, err = dedupeFilteredWindow(got.events)
		}
		if err == nil && useBilling && len(got.events) == 0 {
			// An empty response is not proof of historical retention.
			emptyBilling++
			recordMissing(w)
			continue
		}
		if err == nil {
			var added []event.UsageEvent
			out, added = dedupeUsage(out, seen, got.events)
			cov := w.coverage()
			// Billing v2's accepted closed-month bound ends at 23:59:59.
			// Keep the measured rows. The final fractional second has
			// no coverage proof, so treat that trailing day as open.
			open := useBilling || (w.inclusive && cov.To == now.Format("2006-01-02"))
			cov.Audit = auditWindow(cov, got.declared, got.raw, added, open)
			covered = append(covered, cov)
			continue
		}
		if isNotFound(err) || (useBilling && errors.Is(err, errBillingAvailability)) {
			// A billing availability-only envelope is not usage data.
			if errors.Is(err, errBillingAvailability) {
				availabilityOnly++
				if availabilityDetails == "" {
					availabilityDetails = err.Error()
				}
			}
			recordMissing(w)
			continue
		}
		// Dates and page counts are safe diagnostics. Never include the raw
		// response, credentials, conversation IDs or event bodies.
		if errors.Is(err, errSchemaChanged) {
			err = fmt.Errorf("%w [utc_window=%s..%s]", err, w.coverage().From, w.coverage().To)
		}
		switch kind := CollectionFailure(err); {
		case a.Scheduled && (kind == "SCHEMA" || kind == "COVERAGE" || kind == "SOURCE_UNAVAILABLE"):
			if failed == nil {
				failed = err
			}
			// A broken month must not prevent the other planned windows
			// from completing. Global auth/rate/network failures stop below.
			continue
		default:
			failed = err
		}
		break
	}
	if !a.Scheduled && !useBilling && len(covered) == 0 && missing == len(windows) && failed == nil && len(windows) > 0 {
		// Never retry a dead legacy aggregate endpoint after switching to billing v2.
		start := windows[0].start.UnixMilli()
		end := windows[len(windows)-1].end.UnixMilli()
		agg, aerr := a.fetchAggregated(token, start, end)
		if aerr != nil || len(agg) == 0 {
			if aerr != nil {
				return nil, aerr
			}
			return nil, errEndpointUnavailable
		}
		return tagged(sourceRoot, agg), nil
	}
	if len(covered) > 0 {
		setCoverage(sourceRoot, covered)
		if len(out) == 0 {
			latest := covered[0].To
			for _, w := range covered[1:] {
				if w.To > latest {
					latest = w.To
				}
			}
			day, _ := time.Parse("2006-01-02", latest)
			if latest == now.Format("2006-01-02") {
				day = now
			}
			out = completeAccountUsage(nil, day)
		}
	}
	if failed != nil {
		if len(out) == 0 {
			return nil, failed
		}
		// Successful months stay. The whole lookback is not a finished span.
		return tagged(sourceRoot, degradeAPI(out)), failed
	}
	if missing > 0 {
		// Dates and enumerated failure counts are safe to print; raw response,
		// cookies, account IDs and tokens must never appear in diagnostics.
		partialErr := fmt.Errorf("%w [reason=incomplete_billing_windows missing=%d first_utc=%s last_utc=%s availability_only=%d empty=%d]", errCoverage, missing, firstMissing, lastMissing, availabilityOnly, emptyBilling)
		if availabilityDetails != "" {
			partialErr = fmt.Errorf("%w [availability_detail=%s]", partialErr, availabilityDetails)
		}
		if len(out) == 0 {
			return nil, partialErr
		}
		return tagged(sourceRoot, degradeAPI(out)), partialErr
	}
	return tagged(sourceRoot, out), nil
}

type filteredFetch struct {
	events        []event.UsageEvent
	declared, raw int
}

func (a Adapter) fetchFilteredWindow(token string, w usageSpan, depth int) (filteredFetch, error) {
	endMs := w.end.UnixMilli()
	if !w.inclusive && endMs > w.start.UnixMilli() {
		endMs--
	}
	got, err := a.fetchFiltered(token, w.start.UnixMilli(), endMs)
	if err == nil && a.Scheduled {
		for _, e := range got.events {
			if e.Timestamp.IsZero() || e.Timestamp.UnixMilli() < w.start.UnixMilli() || e.Timestamp.UnixMilli() > endMs {
				return filteredFetch{}, fmt.Errorf("%w [reason=filtered_event_outside_window]", errCoverage)
			}
		}
	}
	// In a live period Cursor can add usage between pages, changing its
	// declared total. Re-query a smaller complete time slice rather than
	// accepting an unverifiable partial page or giving up the entire scan.
	var drift *filteredCountDrift
	canSplit := err == errUsagePageLimit || errors.As(err, &drift)
	if err == nil || !canSplit || depth >= 6 || w.end.Sub(w.start) < time.Hour {
		if err == errUsagePageLimit && (depth >= 6 || w.end.Sub(w.start) < time.Hour) {
			return filteredFetch{}, errPaginationIncomplete
		}
		return got, err
	}
	mid := w.start.Add(w.end.Sub(w.start) / 2)
	left, lerr := a.fetchFilteredWindow(token, usageSpan{start: w.start, end: mid, inclusive: false}, depth+1)
	if lerr != nil {
		return filteredFetch{}, lerr
	}
	right, rerr := a.fetchFilteredWindow(token, usageSpan{start: mid, end: w.end, inclusive: w.inclusive}, depth+1)
	if rerr != nil {
		return filteredFetch{}, rerr
	}
	left.events = append(left.events, right.events...)
	left.declared += right.declared
	left.raw += right.raw
	return left, nil
}

const filteredEventIDPrefix = "cursor-filtered-event:"

func dedupeFilteredWindow(events []event.UsageEvent) ([]event.UsageEvent, error) {
	out := events[:0]
	seen := make(map[string]int, len(events))
	for _, e := range events {
		if i, ok := seen[e.RequestID]; ok {
			if !strings.HasPrefix(e.RequestID, filteredEventIDPrefix) || out[i] != e {
				return nil, fmt.Errorf("%w [reason=ambiguous_filtered_duplicate]", errPaginationIncomplete)
			}
			continue
		}
		seen[e.RequestID] = len(out)
		out = append(out, e)
	}
	return out, nil
}

func dedupeUsage(dst []event.UsageEvent, seen map[string]struct{}, add []event.UsageEvent) (out, added []event.UsageEvent) {
	for _, e := range add {
		if e.RequestID != "" {
			if _, ok := seen[e.RequestID]; ok {
				continue
			}
			seen[e.RequestID] = struct{}{}
		}
		added = append(added, e)
		dst = append(dst, e)
	}
	return dst, added
}

func auditWindow(cov CoverageWindow, declared, raw int, kept []event.UsageEvent, open bool) WindowAudit {
	audit := WindowAudit{
		From: cov.From, To: cov.To,
		Declared: declared, RawRows: raw, Deduped: len(kept),
		Validation: "ok",
	}
	if open {
		audit.Validation = "open"
	}
	days := map[string]struct{}{}
	for _, e := range kept {
		if e.Timestamp.IsZero() {
			continue
		}
		day := e.Timestamp.UTC().Format("2006-01-02")
		if day < cov.From || day > cov.To {
			continue
		}
		days[day] = struct{}{}
		audit.Miss += e.Miss
		audit.CacheRead += e.CacheRead
		audit.CacheCreate += e.CacheCreate
		audit.Output += e.Output
	}
	audit.Days = len(days)
	return audit
}

func isNotFound(err error) bool {
	es, ok := err.(adapter.ErrStatus)
	return ok && es.Code == http.StatusNotFound
}

func stopAccountFallback(err error) bool {
	es, ok := err.(adapter.ErrStatus)
	if !ok {
		return false
	}
	switch es.Code {
	case http.StatusUnauthorized, http.StatusForbidden, http.StatusTooManyRequests:
		return true
	default:
		return false
	}
}

func completeAccountUsage(events []event.UsageEvent, now time.Time) []event.UsageEvent {
	if len(events) != 0 {
		return events
	}
	return []event.UsageEvent{{
		Source:      "cursor",
		RequestID:   "cursor-api:measured-zero",
		Timestamp:   now,
		Quality:     event.QualityAuthoritative,
		Derivation:  event.DeriveProviderAPI,
		SkipRequest: true,
	}}
}

func tagged(sourceRoot string, events []event.UsageEvent) []event.UsageEvent {
	for i := range events {
		events[i].SourceRoot = sourceRoot
	}
	return events
}

func (a Adapter) pageSize() int {
	if a.PageSize > 0 {
		return a.PageSize
	}
	return usagePageSize
}

func (a Adapter) pageCap() int {
	if a.PageCap > 0 {
		return a.PageCap
	}
	return maxUsagePages
}

// filteredCountDrift covers a changing (or internally contradictory)
// upstream count. The error intentionally contains only public pagination
// numbers; it never embeds event records, JWTs or HTTP response bodies.
type filteredCountDrift struct {
	reason        string
	page          int
	before, after int
}

func (e *filteredCountDrift) Error() string {
	return fmt.Sprintf("%s [reason=%s page=%d expected=%d observed=%d]", errSchemaChanged, e.reason, e.page, e.before, e.after)
}

func (e *filteredCountDrift) Unwrap() error { return errSchemaChanged }

func (a Adapter) fetchFiltered(token string, start, end int64) (filteredFetch, error) {
	var out filteredFetch
	var total int
	full := a.pageSize()
	pageSize := full
	for page := 1; page <= a.pageCap(); page++ {
		body := map[string]any{
			"teamId":    0,
			"startDate": start,
			"endDate":   end,
			"page":      page,
			"pageSize":  full,
		}
		raw, err := a.postJSON(filteredRPC, token, body)
		if err != nil {
			return filteredFetch{}, err
		}
		pageEvents, rawN, pageTotal, known, err := parseFiltered(raw)
		if err != nil {
			return filteredFetch{}, err
		}
		if !known {
			return filteredFetch{}, fmt.Errorf("%w [reason=missing_declared_count page=%d]", errSchemaChanged, page)
		}
		if page == 1 {
			total = pageTotal
			if rawN > 0 && rawN < full {
				pageSize = rawN
			}
		} else if pageTotal != total {
			return filteredFetch{}, &filteredCountDrift{reason: "count_changed_between_pages", page: page, before: total, after: pageTotal}
		}
		if out.raw+rawN > total {
			return filteredFetch{}, &filteredCountDrift{reason: "rows_exceed_declared_count", page: page, before: total, after: out.raw + rawN}
		}
		out.events = append(out.events, pageEvents...)
		out.raw += rawN
		out.declared = total
		if out.raw == total {
			if total > 0 && len(out.events) == 0 {
				return filteredFetch{}, fmt.Errorf("%w [reason=missing_token_meters]", errSchemaChanged)
			}
			return out, nil
		}
		if rawN == 0 || rawN < pageSize {
			return filteredFetch{}, errPaginationIncomplete
		}
	}
	if pageSize < full {
		return filteredFetch{}, errPaginationIncomplete
	}
	return filteredFetch{}, errUsagePageLimit
}

func (a Adapter) fetchAggregated(token string, start, end int64) ([]event.UsageEvent, error) {
	body := map[string]any{
		"teamId":    0,
		"startDate": start,
		"endDate":   end,
	}
	raw, err := a.postJSON(aggregatedRPC, token, body)
	if err != nil {
		return nil, err
	}
	events, ok := parseAggregated(raw, a.now())
	if !ok {
		return nil, fmt.Errorf("%w [reason=invalid_aggregate_json]", errSchemaChanged)
	}
	return events, nil
}

func (a Adapter) refreshAccessToken(refresh string) (string, error) {
	payload := map[string]string{
		"grant_type":    "refresh_token",
		"client_id":     cursorOAuthClient,
		"refresh_token": refresh,
	}
	raw, err := a.postJSON(oauthPath, "", payload)
	if err != nil {
		return "", err
	}
	var obj map[string]any
	if json.Unmarshal(raw, &obj) != nil {
		return "", errExpiredAuth
	}
	tok, _ := obj["access_token"].(string)
	tok = strings.TrimSpace(tok)
	if tok == "" {
		return "", errExpiredAuth
	}
	return tok, nil
}

func (a Adapter) postJSON(path, token string, payload any) ([]byte, error) {
	return a.postJSONOnce(path, token, payload, true)
}

func (a Adapter) postJSONOnce(path, token string, payload any, retry bool) ([]byte, error) {
	base := a.apiBase()
	u, err := url.Parse(base + path)
	if err != nil {
		return nil, fmt.Errorf("账号用量接口地址无效")
	}
	if !a.allowedURL(u) {
		return nil, fmt.Errorf("拒绝访问非 Cursor 主机")
	}
	buf, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(a.requestContext(), http.MethodPost, u.String(), bytes.NewReader(buf))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Connect-Protocol-Version", "1")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+normalizeStoredSession(token))
	}
	resp, err := a.client().Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w [phase=request]", errNetwork)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusTooManyRequests && retry {
		if d, ok := retryAfterWait(resp.Header); ok {
			resp.Body.Close()
			timer := time.NewTimer(d)
			defer timer.Stop()
			select {
			case <-a.requestContext().Done():
				return nil, errNetwork
			case <-timer.C:
			}
			return a.postJSONOnce(path, token, payload, false)
		}
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, adapter.ErrStatus{Code: resp.StatusCode}
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, adapter.MaxHTTPBody+1))
	if err != nil {
		return nil, fmt.Errorf("%w [phase=body_read]", errNetwork)
	}
	if len(raw) > adapter.MaxHTTPBody {
		return nil, fmt.Errorf("%w [reason=filtered_body_limit]", errSchemaChanged)
	}
	return raw, nil
}

func retryAfterWait(h http.Header) (time.Duration, bool) {
	v := strings.TrimSpace(h.Get("Retry-After"))
	if v == "" {
		return 0, false
	}
	if n, err := strconv.Atoi(v); err == nil {
		d := time.Duration(n) * time.Second
		if d >= time.Millisecond && d <= 2*time.Second {
			return d, true
		}
		return 0, false
	}
	if t, err := http.ParseTime(v); err == nil {
		d := time.Until(t)
		if d >= time.Millisecond && d <= 2*time.Second {
			return d, true
		}
	}
	return 0, false
}

func (a Adapter) client() *http.Client {
	if a.HTTP != nil {
		c := *a.HTTP
		c.CheckRedirect = a.restrictRedirect
		return &c
	}
	return &http.Client{Timeout: 20 * time.Second, CheckRedirect: a.restrictRedirect}
}

func (a Adapter) restrictRedirect(req *http.Request, _ []*http.Request) error {
	if req.URL == nil || !a.allowedURL(req.URL) {
		return fmt.Errorf("拒绝访问非 Cursor 主机")
	}
	return nil
}

func (a Adapter) apiBase() string {
	if strings.TrimSpace(a.APIBase) != "" {
		return strings.TrimRight(a.APIBase, "/")
	}
	return defaultAPIBase
}

func (a Adapter) now() time.Time {
	if a.Now != nil {
		return a.Now()
	}
	return time.Now()
}

func (a Adapter) allowedURL(u *url.URL) bool {
	if u == nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	switch host {
	case "api2.cursor.sh", "cursor.com", "www.cursor.com":
		return true
	}
	if extra := strings.TrimSpace(a.APIBase); extra != "" {
		eu, err := url.Parse(extra)
		if err == nil && strings.EqualFold(eu.Hostname(), host) {
			return true
		}
	}
	return false
}

func parseFiltered(raw []byte) ([]event.UsageEvent, int, int, bool, error) {
	var top map[string]any
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&top); err != nil || dec.Decode(new(any)) != io.EOF {
		return nil, 0, 0, false, fmt.Errorf("%w [reason=invalid_filtered_json]", errSchemaChanged)
	}
	var total int
	hasCount := false
	for _, field := range []string{"totalUsageEventsCount", "total_usage_events_count"} {
		if value, present := top[field]; present {
			n, err := strconv.ParseInt(adapter.FlexString(value), 10, strconv.IntSize)
			if err != nil || n < 0 || hasCount && int(n) != total {
				return nil, 0, 0, false, fmt.Errorf("%w [reason=invalid_declared_count]", errSchemaChanged)
			}
			total, hasCount = int(n), true
		}
	}
	listValue := adapter.Pick(top, "usageEventsDisplay", "usage_events_display", "usageEvents", "usage_events")
	list, listOK := listValue.([]any)
	if !listOK {
		// A missing or renamed list is not an authoritative zero, even if
		// totalUsageEventsCount happens to be zero.
		return nil, 0, 0, false, fmt.Errorf("%w [reason=missing_usage_events_array]", errSchemaChanged)
	}
	out := make([]event.UsageEvent, 0, len(list))
	for _, item := range list {
		m := adapter.AsMap(item)
		if m == nil {
			return nil, 0, 0, false, fmt.Errorf("%w [reason=invalid_filtered_item]", errSchemaChanged)
		}
		model := adapter.FlexString(adapter.Pick(m, "model", "modelIntent", "model_intent"))
		ts := parseAPITime(adapter.Pick(m, "timestamp"))
		usage := adapter.AsMap(adapter.Pick(m, "tokenUsage", "token_usage"))
		if usage == nil {
			usage = m
		}
		known := false
		for _, field := range []string{"inputTokens", "input_tokens", "cacheReadTokens", "cache_read_tokens", "cacheWriteTokens", "cache_write_tokens", "outputTokens", "output_tokens"} {
			if value, ok := usage[field]; ok && value != nil {
				n, err := strconv.ParseInt(adapter.FlexString(value), 10, 64)
				if err != nil || n < 0 {
					return nil, 0, 0, false, fmt.Errorf("%w [reason=invalid_filtered_meter]", errSchemaChanged)
				}
				known = true
			}
		}
		if !known {
			if metered, ok := adapter.Pick(m, "isTokenBasedCall", "is_token_based_call").(bool); ok && !metered {
				continue
			}
			return nil, 0, 0, false, fmt.Errorf("%w [reason=missing_filtered_meter]", errSchemaChanged)
		}
		e := mapTokens(model, adapter.FlexString(adapter.Pick(m, "conversationId", "conversation_id")), ts, usage, 0)
		if id := adapter.FlexString(adapter.Pick(m, "usageEventId", "usage_event_id", "requestId", "request_id")); id != "" {
			e.RequestID = filteredEventIDPrefix + id
		}
		out = append(out, e)
	}
	return out, len(list), total, hasCount, nil
}

func parseAggregated(raw []byte, now time.Time) ([]event.UsageEvent, bool) {
	var top map[string]any
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if dec.Decode(&top) != nil {
		return nil, false
	}
	rawList, present := top["aggregations"]
	if !present {
		return nil, false
	}
	list := asSlice(rawList)
	out := make([]event.UsageEvent, 0, len(list))
	for i, item := range list {
		m := adapter.AsMap(item)
		if m == nil {
			continue
		}
		model := adapter.FlexString(adapter.Pick(m, "modelIntent", "model_intent", "model"))
		ev := mapTokens(model, "", now, m, i)
		if ev.Miss+ev.CacheRead+ev.CacheCreate+ev.Output == 0 && model == "" {
			continue
		}
		out = append(out, ev)
	}
	return out, true
}

func mapTokens(model, session string, ts time.Time, usage map[string]any, i int) event.UsageEvent {
	miss := adapter.FlexInt(adapter.Pick(usage, "inputTokens", "input_tokens"))
	out := adapter.FlexInt(adapter.Pick(usage, "outputTokens", "output_tokens"))
	cw := adapter.FlexInt(adapter.Pick(usage, "cacheWriteTokens", "cache_write_tokens"))
	cr := adapter.FlexInt(adapter.Pick(usage, "cacheReadTokens", "cache_read_tokens"))
	id := fmt.Sprintf("cursor-api:%s:%d:%s:%d:%d:%d:%d:%d", session, ts.UnixMilli(), model, miss, cr, cw, out, i)
	return event.UsageEvent{
		Source:      "cursor",
		Vendor:      vendor.Lookup(model, ""),
		SessionID:   session,
		RequestID:   id,
		Model:       model,
		Timestamp:   ts,
		Miss:        miss,
		CacheRead:   cr,
		CacheCreate: cw,
		Output:      out,
		Quality:     event.QualityAuthoritative,
		Derivation:  event.DeriveProviderAPI,
		SkipRequest: true,
		Provenance:  "cursor_filtered_usage",
	}
}

func degradeAPI(events []event.UsageEvent) []event.UsageEvent {
	for i := range events {
		if events[i].Derivation == event.DeriveProviderAPI {
			events[i].Quality = event.QualityDegraded
		}
	}
	return events
}

func hasAccountUsage(events []event.UsageEvent) bool {
	for _, e := range events {
		if e.Derivation == event.DeriveProviderAPI || e.Miss != 0 || e.CacheRead != 0 || e.CacheCreate != 0 || e.Output != 0 {
			return true
		}
	}
	return false
}

func asSlice(v any) []any {
	s, _ := v.([]any)
	return s
}

func parseAPITime(v any) time.Time {
	n := adapter.FlexInt(v)
	if n <= 0 {
		if s := adapter.FlexString(v); s != "" {
			if ts, err := time.Parse(time.RFC3339Nano, s); err == nil {
				return ts
			}
			if ts, err := time.Parse(time.RFC3339, s); err == nil {
				return ts
			}
		}
		return time.Time{}
	}
	if n > 1e12 {
		return time.UnixMilli(n)
	}
	if n > 1e9 {
		return time.Unix(n, 0)
	}
	return time.UnixMilli(n)
}

var errExpiredAuth = adapter.ErrStatus{Code: http.StatusUnauthorized}
