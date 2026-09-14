package publicprofile

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rainhuang0220/whereToken/internal/event"
)

func TestRuntimeSchemaMatchesPublishedContract(t *testing.T) {
	runtimeSchema, err := os.ReadFile("public-profile.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	publishedSchema, err := os.ReadFile(filepath.Join("..", "..", "docs", "public-profile.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(runtimeSchema, publishedSchema) {
		t.Fatal("embedded runtime schema drifted from docs/public-profile.schema.json")
	}
}

func TestPublishedDemoPassesRuntimeSchema(t *testing.T) {
	path := filepath.Join("..", "..", "docs", "media", "public-profile-demo", "profile.json")
	if err := ValidateFile(path); err != nil {
		t.Fatal(err)
	}
}

func TestV1ProductionSnapshotStillValidates(t *testing.T) {
	doc := v1SchemaTestDocument(t)
	if err := validateDocument(t, doc); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	var snap Snapshot
	if err := json.Unmarshal(raw, &snap); err != nil {
		t.Fatal(err)
	}
	if err := ValidateProduction(snap, false); err == nil {
		t.Fatal("v1 production snapshot must not pass the v2 production gate")
	}
}

func v1SchemaTestDocument(t *testing.T) map[string]any {
	t.Helper()
	doc := schemaTestDocument(t)
	doc["schema_version"] = float64(SchemaVersionV1)

	periods := objectAt(t, doc, []string{"periods"})
	for _, rawPeriod := range periods {
		period, ok := rawPeriod.(map[string]any)
		if !ok {
			t.Fatal("period is not an object")
		}
		for _, key := range []string{"by_agent", "by_vendor", "by_model"} {
			rows, _ := period[key].([]any)
			for _, rawRow := range rows {
				row, ok := rawRow.(map[string]any)
				if !ok {
					t.Fatal("breakdown row is not an object")
				}
				delete(row, "coverage")
			}
		}
	}

	activity := objectAt(t, doc, []string{"activity"})
	series, _ := activity["series"].([]any)
	v1Series := make([]any, 0, len(series))
	for _, rawSeries := range series {
		item, ok := rawSeries.(map[string]any)
		if !ok {
			t.Fatal("activity series is not an object")
		}
		if item["metric"] != MetricTokens {
			continue
		}
		delete(item, "metric")
		v1Series = append(v1Series, item)
	}
	activity["series"] = v1Series
	return doc
}

func TestValidateFileRejectsAdditionalPropertiesAtEverySchemaLayer(t *testing.T) {
	base := schemaTestDocument(t)
	cases := []struct {
		name string
		path []string
	}{
		{"root", nil},
		{"producer", []string{"producer"}},
		{"owner", []string{"owner"}},
		{"provenance", []string{"provenance"}},
		{"privacy", []string{"privacy"}},
		{"periods", []string{"periods"}},
		{"period", []string{"periods", "all"}},
		{"range", []string{"periods", "all", "range"}},
		{"totals", []string{"periods", "all", "totals"}},
		{"component", []string{"periods", "all", "totals", "total"}},
		{"peak", []string{"periods", "all", "peak"}},
		{"portrait", []string{"periods", "all", "portrait"}},
		{"breakdown", []string{"periods", "all", "by_agent", "0"}},
		{"coverage", []string{"periods", "all", "by_agent", "0", "coverage"}},
		{"cost", []string{"periods", "all", "cost"}},
		{"activity", []string{"activity"}},
		{"series", []string{"activity", "series", "0"}},
		{"links", []string{"links"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc := cloneDocument(t, base)
			objectAt(t, doc, tc.path)["secret_path"] = "/Users/foo/private"
			err := validateDocument(t, doc)
			if err == nil {
				t.Fatal("accepted an additional property")
			}
			if !strings.Contains(err.Error(), "secret_path") {
				t.Fatalf("error does not identify additional property: %v", err)
			}
		})
	}
}

func TestValidateFileEnforcesSchemaRequiredEnumAndType(t *testing.T) {
	base := schemaTestDocument(t)
	tests := []struct {
		name   string
		mutate func(map[string]any)
	}{
		{"missing required", func(doc map[string]any) { delete(doc, "links") }},
		{"invalid enum", func(doc map[string]any) { doc["data_status"] = "secret" }},
		{"wrong type", func(doc map[string]any) { doc["schema_version"] = "1" }},
		{"invalid notice", func(doc map[string]any) { doc["notices"] = []any{"private_note"} }},
		{"private coverage reason", func(doc map[string]any) {
			objectAt(t, doc, []string{"periods", "all", "by_agent", "0", "coverage"})["reason"] = "/Users/foo/private"
		}},
		{"secret token source", func(doc map[string]any) {
			objectAt(t, doc, []string{"periods", "all", "by_agent", "0", "coverage"})["token_source"] = "Bearer abc"
		}},
		{"unsafe token window label", func(doc map[string]any) {
			objectAt(t, doc, []string{"periods", "all", "by_agent", "0", "coverage"})["token_window"] = map[string]any{
				"from": "2025-09-08", "to": "2026-09-14", "label": "<script>",
			}
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			doc := cloneDocument(t, base)
			tc.mutate(doc)
			if err := validateSchemaDocument(t, doc); err == nil {
				t.Fatal("accepted schema-invalid document")
			}
		})
	}
}

func schemaTestDocument(t *testing.T) map[string]any {
	t.Helper()
	loc := shanghai()
	now := time.Date(2026, 9, 13, 15, 0, 0, 0, loc)
	snap, err := Build(Input{
		Events: []event.UsageEvent{{
			Source: "claude", Vendor: "anthropic", Model: "claude-opus-4.6",
			RequestID: "schema", Timestamp: now.Add(-time.Hour), Miss: 10, Output: 1,
			Quality: event.QualityAuthoritative,
		}},
		Now: now, Loc: loc, Version: "test", IncludeModels: true, IncludeCost: true,
		Owner: &Owner{DisplayName: "Test", GitHubLogin: "tester", ProfileURL: "https://github.com/tester"},
	})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := Marshal(snap)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	return doc
}

func cloneDocument(t *testing.T, doc map[string]any) map[string]any {
	t.Helper()
	raw, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func objectAt(t *testing.T, doc map[string]any, path []string) map[string]any {
	t.Helper()
	var cur any = doc
	for _, part := range path {
		switch node := cur.(type) {
		case map[string]any:
			cur = node[part]
		case []any:
			if part != "0" || len(node) == 0 {
				t.Fatalf("bad array path %v", path)
			}
			cur = node[0]
		default:
			t.Fatalf("bad object path %v at %q", path, part)
		}
	}
	out, ok := cur.(map[string]any)
	if !ok {
		t.Fatalf("path %v is %T", path, cur)
	}
	return out
}

func TestValidateFileRejectsSemanticTamperingAfterResigning(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Snapshot)
	}{
		{"invalid url", func(s *Snapshot) { s.Links.LivePage = "javascript:alert(1)" }},
		{"token invariant mismatch", func(s *Snapshot) {
			one := int64(1)
			s.Periods.All.Totals.Total.Value = &one
		}},
		{"activity length mismatch", func(s *Snapshot) {
			s.Activity.Series[0].Values = []int64{1, 2, 3}
		}},
		{"coverage available with unavailable total", func(s *Snapshot) {
			s.Periods.All.ByAgent[0].Coverage.Tokens = StatusAvailable
			s.Periods.All.ByAgent[0].Totals = emptyTotals()
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := schemaTestSnapshot(t)
			tc.mutate(&s)
			refreshSnapshotID(&s)
			raw, err := Marshal(s)
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(t.TempDir(), "profile.json")
			if err := os.WriteFile(path, raw, 0o600); err != nil {
				t.Fatal(err)
			}
			if err := ValidateFile(path); err == nil {
				t.Fatal("accepted semantically invalid document")
			}
		})
	}
}

func TestAllowPartialDoesNotBypassSemanticValidation(t *testing.T) {
	s := schemaTestSnapshot(t)
	one := int64(1)
	s.Periods.All.Totals.Total.Value = &one
	refreshSnapshotID(&s)
	if err := ValidateProduction(s, true); err == nil {
		t.Fatal("allow-partial bypassed a token invariant")
	}
}

func schemaTestSnapshot(t *testing.T) Snapshot {
	t.Helper()
	loc := shanghai()
	now := time.Date(2026, 9, 13, 15, 0, 0, 0, loc)
	snap, err := Build(Input{
		Events: []event.UsageEvent{{
			Source: "claude", Vendor: "anthropic", Model: "claude-opus-4.6",
			RequestID: "schema", Timestamp: now.Add(-time.Hour), Miss: 10, Output: 1,
			Quality: event.QualityAuthoritative,
		}},
		Now: now, Loc: loc, Version: "test", IncludeModels: true, IncludeCost: true,
		Owner: &Owner{DisplayName: "Test", GitHubLogin: "tester", ProfileURL: "https://github.com/tester"},
	})
	if err != nil {
		t.Fatal(err)
	}
	return snap
}

func validateDocument(t *testing.T, doc map[string]any) error {
	t.Helper()
	raw, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "profile.json")
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	return ValidateFile(path)
}

func validateSchemaDocument(t *testing.T, doc map[string]any) error {
	t.Helper()
	raw, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	return validateSchemaJSON(raw)
}
