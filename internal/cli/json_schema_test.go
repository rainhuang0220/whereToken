package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func loadCLIJSONSchema(t *testing.T) map[string]any {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	raw, err := os.ReadFile(filepath.Join(filepath.Dir(file), "..", "..", "docs", "cli-json.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	var schema map[string]any
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatal(err)
	}
	return schema
}

func schemaObjectProperties(t *testing.T, root map[string]any, path ...string) map[string]any {
	t.Helper()
	cur := any(root)
	for _, key := range path {
		obj, ok := cur.(map[string]any)
		if !ok {
			t.Fatalf("schema %v is %T, want object", path, cur)
		}
		next, ok := obj[key]
		if !ok {
			t.Fatalf("schema missing %q under %v", key, path)
		}
		cur = next
	}
	props, ok := cur.(map[string]any)
	if !ok {
		t.Fatalf("schema %v is %T, want object", path, cur)
	}
	return props
}

func assertCLICommunityAgainstSchema(t *testing.T, payload map[string]any) {
	t.Helper()
	schema := loadCLIJSONSchema(t)
	communityProps := schemaObjectProperties(t, schema, "$defs", "community", "properties")
	standingProps := schemaObjectProperties(t, schema, "$defs", "standing", "properties")
	if standingProps["rank"] == nil {
		t.Fatal("schema $defs/standing must declare rank")
	}
	raw, ok := payload["community"]
	if !ok {
		t.Fatal("CLI --json must include community")
	}
	comm, ok := raw.(map[string]any)
	if !ok {
		t.Fatalf("community must be an object, got %T", raw)
	}
	assertNoExtraKeys(t, comm, communityProps, "community")
	for _, period := range []string{"today", "all"} {
		stRaw, ok := comm[period]
		if !ok {
			continue
		}
		st, ok := stRaw.(map[string]any)
		if !ok {
			t.Fatalf("community.%s must be an object, got %T", period, stRaw)
		}
		path := "community." + period
		assertNoExtraKeys(t, st, standingProps, path)
		if _, ok := st["status"]; !ok {
			t.Errorf("%s missing required %q", path, "status")
		}
		if rank, has := st["rank"]; has {
			n, ok := rank.(float64)
			min := standingRankMinimum(standingProps)
			if !ok || n < min || n != float64(int64(n)) {
				t.Errorf("%s.rank=%v must be an integer >= %v (never 0)", path, rank, min)
			}
		}
	}
}

func standingRankMinimum(standingProps map[string]any) float64 {
	min := 1.0
	spec, ok := standingProps["rank"].(map[string]any)
	if !ok {
		return min
	}
	if n, ok := spec["minimum"].(float64); ok && n > min {
		return n
	}
	return min
}

func assertNoExtraKeys(t *testing.T, obj map[string]any, allowed map[string]any, path string) {
	t.Helper()
	for k := range obj {
		if _, ok := allowed[k]; !ok {
			t.Errorf("%s undeclared key %q", path, k)
		}
	}
}
