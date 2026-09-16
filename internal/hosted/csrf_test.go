package hosted

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// These tests pin the CSRF contract documented in
// docs/architecture/hosted-web-v070.md: mutating cookie-authenticated routes
// require the caller to prove it read the CSRF value out of the page (via
// the X-CSRF-Token header), not merely that the browser attached cookies.
// A same-site sibling origin can attach cookies to a request but cannot read
// another origin's response body to obtain the header value, so a cookie
// fallback would silently reopen that gap.

func TestCSRFRejectsCookieOnlyWithoutHeader(t *testing.T) {
	h := testMux(t, githubStub())
	auth := loginAuth(t, h)
	if auth.CSRF == nil {
		t.Fatal("login did not set a CSRF cookie")
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/account/usage", nil)
	req.AddCookie(auth.Session)
	req.AddCookie(auth.CSRF) // cookie present, header intentionally omitted
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("cookie-only CSRF must be rejected, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCSRFAcceptsHeaderWithoutCSRFCookie(t *testing.T) {
	h := testMux(t, githubStub())
	auth := loginAuth(t, h)
	if auth.CSRF == nil {
		t.Fatal("login did not set a CSRF cookie")
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/account/usage", nil)
	req.AddCookie(auth.Session)
	req.Header.Set("X-CSRF-Token", auth.CSRF.Value) // header present, cookie intentionally omitted
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("valid header alone must be accepted, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCSRFRejectsWrongHeaderEvenWithValidCookie(t *testing.T) {
	h := testMux(t, githubStub())
	auth := loginAuth(t, h)
	if auth.CSRF == nil {
		t.Fatal("login did not set a CSRF cookie")
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/account/usage", nil)
	req.AddCookie(auth.Session)
	req.AddCookie(auth.CSRF)
	req.Header.Set("X-CSRF-Token", "not-the-right-token")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("mismatched header must be rejected, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestCSRFRejectedPairConfirmCreatesNoDevice guards the pairing endpoint
// specifically: a request that fails the CSRF check must not have any
// side effect on the pairing challenge or create a device.
func TestCSRFRejectedPairConfirmCreatesNoDevice(t *testing.T) {
	h := testMux(t, githubStub())
	auth := loginAuth(t, h)

	startReq := httptest.NewRequest(http.MethodPost, "/api/v1/pair/start", strings.NewReader(`{"os":"linux","arch":"amd64","client_version":"0.7.0"}`))
	startReq.Header.Set("Content-Type", "application/json")
	startRec := httptest.NewRecorder()
	h.ServeHTTP(startRec, startReq)
	var started map[string]any
	if err := json.Unmarshal(startRec.Body.Bytes(), &started); err != nil {
		t.Fatal(err)
	}
	code := started["display_code"].(string)

	confirm := strings.NewReader(`{"display_code":"` + code + `","accept":true}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/pair/confirm", confirm)
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(auth.Session) // no CSRF header, no CSRF cookie at all
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("unconfirmed CSRF must be rejected, got %d: %s", rec.Code, rec.Body.String())
	}

	peek := httptest.NewRequest(http.MethodGet, "/api/v1/pair/challenge?code="+code, nil)
	auth.apply(peek)
	peekRec := httptest.NewRecorder()
	h.ServeHTTP(peekRec, peek)
	var got map[string]any
	if err := json.Unmarshal(peekRec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got["consumed"] == true {
		t.Fatal("CSRF-rejected confirm must not consume the pairing challenge")
	}
}
