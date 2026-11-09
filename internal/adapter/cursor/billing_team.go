package cursor

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

var errTeamSelectionRequired = fmt.Errorf("%w [reason=team_selection_required]", errEndpointUnavailable)

// uniqueBillingTeamID resolves a team only when the logged-in account has
// exactly one valid membership. Never infer a team ID from an identity token
// or probe arbitrary IDs. The credential stays inside this process.
func (a Adapter) uniqueBillingTeamID(userID, jwt string) (int64, error) {
	base := "https://cursor.com"
	if a.APIBase != "" {
		base = strings.TrimRight(a.APIBase, "/")
	}
	req, err := http.NewRequest(http.MethodPost, base+"/api/dashboard/teams", bytes.NewBufferString("{}"))
	if err != nil {
		return 0, fmt.Errorf("%w [reason=team_lookup_request]", errEndpointUnavailable)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://cursor.com")
	req.Header.Set("Cookie", "WorkosCursorSessionToken="+url.QueryEscape(userID+"::"+jwt))
	c := *a.client()
	c.CheckRedirect = func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }
	resp, err := c.Do(req)
	if err != nil {
		return 0, fmt.Errorf("%w [reason=team_lookup_network]", errEndpointUnavailable)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden || (resp.StatusCode >= 300 && resp.StatusCode < 400) {
		return 0, errTokenRejected
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		return 0, errRateLimited
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return 0, fmt.Errorf("%w [reason=team_lookup_http status=%d]", errEndpointUnavailable, resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return 0, fmt.Errorf("%w [reason=team_lookup_read]", errEndpointUnavailable)
	}
	var envelope struct {
		Teams []struct {
			ID json.RawMessage `json:"id"`
		} `json:"teams"`
	}
	if json.Unmarshal(b, &envelope) != nil {
		return 0, fmt.Errorf("%w [reason=team_lookup_schema]", errSchemaChanged)
	}
	if len(envelope.Teams) != 1 {
		return 0, fmt.Errorf("%w [team_count=%d]", errTeamSelectionRequired, len(envelope.Teams))
	}
	var n int64
	val := envelope.Teams[0].ID
	if len(val) > 0 && val[0] == '"' {
		var str string
		if json.Unmarshal(val, &str) == nil {
			n, _ = strconv.ParseInt(str, 10, 64)
		}
	} else {
		n, _ = strconv.ParseInt(string(val), 10, 64)
	}
	if n <= 0 {
		return 0, fmt.Errorf("%w [reason=invalid_account_team_id]", errSchemaChanged)
	}
	return n, nil
}
