package cursor

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/rainhuang0220/whereToken/internal/event"
	"github.com/rainhuang0220/whereToken/internal/vendor"
)

const dashboardCSVPath = "/api/dashboard/export-usage-events-csv"
const maxDashboardCSVBytes = 64 << 20

// fetchCSVWindow uses the signed-in dashboard identity. CSV rows have no
// request ids: these events can prove token attribution, not request counts.
func (a Adapter) fetchCSVWindow(token string, w usageSpan) (filteredFetch, error) {
	userID, jwt, ok := billingSessionIdentity(token)
	if !ok {
		return filteredFetch{}, errTokenRejected
	}
	end := w.end.UnixMilli()
	if !w.inclusive {
		end--
	}
	q := url.Values{}
	q.Set("startDate", strconv.FormatInt(w.start.UnixMilli(), 10))
	q.Set("endDate", strconv.FormatInt(end, 10))
	q.Set("strategy", "tokens")
	base := "https://cursor.com"
	if a.APIBase != "" {
		base = strings.TrimRight(a.APIBase, "/")
	}
	req, err := http.NewRequest(http.MethodGet, base+dashboardCSVPath+"?"+q.Encode(), nil)
	if err != nil {
		return filteredFetch{}, fmt.Errorf("%w [endpoint=dashboard_csv phase=request_build]", errEndpointUnavailable)
	}
	req.Header.Set("Cookie", "WorkosCursorSessionToken="+url.QueryEscape(userID+"::"+jwt))
	req.Header.Set("Accept", "text/csv")
	c := *a.client()
	if c.Timeout <= 0 {
		c.Timeout = 20 * time.Second
	}
	// Clone the client and reject every redirect, including same-host ones.
	// A login redirect is not an independently authenticated usage export.
	c.CheckRedirect = func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }
	resp, err := c.Do(req)
	if err != nil {
		return filteredFetch{}, fmt.Errorf("%w [endpoint=dashboard_csv phase=network]", errEndpointUnavailable)
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden || resp.StatusCode >= 300 && resp.StatusCode < 400:
		return filteredFetch{}, errTokenRejected
	case resp.StatusCode == http.StatusTooManyRequests:
		return filteredFetch{}, errRateLimited
	case resp.StatusCode < 200 || resp.StatusCode >= 300:
		return filteredFetch{}, fmt.Errorf("%w [endpoint=dashboard_csv phase=http status=%d]", errEndpointUnavailable, resp.StatusCode)
	}
	return decodeUsageCSV(resp.Body, w)
}

// decodeUsageCSV accepts only a complete, bounded export with reconciled
// independent token components. Errors contain fixed schema labels, never
// CSV fields, body fragments, paths, or session material.
func decodeUsageCSV(r io.Reader, w usageSpan) (filteredFetch, error) {
	raw, err := io.ReadAll(io.LimitReader(r, maxDashboardCSVBytes+1))
	if err != nil {
		return filteredFetch{}, fmt.Errorf("%w [endpoint=dashboard_csv phase=body_read]", errEndpointUnavailable)
	}
	if len(raw) > maxDashboardCSVBytes {
		return filteredFetch{}, fmt.Errorf("%w [endpoint=dashboard_csv reason=body_limit]", errSchemaChanged)
	}
	reader := csv.NewReader(bytes.NewReader(raw))
	header, err := reader.Read()
	if err != nil {
		return filteredFetch{}, fmt.Errorf("%w [endpoint=dashboard_csv reason=invalid_header]", errSchemaChanged)
	}
	columns := map[string]int{}
	for i, name := range header {
		name = strings.TrimSpace(name)
		if i == 0 {
			name = strings.TrimPrefix(name, "\ufeff")
		}
		if _, duplicate := columns[name]; duplicate {
			return filteredFetch{}, fmt.Errorf("%w [endpoint=dashboard_csv reason=duplicate_header]", errSchemaChanged)
		}
		columns[name] = i
	}
	meters := [...]string{"Input (w/o Cache Write)", "Cache Read", "Input (w/ Cache Write)", "Output Tokens"}
	for _, name := range []string{"Date", "Model", meters[0], meters[1], meters[2], meters[3], "Total Tokens"} {
		if _, ok := columns[name]; !ok {
			return filteredFetch{}, fmt.Errorf("%w [endpoint=dashboard_csv reason=missing_header]", errSchemaChanged)
		}
	}
	out := filteredFetch{declared: -1}
	for {
		row, readErr := reader.Read()
		if readErr == io.EOF {
			return out, nil
		}
		if readErr != nil {
			return filteredFetch{}, fmt.Errorf("%w [endpoint=dashboard_csv reason=invalid_record]", errSchemaChanged)
		}
		out.raw++
		stamp, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(row[columns["Date"]]))
		if err != nil {
			return filteredFetch{}, fmt.Errorf("%w [endpoint=dashboard_csv reason=invalid_time row=%d]", errSchemaChanged, out.raw)
		}
		stamp = stamp.UTC()
		if stamp.Before(w.start) || stamp.After(w.end) || !w.inclusive && stamp.Equal(w.end) {
			return filteredFetch{}, fmt.Errorf("%w [endpoint=dashboard_csv reason=event_outside_window row=%d]", errSchemaChanged, out.raw)
		}
		var values [4]int64
		missing := 0
		var sum int64
		for i, name := range meters {
			text := strings.TrimSpace(row[columns[name]])
			if text == "" || text == "-" {
				missing++
				continue
			}
			n, parseErr := strconv.ParseInt(text, 10, 64)
			if parseErr != nil || n < 0 || n > math.MaxInt64-sum {
				return filteredFetch{}, fmt.Errorf("%w [endpoint=dashboard_csv reason=invalid_meter row=%d]", errSchemaChanged, out.raw)
			}
			values[i] = n
			sum += n
		}
		totalText := strings.TrimSpace(row[columns["Total Tokens"]])
		totalMissing := totalText == "" || totalText == "-"
		var total int64
		if !totalMissing {
			var parseErr error
			total, parseErr = strconv.ParseInt(totalText, 10, 64)
			if parseErr != nil || total < 0 {
				return filteredFetch{}, fmt.Errorf("%w [endpoint=dashboard_csv reason=invalid_total row=%d]", errSchemaChanged, out.raw)
			}
		}
		if missing == len(meters) && (totalMissing || total == 0) {
			// Unavailable meters are counted in raw, never measured as zero.
			continue
		}
		if missing != 0 || totalMissing || sum != total {
			return filteredFetch{}, fmt.Errorf("%w [endpoint=dashboard_csv reason=unreconciled_tokens row=%d]", errSchemaChanged, out.raw)
		}
		model := strings.TrimSpace(row[columns["Model"]])
		out.events = append(out.events, event.UsageEvent{
			Source: "cursor", Vendor: vendor.Lookup(model, ""), Model: model,
			Timestamp: stamp, Miss: values[0], CacheRead: values[1], CacheCreate: values[2], Output: values[3],
			Quality: event.QualityAuthoritative, Derivation: event.DeriveProviderAPI,
			Provenance: "cursor_dashboard_csv", SkipRequest: true,
		})
	}
}
