package adapter

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

// Pick returns the first non-nil value among names in a decoded JSON
// object. Provider usage APIs mix snake_case and camelCase keys.
func Pick(m map[string]any, names ...string) any {
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

func AsMap(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}

func FlexString(v any) string {
	switch x := v.(type) {
	case string:
		return strings.TrimSpace(x)
	case json.Number:
		return x.String()
	default:
		return ""
	}
}

// FlexInt reads a token counter that may arrive as a JSON number, a
// float, or a numeric string ("1e12" and "1.5" count too).
func FlexInt(v any) int64 {
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
		if s == "" || s == "null" {
			return 0
		}
		if n, err := strconv.ParseInt(s, 10, 64); err == nil {
			return n
		}
		if f, err := strconv.ParseFloat(s, 64); err == nil {
			return int64(f)
		}
		return 0
	default:
		return 0
	}
}

// ErrStatus is a usage-API HTTP failure. The message never carries the
// token, the URL, or the response body.
type ErrStatus struct{ Code int }

func (e ErrStatus) Error() string {
	if e.Code == http.StatusUnauthorized || e.Code == http.StatusForbidden {
		return "本机登录态已失效"
	}
	return fmt.Sprintf("账号用量接口 HTTP %d", e.Code)
}

func IsUnauthorized(err error) bool {
	es, ok := err.(ErrStatus)
	return ok && (es.Code == http.StatusUnauthorized || es.Code == http.StatusForbidden)
}
