package hosted

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rainhuang0220/whereToken/internal/syncagg"
)

func TestDeleteUsageKeepsAccountAndAllowsResync(t *testing.T) {
	id := uniqueGitHubID(t)
	h := testMux(t, githubStubID(id, "delusage"))
	auth := loginAuth(t, h)
	tok, devID, hmacKey := pairDevice(t, h, auth)

	putUsage(t, h, tok, devID, hmacKey, 42)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/account/usage", nil)
	auth.apply(req)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete usage %d %s", rec.Code, rec.Body.String())
	}

	dash := dashboard(h, auth)
	if dash.Code != http.StatusOK {
		t.Fatalf("dashboard %d", dash.Code)
	}
	if !bytes.Contains(dash.Body.Bytes(), []byte(`"never_synced":true`)) && !bytes.Contains(dash.Body.Bytes(), []byte(`"never_synced": true`)) {
		t.Fatalf("expected never_synced after delete: %s", dash.Body.Bytes())
	}

	putUsage(t, h, tok, devID, hmacKey, 7)
	dash = dashboard(h, auth)
	if !bytes.Contains(dash.Body.Bytes(), []byte(`"miss":7`)) && !bytes.Contains(dash.Body.Bytes(), []byte(`"miss": 7`)) {
		t.Fatalf("resync did not restore usage: %s", dash.Body.Bytes())
	}
}

func TestDeleteUsageRequiresCSRF(t *testing.T) {
	h := testMux(t, githubStubID(uniqueGitHubID(t), "csrfuser"))
	auth := loginAuth(t, h)
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/account/usage", nil)
	req.AddCookie(auth.Session)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("csrf %d", rec.Code)
	}
}

func TestDeleteAccountInvalidatesSessionAndDeviceToken(t *testing.T) {
	h := testMux(t, githubStubID(uniqueGitHubID(t), "gone"))
	auth := loginAuth(t, h)
	tok, _, _ := pairDevice(t, h, auth)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/account", nil)
	auth.apply(req)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete account %d %s", rec.Code, rec.Body.String())
	}

	me := httptest.NewRequest(http.MethodGet, "/api/v1/session", nil)
	me.AddCookie(auth.Session)
	meRec := httptest.NewRecorder()
	h.ServeHTTP(meRec, me)
	if meRec.Code != http.StatusUnauthorized {
		t.Fatalf("session after delete %d", meRec.Code)
	}

	syncReq := httptest.NewRequest(http.MethodPut, "/api/v1/sync/batch", bytes.NewReader([]byte(`{"schema_version":1,"device_id":"x","idempotency_key":"k"}`)))
	syncReq.Header.Set("Content-Type", "application/json")
	syncReq.Header.Set("Authorization", "Bearer "+tok)
	syncRec := httptest.NewRecorder()
	h.ServeHTTP(syncRec, syncReq)
	if syncRec.Code != http.StatusUnauthorized {
		t.Fatalf("device token after delete %d %s", syncRec.Code, syncRec.Body.String())
	}
}

func githubStubID(id int64, login string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "access_token") {
			w.Header().Set("Content-Type", "application/json")
			ioWrite(w, `{"access_token":"gho_x","token_type":"bearer"}`)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		ioWrite(w, fmt.Sprintf(`{"id":%d,"login":%q,"avatar_url":""}`, id, login))
	})
}

func pairDevice(t *testing.T, h http.Handler, auth authCookies) (token, deviceID, hmacKey string) {
	t.Helper()
	startReq := httptest.NewRequest(http.MethodPost, "/api/v1/pair/start", bytes.NewReader([]byte(`{"os":"darwin","arch":"arm64","client_version":"0.7.0","label":"Mac"}`)))
	startReq.Header.Set("Content-Type", "application/json")
	startRec := httptest.NewRecorder()
	h.ServeHTTP(startRec, startReq)
	var started map[string]any
	_ = json.Unmarshal(startRec.Body.Bytes(), &started)
	confirm, _ := json.Marshal(map[string]any{"display_code": started["display_code"], "accept": true})
	cr := httptest.NewRequest(http.MethodPost, "/api/v1/pair/confirm", bytes.NewReader(confirm))
	cr.Header.Set("Content-Type", "application/json")
	auth.apply(cr)
	crRec := httptest.NewRecorder()
	h.ServeHTTP(crRec, cr)
	if crRec.Code != http.StatusOK {
		t.Fatalf("confirm %d %s", crRec.Code, crRec.Body.String())
	}
	stBody, _ := json.Marshal(map[string]any{"display_code": started["display_code"], "device_secret": started["device_secret"]})
	stReq := httptest.NewRequest(http.MethodPost, "/api/v1/pair/status", bytes.NewReader(stBody))
	stReq.Header.Set("Content-Type", "application/json")
	stRec := httptest.NewRecorder()
	h.ServeHTTP(stRec, stReq)
	var approved map[string]any
	_ = json.Unmarshal(stRec.Body.Bytes(), &approved)
	tok, _ := approved["device_token"].(string)
	dev, _ := approved["device_id"].(string)
	key, _ := approved["source_hmac_key"].(string)
	if tok == "" {
		t.Fatalf("pair %s", stRec.Body.Bytes())
	}
	return tok, dev, key
}

func putUsage(t *testing.T, h http.Handler, tok, devID, hmacB64 string, miss int64) {
	t.Helper()
	key, err := decodeHMACKey(hmacB64)
	if err != nil {
		t.Fatal(err)
	}
	hsh, err := syncagg.SourceKeyHash(key, "claude", syncagg.ScopeDeviceLocal, devID)
	if err != nil {
		t.Fatal(err)
	}
	batch := syncagg.Batch{
		SchemaVersion:  syncagg.SchemaVersion,
		IdempotencyKey: uniqueID(t),
		DeviceID:       devID,
		Timezone:       "UTC",
		DailyModelUsage: []syncagg.DailyModel{{
			Date: "2026-09-03", Tool: "claude", SourceScope: syncagg.ScopeDeviceLocal, SourceKeyHash: hsh,
			Vendor: "anthropic", Model: "claude-opus-4.6", Miss: miss, Revision: 1,
		}},
	}
	raw, _ := json.Marshal(batch)
	req := httptest.NewRequest(http.MethodPut, "/api/v1/sync/batch", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("sync %d %s", rec.Code, rec.Body.String())
	}
}

func dashboard(h http.Handler, auth authCookies) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard/summary", nil)
	auth.apply(req)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}
