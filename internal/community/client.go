package community

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
)

const defaultTimeout = 2 * time.Second

// Client talks to a Community Rank service. Missing URL, offline, or any
// network error become an unavailable standing. Local analytics must not fail.
type Client struct {
	BaseURL  string
	HTTP     *http.Client
	File     *File
	Path     string
	Offline  bool
	Version  string
	MinCache time.Duration

	mu         sync.Mutex
	lastAgg    LocalAgg
	lastView   View
	lastAt     time.Time
	haveCached bool
}

func (c *Client) httpc() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return &http.Client{Timeout: defaultTimeout}
}

func (c *Client) Sync(ctx context.Context, events []event.UsageEvent, now time.Time, loc *time.Location) View {
	agg := BuildLocalAgg(events, now, loc)
	return c.SyncAgg(ctx, agg)
}

func (c *Client) SyncAgg(ctx context.Context, agg LocalAgg) View {
	if c == nil || c.File == nil || !c.File.Enabled {
		v := EmptyView(StatusOptedOut, DisclaimerEN)
		v.Enabled = false
		return v
	}
	if c.Offline {
		return EmptyView(StatusOffline, DisclaimerEN)
	}
	if strings.TrimSpace(c.BaseURL) == "" {
		return EmptyView(StatusServiceUnconfigured, "Community Rank service is not configured.")
	}
	if agg.TodayTokens <= 0 && agg.TodayCostUSD == nil {
		// still try all-time if they uploaded before
	}

	c.mu.Lock()
	cacheFor := c.MinCache
	if cacheFor == 0 {
		cacheFor = 5 * time.Minute
	}
	if c.haveCached && time.Since(c.lastAt) < cacheFor && agg.Equal(c.lastAgg) {
		v := c.lastView
		c.mu.Unlock()
		return v
	}
	c.mu.Unlock()

	view := View{
		Enabled:      true,
		Metric:       MetricTokens,
		SelfReported: true,
		Note:         DisclaimerEN,
	}

	if agg.TodayTokens > 0 {
		if u, err := MakeUpload(c.File.ParticipantID, sanitizeVersion(c.Version), agg); err == nil {
			if err := c.put(ctx, u); err != nil {
				view.Today = Standing{Status: StatusNetworkError, Period: PeriodToday, Metric: MetricTokens, SelfReported: true, Note: DisclaimerEN}
				view.All = Standing{Status: StatusNetworkError, Period: PeriodAll, Metric: MetricTokens, SelfReported: true, Note: DisclaimerEN}
				return view
			}
		} else {
			view.Today = Standing{Status: StatusUnavailable, Period: PeriodToday, Metric: MetricTokens, SelfReported: true, Note: DisclaimerEN}
			view.All = view.Today
			view.All.Period = PeriodAll
			return view
		}
	}

	view.Today = c.get(ctx, agg.LocalDate, MetricTokens)
	view.Today.Period = PeriodToday
	view.All = c.get(ctx, PeriodAll, MetricTokens)
	view.All.Period = PeriodAll

	c.mu.Lock()
	c.lastAgg = agg
	c.lastView = view
	c.lastAt = time.Now()
	c.haveCached = true
	c.mu.Unlock()
	return view
}

func sanitizeVersion(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		v = "dev"
	}
	if len(v) > MaxVersionLen {
		v = v[:MaxVersionLen]
	}
	if !versionRe.MatchString(v) {
		return "dev"
	}
	return v
}

func (c *Client) put(ctx context.Context, u Upload) error {
	raw, err := json.Marshal(u)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/v1/community/usage", bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	res, err := c.httpc().Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 4096))
	if res.StatusCode >= 300 {
		return fmt.Errorf("community upload %d", res.StatusCode)
	}
	return nil
}

func (c *Client) get(ctx context.Context, period, metric string) Standing {
	q := url.Values{}
	q.Set("participant_id", c.File.ParticipantID)
	q.Set("period", period)
	q.Set("metric", metric)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/v1/community/rank?"+q.Encode(), nil)
	if err != nil {
		return Standing{Status: StatusNetworkError, Period: period, Metric: metric, SelfReported: true, Note: DisclaimerEN}
	}
	req.Header.Set("Accept", "application/json")
	res, err := c.httpc().Do(req)
	if err != nil {
		return Standing{Status: StatusNetworkError, Period: period, Metric: metric, SelfReported: true, Note: DisclaimerEN}
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 32<<10))
	if err != nil {
		return Standing{Status: StatusNetworkError, Period: period, Metric: metric, SelfReported: true, Note: DisclaimerEN}
	}
	if res.StatusCode >= 300 {
		return Standing{Status: StatusNetworkError, Period: period, Metric: metric, SelfReported: true, Note: DisclaimerEN}
	}
	var st Standing
	if err := json.Unmarshal(body, &st); err != nil {
		return Standing{Status: StatusNetworkError, Period: period, Metric: metric, SelfReported: true, Note: DisclaimerEN}
	}
	if st.Status == "" {
		st.Status = StatusUnavailable
	}
	st.Period = period
	st.Metric = metric
	return SanitizeStanding(st)
}

func (c *Client) Leave(ctx context.Context) error {
	if c == nil || c.File == nil || strings.TrimSpace(c.BaseURL) == "" || c.Offline {
		return nil
	}
	raw, err := json.Marshal(map[string]string{"participant_id": c.File.ParticipantID})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/v1/community/leave", bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := c.httpc().Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 4096))
	if res.StatusCode >= 300 {
		return fmt.Errorf("community leave %d", res.StatusCode)
	}
	return nil
}
