package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rainhuang0220/whereToken/internal/adapter/testhome"
)

func TestCommunityAPILocalOnlyAndNoEnumerate(t *testing.T) {
	srv := httptest.NewServer(NewMux(testhome.New(t.TempDir())))
	t.Cleanup(srv.Close)

	res, err := http.Get(srv.URL + "/api/community")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d", res.StatusCode)
	}
	var body map[string]any
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if _, ok := body["participant_id"]; ok {
		t.Fatal("local API must not put participant_id on the dashboard payload")
	}
	if _, ok := body["users"]; ok {
		t.Fatal("must not enumerate users")
	}

	req, err := http.NewRequest(http.MethodPost, srv.URL+"/api/community", strings.NewReader(`{"enabled":false}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://evil.example")
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusForbidden {
		t.Fatalf("foreign origin %d", res.StatusCode)
	}

	for _, path := range []string{
		"/v1/community/users",
		"/v1/community/participants",
		"/v1/community/rank",
		"/v1/community/rank?period=2026-08-19",
	} {
		t.Run(path, func(t *testing.T) {
			res, err := http.Get(srv.URL + path)
			if err != nil {
				t.Fatal(err)
			}
			res.Body.Close()
			if res.StatusCode != http.StatusNotFound {
				t.Fatalf("%s → %d", path, res.StatusCode)
			}
		})
	}
}
