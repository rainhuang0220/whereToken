package cursor

import (
	"bytes"
	"encoding/json"
	"fmt"
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
		next, rerr := a.refreshAccessToken(refresh)
		if rerr != nil || !usableSessionToken(next) {
			if len(events) > 0 {
				return events, errExpiredAuth
			}
			return nil, errExpiredAuth
		}
		retried, rerr := a.fetchUsageWithToken(sourceRoot, next)
		if rerr != nil && len(retried) == 0 && len(events) > 0 {
			return events, rerr
		}
		return retried, rerr
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
	now := a.now().UTC()
	if a.Probe {
		start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
		if !start.Before(now) {
			start = now
		}
		return []usageSpan{{start: start, end: now, inclusive: true}}
	}
	earliest := now.AddDate(0, 0, -53*7)
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

func (a Adapter) fetchUsageWithToken(sourceRoot, token string) ([]event.UsageEvent, error) {
	clearCoverage(sourceRoot)
	windows := a.usageWindows()
	seen := map[string]struct{}{}
	var out []event.UsageEvent
	var covered []CoverageWindow
	missing := 0
	var failed error
	for _, w := range windows {
		evs, err := a.fetchFilteredWindow(token, w, 0)
		if err == nil {
			out = dedupeUsage(out, seen, evs)
			covered = append(covered, w.coverage())
			continue
		}
		if isNotFound(err) {
			missing++
			continue
		}
		failed = err
		break
	}
	if len(covered) == 0 && missing == len(windows) && failed == nil && len(windows) > 0 {
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
	}
	if failed != nil {
		if len(out) == 0 {
			return nil, failed
		}
		return tagged(sourceRoot, out), failed
	}
	if missing > 0 && len(covered) > 0 {
		if len(out) == 0 {
			return nil, errEndpointUnavailable
		}
		return tagged(sourceRoot, out), errEndpointUnavailable
	}
	if len(covered) == len(windows) && len(out) == 0 {
		return tagged(sourceRoot, completeAccountUsage(nil, a.now())), nil
	}
	return tagged(sourceRoot, out), nil
}

func (a Adapter) fetchFilteredWindow(token string, w usageSpan, depth int) ([]event.UsageEvent, error) {
	endMs := w.end.UnixMilli()
	if !w.inclusive && endMs > w.start.UnixMilli() {
		endMs--
	}
	evs, err := a.fetchFiltered(token, w.start.UnixMilli(), endMs)
	if err == nil || err != errUsagePageLimit || depth >= 6 || w.end.Sub(w.start) < time.Hour {
		if err == errUsagePageLimit && (depth >= 6 || w.end.Sub(w.start) < time.Hour) {
			return nil, errPaginationIncomplete
		}
		return evs, err
	}
	mid := w.start.Add(w.end.Sub(w.start) / 2)
	left, lerr := a.fetchFilteredWindow(token, usageSpan{start: w.start, end: mid, inclusive: false}, depth+1)
	if lerr != nil {
		return nil, lerr
	}
	right, rerr := a.fetchFilteredWindow(token, usageSpan{start: mid, end: w.end, inclusive: w.inclusive}, depth+1)
	if rerr != nil {
		return nil, rerr
	}
	return append(left, right...), nil
}

func dedupeUsage(dst []event.UsageEvent, seen map[string]struct{}, add []event.UsageEvent) []event.UsageEvent {
	for _, e := range add {
		if e.RequestID != "" {
			if _, ok := seen[e.RequestID]; ok {
				continue
			}
			seen[e.RequestID] = struct{}{}
		}
		dst = append(dst, e)
	}
	return dst
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

func (a Adapter) fetchFiltered(token string, start, end int64) ([]event.UsageEvent, error) {
	var out []event.UsageEvent
	var total, rawSeen int
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
			return nil, err
		}
		pageEvents, rawN, pageTotal, known, err := parseFiltered(raw)
		if err != nil {
			return nil, err
		}
		if !known {
			return nil, errSchemaChanged
		}
		if page == 1 {
			total = pageTotal
			if rawN > 0 && rawN < full {
				pageSize = rawN
			}
		} else if pageTotal != total {
			return nil, errSchemaChanged
		}
		if rawSeen+rawN > total {
			return nil, errSchemaChanged
		}
		out = append(out, pageEvents...)
		rawSeen += rawN
		if rawSeen == total {
			return out, nil
		}
		if rawN == 0 || rawN < pageSize {
			return nil, errPaginationIncomplete
		}
	}
	if pageSize < full {
		return nil, errPaginationIncomplete
	}
	return nil, errUsagePageLimit
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
		return nil, fmt.Errorf("账号用量接口返回无法解析")
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
	req, err := http.NewRequest(http.MethodPost, u.String(), bytes.NewReader(buf))
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
		return nil, fmt.Errorf("账号用量接口请求失败")
	}
	defer resp.Body.Close()
	raw, _ := adapter.ReadHTTPBody(resp.Body)
	if resp.StatusCode == http.StatusTooManyRequests {
		if retry {
			if d, ok := retryAfterWait(resp.Header); ok {
				time.Sleep(d)
				return a.postJSONOnce(path, token, payload, false)
			}
		}
		return nil, adapter.ErrStatus{Code: resp.StatusCode}
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return nil, adapter.ErrStatus{Code: resp.StatusCode}
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil, adapter.ErrStatus{Code: resp.StatusCode}
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, adapter.ErrStatus{Code: resp.StatusCode}
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
	if err := dec.Decode(&top); err != nil {
		return nil, 0, 0, false, fmt.Errorf("账号用量接口返回无法解析")
	}
	_, hasCamel := top["totalUsageEventsCount"]
	_, hasSnake := top["total_usage_events_count"]
	total := int(adapter.FlexInt(adapter.Pick(top, "totalUsageEventsCount", "total_usage_events_count")))
	list := asSlice(adapter.Pick(top, "usageEventsDisplay", "usage_events_display", "usageEvents", "usage_events"))
	out := make([]event.UsageEvent, 0, len(list))
	for i, item := range list {
		m := adapter.AsMap(item)
		if m == nil {
			continue
		}
		model := adapter.FlexString(adapter.Pick(m, "model", "modelIntent", "model_intent"))
		ts := parseAPITime(adapter.Pick(m, "timestamp"))
		usage := adapter.AsMap(adapter.Pick(m, "tokenUsage", "token_usage"))
		if usage == nil {
			usage = m
		}
		ev := mapTokens(model, adapter.FlexString(adapter.Pick(m, "conversationId", "conversation_id")), ts, usage, i)
		if ev.Model != "" || ev.Miss+ev.CacheRead+ev.CacheCreate+ev.Output != 0 {
			out = append(out, ev)
		}
	}
	return out, len(list), total, hasCamel || hasSnake, nil
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
	}
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
