package hosted

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rainhuang0220/whereToken/internal/syncagg"
)

func decodeHMACKey(s string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(s)
}

func TestSyncBatchRequiresDeviceBearerAndIsIdempotent(t *testing.T) {
	h := testMux(t, githubStub())
	sess := loginSession(t, h)
	startReq := httptest.NewRequest(http.MethodPost, "/api/v1/pair/start", bytes.NewReader([]byte(`{"os":"darwin","arch":"arm64","client_version":"0.7.0","label":"Mac"}`)))
	startReq.Header.Set("Content-Type", "application/json")
	startRec := httptest.NewRecorder()
	h.ServeHTTP(startRec, startReq)
	var started map[string]any
	_ = json.Unmarshal(startRec.Body.Bytes(), &started)
	confirm, _ := json.Marshal(map[string]any{"display_code": started["display_code"], "accept": true})
	cr := httptest.NewRequest(http.MethodPost, "/api/v1/pair/confirm", bytes.NewReader(confirm))
	cr.Header.Set("Content-Type", "application/json")
	cr.AddCookie(sess)
	h.ServeHTTP(httptest.NewRecorder(), cr)
	stBody, _ := json.Marshal(map[string]any{"display_code": started["display_code"], "device_secret": started["device_secret"]})
	stReq := httptest.NewRequest(http.MethodPost, "/api/v1/pair/status", bytes.NewReader(stBody))
	stReq.Header.Set("Content-Type", "application/json")
	stRec := httptest.NewRecorder()
	h.ServeHTTP(stRec, stReq)
	var approved map[string]any
	_ = json.Unmarshal(stRec.Body.Bytes(), &approved)
	tok, _ := approved["device_token"].(string)
	hmacKey, _ := approved["source_hmac_key"].(string)
	devID, _ := approved["device_id"].(string)
	if tok == "" || hmacKey == "" {
		t.Fatalf("pair %s", stRec.Body.Bytes())
	}

	key, err := decodeHMACKey(hmacKey)
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
		DailyModelUsage: []syncagg.DailyModel{{
			Date: "2026-09-03", Tool: "claude", SourceScope: syncagg.ScopeDeviceLocal, SourceKeyHash: hsh,
			Vendor: "anthropic", Model: "claude-opus-4.6", Miss: 12, Revision: 1,
		}},
	}
	raw, _ := json.Marshal(batch)
	put := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPut, "/api/v1/sync/batch", bytes.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+tok)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}
	if rec := put(); rec.Code != http.StatusOK {
		t.Fatalf("sync %d %s", rec.Code, rec.Body.String())
	}
	if rec := put(); rec.Code != http.StatusOK {
		t.Fatalf("retry %d %s", rec.Code, rec.Body.String())
	}
	unauth := httptest.NewRequest(http.MethodPut, "/api/v1/sync/batch", bytes.NewReader(raw))
	unauth.Header.Set("Content-Type", "application/json")
	unRec := httptest.NewRecorder()
	h.ServeHTTP(unRec, unauth)
	if unRec.Code != http.StatusUnauthorized {
		t.Fatalf("unauth %d", unRec.Code)
	}
}

func TestSyncRejectsForeignDeviceID(t *testing.T) {
	h := testMux(t, githubStub())
	sess := loginSession(t, h)
	startReq := httptest.NewRequest(http.MethodPost, "/api/v1/pair/start", bytes.NewReader([]byte(`{"os":"darwin","arch":"arm64","client_version":"0.7.0"}`)))
	startReq.Header.Set("Content-Type", "application/json")
	startRec := httptest.NewRecorder()
	h.ServeHTTP(startRec, startReq)
	var started map[string]any
	_ = json.Unmarshal(startRec.Body.Bytes(), &started)
	confirm, _ := json.Marshal(map[string]any{"display_code": started["display_code"], "accept": true})
	cr := httptest.NewRequest(http.MethodPost, "/api/v1/pair/confirm", bytes.NewReader(confirm))
	cr.Header.Set("Content-Type", "application/json")
	cr.AddCookie(sess)
	h.ServeHTTP(httptest.NewRecorder(), cr)
	stBody, _ := json.Marshal(map[string]any{"display_code": started["display_code"], "device_secret": started["device_secret"]})
	stReq := httptest.NewRequest(http.MethodPost, "/api/v1/pair/status", bytes.NewReader(stBody))
	stReq.Header.Set("Content-Type", "application/json")
	stRec := httptest.NewRecorder()
	h.ServeHTTP(stRec, stReq)
	var approved map[string]any
	_ = json.Unmarshal(stRec.Body.Bytes(), &approved)
	tok := approved["device_token"].(string)
	batch := syncagg.Batch{SchemaVersion: 1, IdempotencyKey: uniqueID(t), DeviceID: "other-device"}
	raw, _ := json.Marshal(batch)
	req := httptest.NewRequest(http.MethodPut, "/api/v1/sync/batch", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("foreign device %d %s", rec.Code, rec.Body.String())
	}
}
