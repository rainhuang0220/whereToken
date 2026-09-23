package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/rainhuang0220/whereToken/internal/adapter/testhome"
	"github.com/rainhuang0220/whereToken/internal/publicprofile"
)

func TestPublicProfileAPIIsLocalAndDoesNotInventPublication(t *testing.T) {
	t.Setenv("WHERETOKEN_PUBLIC_PROFILE_FILE", "")
	t.Setenv("WHERETOKEN_PUBLIC_PROFILE_DIR", "")
	root := t.TempDir()
	srv := httptest.NewServer(NewMux(testhome.New(root)))
	t.Cleanup(srv.Close)

	res, err := http.Get(srv.URL + "/api/public-profile")
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
	if body["status"] != publicprofile.StatusUnconfigured || body["public_palette"] != "newsprint" {
		t.Fatalf("%v", body)
	}
	if body["status"] == "published" {
		t.Fatal("unconfigured local state claimed publication")
	}

	req, err := http.NewRequest(http.MethodPost, srv.URL+"/api/public-profile", strings.NewReader(`{"public_palette":"cobalt"}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Origin", "https://evil.example")
	denied, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	denied.Body.Close()
	if denied.StatusCode != http.StatusForbidden {
		t.Fatalf("foreign origin %d", denied.StatusCode)
	}

	bad, err := http.Post(srv.URL+"/api/public-profile", "application/json", strings.NewReader(`{"public_palette":"kiln"}`))
	if err != nil {
		t.Fatal(err)
	}
	bad.Body.Close()
	if bad.StatusCode != http.StatusBadRequest {
		t.Fatalf("kiln status %d", bad.StatusCode)
	}

	ok, err := http.Post(srv.URL+"/api/public-profile", "application/json", strings.NewReader(`{"public_palette":"cobalt"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer ok.Body.Close()
	if ok.StatusCode != http.StatusOK {
		t.Fatalf("apply %d", ok.StatusCode)
	}
	var applied map[string]any
	if err := json.NewDecoder(ok.Body).Decode(&applied); err != nil {
		t.Fatal(err)
	}
	if applied["status"] != publicprofile.StatusSavedLocally || applied["public_palette"] != "cobalt" {
		t.Fatalf("%v", applied)
	}
	if strings.Contains(applied["status"].(string), "published") && applied["status"] != publicprofile.StatusReadyToPublish {
		t.Fatal(applied["status"])
	}
	raw, err := os.ReadFile(publicprofile.ConfigPath(testhome.New(root)))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"public_palette": "cobalt"`) {
		t.Fatalf("config=%s", raw)
	}
}
