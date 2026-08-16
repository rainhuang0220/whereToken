package httpapi

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rainhuang0220/whereToken/internal/adapter/testhome"
)

func TestServeFallsBackToEmbedWhenNoWebDist(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("WHERETOKEN_WEB", "")
	srv := httptest.NewServer(NewMux(testhome.New(t.TempDir())))
	defer srv.Close()
	resp, err := srv.Client().Get(srv.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d", resp.StatusCode)
	}
	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "text/html") {
		t.Fatalf("content-type=%q", ct)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "<!doctype html>") && !strings.Contains(string(body), "<!DOCTYPE html>") {
		t.Fatalf("expected embedded HTML, got %q", body)
	}

	themes, err := srv.Client().Get(srv.URL + "/themes")
	if err != nil {
		t.Fatal(err)
	}
	defer themes.Body.Close()
	tb, err := io.ReadAll(themes.Body)
	if err != nil {
		t.Fatal(err)
	}
	if themes.StatusCode != http.StatusOK {
		t.Fatalf("themes status=%d", themes.StatusCode)
	}
	if !strings.Contains(string(tb), "<!doctype html>") && !strings.Contains(string(tb), "<!DOCTYPE html>") {
		t.Fatalf("SPA fallback missing: %q", tb)
	}
}
