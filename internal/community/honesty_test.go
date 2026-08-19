package community

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/rainhuang0220/whereToken/internal/price"
)

func TestDisclaimerDeniesGlobalRank(t *testing.T) {
	if !strings.Contains(DisclaimerEN, "global") || !strings.Contains(DisclaimerEN, "worldwide") || !strings.Contains(DisclaimerEN, "all-AI-users") {
		t.Fatalf("DisclaimerEN must deny global/worldwide/all-AI-users: %s", DisclaimerEN)
	}
	if !strings.Contains(DisclaimerZH, "全球") || !strings.Contains(DisclaimerZH, "全世界") || !strings.Contains(DisclaimerZH, "全体 AI 用户") {
		t.Fatalf("DisclaimerZH must deny 全球/全世界/全体 AI 用户: %s", DisclaimerZH)
	}
}

func TestForbiddenUploadKeysCoverPrivacyBoundary(t *testing.T) {
	have := map[string]struct{}{}
	for _, k := range ForbiddenUploadKeys {
		have[k] = struct{}{}
	}
	for _, want := range []string{"prompt", "request_id", "path", "sqlite", "sqlite_path", "index_path"} {
		if _, ok := have[want]; !ok {
			t.Errorf("ForbiddenUploadKeys missing %q", want)
		}
	}
}

func TestCommunityPayloadOmitsSensitiveFields(t *testing.T) {
	usd := 12.5
	u, err := MakeUpload("aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee", "0.5.0", LocalAgg{
		LocalDate:       "2026-08-19",
		UTCOffsetMin:    480,
		TodayTokens:     1000,
		TodayCostUSD:    &usd,
		TodayCostStatus: price.StatusComplete,
	})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(u)
	if err != nil {
		t.Fatal(err)
	}
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{
		"path", "prompt", "request_id", "requestId", "sqlite_path", "sqlite", "index_path",
	} {
		if _, ok := obj[bad]; ok {
			t.Fatalf("upload JSON key %q: %s", bad, raw)
		}
	}

	id := "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee"
	base := map[string]any{
		"participant_id":     id,
		"period":             "2026-08-19",
		"utc_offset_minutes": 480,
		"tokens":             100,
		"client_version":     "0.5.0",
	}
	for _, bad := range []string{"path", "prompt", "request_id", "sqlite_path"} {
		t.Run("reject_"+bad, func(t *testing.T) {
			m := map[string]any{}
			for k, v := range base {
				m[k] = v
			}
			m[bad] = "/tmp/index.v1.db"
			blob, err := json.Marshal(m)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := DecodeUpload(blob); err == nil {
				t.Fatalf("accepted %s", bad)
			}
		})
	}
}
