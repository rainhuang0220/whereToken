package trae

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/rainhuang0220/whereToken/internal/event"
	"github.com/rainhuang0220/whereToken/internal/vendor"
)

const (
	sessionUsagePath      = "/api/v1/commercial/get_session_usage"
	defaultAPICN          = "https://trae-api-cn.mchost.guru"
	defaultAPISG          = "https://coresg-normal.trae.ai"
	maxSessions           = 500
	defaultSessionWorkers = 8
	defaultFetchBudget    = 30 * time.Second
)

func (a Adapter) fetchBudget() time.Duration {
	if a.FetchBudget > 0 {
		return a.FetchBudget
	}
	return defaultFetchBudget
}

func (a Adapter) fetchAccountUsage(sourceRoot, authPath, token string, sessions []string) ([]event.UsageEvent, error) {
	truncated := false
	if len(sessions) > maxSessions {
		sessions = sessions[:maxSessions]
		truncated = true
	}
	if len(sessions) == 0 {
		return nil, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), a.fetchBudget())
	defer cancel()

	jobs := make(chan string)
	var mu sync.Mutex
	var out []event.UsageEvent
	var lastErr error
	var unauth error
	var wg sync.WaitGroup
	workers := defaultSessionWorkers
	if workers > len(sessions) {
		workers = len(sessions)
	}
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for id := range jobs {
				if ctx.Err() != nil {
					return
				}
				raw, err := a.postJSON(ctx, sessionUsagePath, token, map[string]any{"session_id": id}, sourceRoot, authPath)
				mu.Lock()
				if err != nil {
					if isUnauthorized(err) {
						unauth = err
						mu.Unlock()
						cancel()
						return
					}
					lastErr = err
					mu.Unlock()
					continue
				}
				out = append(out, parseSessionUsage(raw, sourceRoot)...)
				mu.Unlock()
			}
		}()
	}
	go func() {
		defer close(jobs)
		for _, id := range sessions {
			select {
			case jobs <- id:
			case <-ctx.Done():
				return
			}
		}
	}()
	wg.Wait()
	if unauth != nil {
		return nil, unauth
	}
	if ctx.Err() == context.DeadlineExceeded {
		lastErr = fmt.Errorf("账号用量接口超时")
	}
	if len(out) == 0 && lastErr != nil {
		return nil, lastErr
	}
	if truncated {
		return out, fmt.Errorf("只拉了前 %d 个会话", maxSessions)
	}
	if lastErr != nil {
		return out, lastErr
	}
	return out, nil
}

func parseSessionUsage(raw []byte, sourceRoot string) []event.UsageEvent {
	var top any
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&top); err != nil {
		return nil
	}
	var out []event.UsageEvent
	seq := 0
	walkUsage(top, sourceRoot, &out, &seq)
	return out
}

func walkUsage(v any, sourceRoot string, out *[]event.UsageEvent, seq *int) {
	switch x := v.(type) {
	case map[string]any:
		nested := pick(x, "user_usage_group_by_session", "data")
		if nested != nil {
			walkUsage(nested, sourceRoot, out, seq)
		} else if ev, ok := usageFromMap(x, sourceRoot); ok {
			*seq++
			ev.RequestID = fmt.Sprintf("%s:%d", ev.RequestID, *seq)
			*out = append(*out, ev)
		}
		for k, child := range x {
			if k == "user_usage_group_by_session" || k == "data" || k == "extra_info" || k == "usage" {
				continue
			}
			if _, isMap := child.(map[string]any); isMap {
				walkUsage(child, sourceRoot, out, seq)
			}
			if _, isSlice := child.([]any); isSlice {
				walkUsage(child, sourceRoot, out, seq)
			}
		}
	case []any:
		for _, child := range x {
			walkUsage(child, sourceRoot, out, seq)
		}
	}
}

func usageFromMap(m map[string]any, sourceRoot string) (event.UsageEvent, bool) {
	extra := asMap(pick(m, "extra_info", "extraInfo"))
	usage := asMap(pick(m, "usage"))
	blob := m
	if extra != nil {
		blob = extra
	} else if usage != nil {
		blob = usage
	} else if pick(m, "input_token", "prompt_tokens", "input_tokens") == nil {
		return event.UsageEvent{}, false
	}
	input := flexInt(pick(blob, "input_token", "prompt_tokens", "input_tokens", "inputTokens"))
	output := flexInt(pick(blob, "output_token", "completion_tokens", "output_tokens", "outputTokens"))
	cacheRead := flexInt(pick(blob, "cache_read_token", "cache_read_input_tokens", "cached_tokens", "cacheReadTokens"))
	if cacheRead == 0 {
		if details := asMap(pick(blob, "prompt_tokens_details", "input_tokens_details")); details != nil {
			cacheRead = flexInt(pick(details, "cached_tokens", "cache_read_tokens"))
		}
	}
	cacheCreate := flexInt(pick(blob, "cache_write_token", "cache_creation_input_tokens", "cacheWriteTokens"))
	if input == 0 && output == 0 && cacheRead == 0 && cacheCreate == 0 {
		return event.UsageEvent{}, false
	}
	miss := input - cacheRead
	if miss < 0 {
		miss = 0
	}
	model := flexString(pick(m, "model_name", "modelName", "model"))
	if model == "" {
		model = flexString(pick(blob, "model_name", "modelName", "model"))
	}
	sess := flexString(pick(m, "session_id", "sessionId"))
	id := sess
	if id == "" {
		id = fmt.Sprintf("%d:%s", miss+cacheRead+cacheCreate+output, model)
	}
	return event.UsageEvent{
		Source:      "trae",
		Vendor:      vendor.Lookup(model, ""),
		SourceRoot:  sourceRoot,
		SessionID:   sess,
		RequestID:   "trae-api:" + id,
		Model:       model,
		Miss:        miss,
		CacheRead:   cacheRead,
		CacheCreate: cacheCreate,
		Output:      output,
		Quality:     event.QualityAuthoritative,
	}, true
}

func (a Adapter) postJSON(ctx context.Context, path, token string, payload any, hints ...string) ([]byte, error) {
	base := a.apiBase(hints...)
	u, err := url.Parse(base + path)
	if err != nil {
		return nil, fmt.Errorf("账号用量接口地址无效")
	}
	if !a.allowedURL(u) {
		return nil, fmt.Errorf("拒绝访问非 Trae 主机")
	}
	buf, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(buf))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Cloud-IDE-JWT "+token)
		req.Header.Set("X-Cloudide-Token", token)
	}
	resp, err := a.client().Do(req)
	if err != nil {
		return nil, fmt.Errorf("账号用量接口请求失败")
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return nil, errStatus{code: resp.StatusCode}
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, errStatus{code: resp.StatusCode}
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
		return fmt.Errorf("拒绝访问非 Trae 主机")
	}
	return nil
}

func (a Adapter) now() time.Time {
	if a.Now != nil {
		return a.Now()
	}
	return time.Now()
}

func (a Adapter) apiBase(hints ...string) string {
	if strings.TrimSpace(a.APIBase) != "" {
		return strings.TrimRight(a.APIBase, "/")
	}
	blob := strings.ToLower(strings.Join(hints, "\n"))
	if strings.Contains(blob, "cn") {
		return defaultAPICN
	}
	return defaultAPISG
}

func (a Adapter) allowedURL(u *url.URL) bool {
	if u == nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	switch host {
	case "trae-api-cn.mchost.guru", "api.trae.cn", "api.trae.ai",
		"coresg-normal.trae.ai", "coreva-normal.trae.ai", "api-sg-central.trae.ai",
		"api5-normal.mchost.guru", "api3-normal.mchost.guru":
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

func pick(m map[string]any, names ...string) any {
	if m == nil {
		return nil
	}
	for _, n := range names {
		if v, ok := m[n]; ok && v != nil {
			return v
		}
	}
	return nil
}

func asMap(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}

func flexString(v any) string {
	switch x := v.(type) {
	case string:
		return strings.TrimSpace(x)
	case json.Number:
		return x.String()
	default:
		return ""
	}
}

func flexInt(v any) int64 {
	switch x := v.(type) {
	case nil:
		return 0
	case json.Number:
		n, err := x.Int64()
		if err == nil {
			return n
		}
		f, err := x.Float64()
		if err == nil {
			return int64(f)
		}
		return 0
	case float64:
		return int64(x)
	case int64:
		return x
	case int:
		return int64(x)
	case string:
		s := strings.TrimSpace(x)
		if s == "" {
			return 0
		}
		var n int64
		fmt.Sscan(s, &n)
		return n
	default:
		return 0
	}
}

type errStatus struct{ code int }

func (e errStatus) Error() string {
	if e.code == http.StatusUnauthorized || e.code == http.StatusForbidden {
		return "本机登录态已失效"
	}
	return fmt.Sprintf("账号用量接口 HTTP %d", e.code)
}

func isUnauthorized(err error) bool {
	es, ok := err.(errStatus)
	return ok && (es.code == http.StatusUnauthorized || es.code == http.StatusForbidden)
}
