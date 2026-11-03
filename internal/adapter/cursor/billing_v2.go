package cursor

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/rainhuang0220/whereToken/internal/event"
	"github.com/rainhuang0220/whereToken/internal/vendor"
)

const billingV2Path = "/api/dashboard/v2/query-billing-items"
const billingV2PageSize = 100
const billingV2MaxPages = 500

// Cursor can return an availability-only metadata envelope for a time range
// without returning the requested usage rows. That is unknown coverage,
// never zero usage or a completed billing window.
var errBillingAvailability = fmt.Errorf("%w [reason=billing_range_availability_only]", errEndpointUnavailable)

// This is a compatibility path for the personal Cursor web dashboard, which
// moved from the DashboardService filtered event RPC in late 2026. It is not
// Cursor's public team Admin API. Never persist or log its session cookie.
func (a Adapter) fetchBillingWindow(token string, w usageSpan) (filteredFetch, error) {
	var out filteredFetch
	userID, jwt, ok := billingSessionIdentity(token)
	if !ok {
		return out, errTokenRejected
	}
	next := ""
	seenCursor := map[string]bool{}
	seenItems := map[string]bool{}
	// Billing v2 has been observed to return a different object for
	// truncated historical ranges. Query whole UTC months, then trim the
	// returned events to the caller's safe requested interval.
	queryFrom := time.Date(w.start.Year(), w.start.Month(), 1, 0, 0, 0, 0, time.UTC)
	// Cursor's personal v2 endpoint has been observed returning only an
	// availability envelope for a closed 31-day month ending at the next
	// midnight (or at 23:59:59.999). The full month ending at 23:59:59
	// UTC was successfully paged in a live acceptance probe. This endpoint
	// appears to use second-granularity bounds, so the last fractional
	// second cannot be independently proven by the response contract.
	queryTo := w.end
	if !w.inclusive {
		queryTo = queryTo.Add(-time.Second)
	}
	for page := 1; page <= billingV2MaxPages; page++ {
		q := url.Values{}
		q.Set("scope", "user")
		q.Set("startTime", queryFrom.UTC().Format(time.RFC3339Nano))
		q.Set("endTime", queryTo.UTC().Format(time.RFC3339Nano))
		q.Set("pageSize", strconv.Itoa(billingV2PageSize))
		if next != "" {
			q.Set("cursor", next)
		}
		base := "https://cursor.com"
		if a.APIBase != "" {
			base = strings.TrimRight(a.APIBase, "/")
		}
		endpoint := base + billingV2Path + "?" + q.Encode()
		req, err := http.NewRequest(http.MethodGet, endpoint, nil)
		if err != nil {
			return filteredFetch{}, errEndpointUnavailable
		}
		req.Header.Set("Cookie", "WorkosCursorSessionToken="+url.QueryEscape(userID+"::"+jwt))
		req.Header.Set("Accept", "application/json")
		c := a.client()
		// Cookie-bearing requests must not follow redirects of any kind.
		c.CheckRedirect = func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }
		resp, err := c.Do(req)
		if err != nil {
			return filteredFetch{}, errEndpointUnavailable
		}
		limited, readErr := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
		resp.Body.Close()
		if readErr != nil {
			return filteredFetch{}, errEndpointUnavailable
		}
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden || (resp.StatusCode >= 300 && resp.StatusCode < 400) {
			return filteredFetch{}, errTokenRejected
		}
		if resp.StatusCode == http.StatusTooManyRequests {
			return filteredFetch{}, errRateLimited
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return filteredFetch{}, errEndpointUnavailable
		}
		part, raw, after, err := decodeBillingPage(limited, userID, w, queryFrom)
		if err != nil {
			return filteredFetch{}, err
		}
		out.raw += raw
		for _, e := range part {
			if seenItems[e.RequestID] {
				continue
			}
			seenItems[e.RequestID] = true
			out.events = append(out.events, e)
		}
		if after == "" {
			// The new API offers no declared total. -1 records "not provided",
			// rather than falsely attributing a measured count to its server.
			out.declared = -1
			return out, nil
		}
		if seenCursor[after] {
			return filteredFetch{}, fmt.Errorf("%w [reason=repeated_billing_cursor]", errPaginationIncomplete)
		}
		seenCursor[after] = true
		next = after
	}
	return filteredFetch{}, fmt.Errorf("%w [reason=billing_page_limit]", errPaginationIncomplete)
}

func billingSessionIdentity(token string) (userID, jwt string, ok bool) {
	jwt = jwtBody(token)
	if !usableSessionToken(jwt) {
		return "", "", false
	}
	sub := JWTSubject(jwt)
	if i := strings.LastIndex(sub, "|"); i >= 0 {
		sub = sub[i+1:]
	}
	if !strings.HasPrefix(sub, "user_") || len(sub) > 150 || strings.ContainsAny(sub, "\r\n;= \t") {
		return "", "", false
	}
	return sub, jwt, true
}

type billingPage struct {
	Data       []json.RawMessage `json:"data"`
	Pagination *struct {
		NextCursor string `json:"nextCursor"`
	} `json:"pagination"`
}

func decodeBillingPage(raw []byte, userID string, w usageSpan, queryFrom time.Time) ([]event.UsageEvent, int, string, error) {
	var obj map[string]json.RawMessage
	if json.Unmarshal(raw, &obj) != nil {
		return nil, 0, "", fmt.Errorf("%w [reason=invalid_billing_json]", errSchemaChanged)
	}
	data, found := obj["data"]
	if !found {
		if _, ok := obj["availability"]; ok && len(obj) == 1 {
			return nil, 0, "", errBillingAvailability
		}
		// Only schema key names are disclosed, never error text, IDs,
		// response bodies or anything from the account.
		if len(obj) == 0 {
			return nil, 0, "", fmt.Errorf("%w [reason=empty_billing_object]", errSchemaChanged)
		}
		var names []string
		for key := range obj {
			if len(key) == 0 || len(key) > 48 {
				continue
			}
			valid := true
			for _, ch := range key {
				if !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_' || ch == '-') {
					valid = false
					break
				}
			}
			if valid {
				names = append(names, key)
			}
		}
		sort.Strings(names)
		return nil, 0, "", fmt.Errorf("%w [reason=missing_billing_data schema_keys=%s field_count=%d]", errSchemaChanged, strings.Join(names, ","), len(obj))
	}
	var list []json.RawMessage
	if json.Unmarshal(data, &list) != nil || list == nil {
		return nil, 0, "", fmt.Errorf("%w [reason=invalid_billing_data]", errSchemaChanged)
	}
	var pagination struct {
		NextCursor string `json:"nextCursor"`
	}
	p, hasPagination := obj["pagination"]
	if !hasPagination || string(p) == "null" || json.Unmarshal(p, &pagination) != nil {
		return nil, 0, "", fmt.Errorf("%w [reason=missing_or_invalid_billing_pagination]", errPaginationIncomplete)
	}
	// A cursor must have an explicit nextCursor field even on the last
	// page; otherwise a truncated response could masquerade as complete.
	var paginationKeys map[string]json.RawMessage
	if json.Unmarshal(p, &paginationKeys) != nil {
		return nil, 0, "", fmt.Errorf("%w [reason=invalid_billing_pagination]", errSchemaChanged)
	}
	if _, ok := paginationKeys["nextCursor"]; !ok {
		return nil, 0, "", fmt.Errorf("%w [reason=missing_billing_next_cursor]", errPaginationIncomplete)
	}
	events := make([]event.UsageEvent, 0, len(list))
	for _, item := range list {
		var row struct {
			BillingItemID string `json:"billingItemId"`
			EventTime     string `json:"eventTime"`
			Principal     struct {
				Type string `json:"type"`
				ID   string `json:"id"`
			} `json:"principal"`
			HasTokenMeters bool                       `json:"hasTokenMeters"`
			Usage          map[string]json.RawMessage `json:"usage"`
			TotalTokens    json.RawMessage            `json:"totalTokens"`
			DisplayExtras  struct {
				Model string `json:"model"`
			} `json:"displayExtras"`
		}
		if json.Unmarshal(item, &row) != nil {
			return nil, 0, "", fmt.Errorf("%w [reason=invalid_billing_item]", errSchemaChanged)
		}
		if row.Principal.Type == "user" && row.Principal.ID != "" && row.Principal.ID != userID {
			return nil, 0, "", errAccountMismatch
		}
		if !row.HasTokenMeters && len(row.Usage) == 0 {
			continue
		}
		if row.BillingItemID == "" {
			return nil, 0, "", fmt.Errorf("%w [reason=missing_billing_item_id]", errSchemaChanged)
		}
		stamp, err := time.Parse(time.RFC3339Nano, row.EventTime)
		if err != nil {
			return nil, 0, "", fmt.Errorf("%w [reason=invalid_billing_time]", errSchemaChanged)
		}
		stamp = stamp.UTC()
		if stamp.Before(w.start) {
			if stamp.Before(queryFrom) {
				return nil, 0, "", fmt.Errorf("%w [reason=billing_event_outside_query]", errSchemaChanged)
			}
			continue // historical prefix is retained from the verified snapshot
		}
		if (w.inclusive && stamp.After(w.end)) || (!w.inclusive && !stamp.Before(w.end)) {
			return nil, 0, "", fmt.Errorf("%w [reason=billing_event_outside_window]", errSchemaChanged)
		}
		values := []string{"prompt-text-tokens", "cached-prompt-text-tokens", "cache-write-text-tokens", "completion-text-tokens"}
		n := make([]int64, 4)
		known := false
		for i, k := range values {
			v, found := row.Usage[k]
			if !found {
				continue
			}
			known = true
			val, err := parseBillingInt(v)
			if err != nil {
				return nil, 0, "", fmt.Errorf("%w [reason=invalid_billing_meter]", errSchemaChanged)
			}
			n[i] = val
		}
		if !known {
			return nil, 0, "", fmt.Errorf("%w [reason=unknown_billing_meters]", errSchemaChanged)
		}
		if len(row.TotalTokens) > 0 && string(row.TotalTokens) != "null" {
			total, err := parseBillingInt(row.TotalTokens)
			if err != nil || total != n[0]+n[1]+n[2]+n[3] {
				return nil, 0, "", fmt.Errorf("%w [reason=billing_total_mismatch]", errSchemaChanged)
			}
		}
		hash := sha256.Sum256([]byte(row.BillingItemID))
		events = append(events, event.UsageEvent{
			Source: "cursor", Vendor: vendor.Lookup(row.DisplayExtras.Model, ""),
			RequestID: "cursor-billing-v2:" + hex.EncodeToString(hash[:16]),
			Model:     row.DisplayExtras.Model, Timestamp: stamp,
			Miss: n[0], CacheRead: n[1], CacheCreate: n[2], Output: n[3],
			Quality: event.QualityAuthoritative, Derivation: event.DeriveProviderAPI, SkipRequest: true,
		})
	}
	return events, len(list), pagination.NextCursor, nil
}

func parseBillingInt(raw json.RawMessage) (int64, error) {
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		s = string(raw)
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("invalid numeric meter")
	}
	return n, nil
}
