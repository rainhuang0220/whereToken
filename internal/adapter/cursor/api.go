package cursor

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
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
	token := access
	events, err := a.fetchUsageWithToken(sourceRoot, token)
	if err != nil && adapter.IsUnauthorized(err) && refresh != "" {
		next, rerr := a.refreshAccessToken(refresh)
		if rerr != nil {
			return nil, errExpiredAuth
		}
		token = next
		events, err = a.fetchUsageWithToken(sourceRoot, token)
	}
	return events, err
}

func (a Adapter) fetchUsageWithToken(sourceRoot, token string) ([]event.UsageEvent, error) {
	now := a.now()
	start := now.AddDate(0, 0, -53*7).UnixMilli()
	end := now.UnixMilli()

	filtered, ferr := a.fetchFiltered(token, start, end)
	if hasTokenTotals(filtered) {
		return tagged(sourceRoot, filtered), ferr
	}
	if adapter.IsUnauthorized(ferr) {
		return nil, ferr
	}

	agg, aerr := a.fetchAggregated(token, start, end)
	if aerr == nil && hasTokenTotals(agg) {
		return tagged(sourceRoot, agg), nil
	}
	if ferr == nil {
		return tagged(sourceRoot, filtered), nil
	}
	return nil, ferr
}

func tagged(sourceRoot string, events []event.UsageEvent) []event.UsageEvent {
	for i := range events {
		events[i].SourceRoot = sourceRoot
	}
	return events
}

func (a Adapter) fetchFiltered(token string, start, end int64) ([]event.UsageEvent, error) {
	var out []event.UsageEvent
	var total, rawSeen int
	pageSize := usagePageSize
	for page := 1; page <= maxUsagePages; page++ {
		body := map[string]any{
			"teamId":    0,
			"startDate": start,
			"endDate":   end,
			"page":      page,
			"pageSize":  usagePageSize,
		}
		raw, err := a.postJSON(filteredRPC, token, body)
		if err != nil {
			if len(out) > 0 {
				return out, err
			}
			return nil, err
		}
		pageEvents, rawN, pageTotal, err := parseFiltered(raw)
		if err != nil {
			if len(out) > 0 {
				return out, err
			}
			return nil, err
		}
		if page == 1 {
			total = pageTotal
			if rawN > 0 && rawN < usagePageSize {
				pageSize = rawN
			}
		}
		out = append(out, pageEvents...)
		rawSeen += rawN
		if rawN == 0 {
			break
		}
		if total > 0 && rawSeen >= total {
			break
		}
		if rawN < pageSize {
			break
		}
		if page == maxUsagePages && (total == 0 || rawSeen < total) {
			return out, fmt.Errorf("只拉了前 %d 页账号用量", maxUsagePages)
		}
	}
	return out, nil
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
	return parseAggregated(raw, a.now()), nil
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
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := a.client().Do(req)
	if err != nil {
		return nil, fmt.Errorf("账号用量接口请求失败")
	}
	defer resp.Body.Close()
	raw, _ := adapter.ReadHTTPBody(resp.Body)
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

func parseFiltered(raw []byte) ([]event.UsageEvent, int, int, error) {
	var top map[string]any
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&top); err != nil {
		return nil, 0, 0, fmt.Errorf("账号用量接口返回无法解析")
	}
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
	return out, len(list), total, nil
}

func parseAggregated(raw []byte, now time.Time) []event.UsageEvent {
	var top map[string]any
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if dec.Decode(&top) != nil {
		return nil
	}
	list := asSlice(adapter.Pick(top, "aggregations"))
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
	return out
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

func hasTokenTotals(events []event.UsageEvent) bool {
	for _, e := range events {
		if e.Miss != 0 || e.CacheRead != 0 || e.CacheCreate != 0 || e.Output != 0 {
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
