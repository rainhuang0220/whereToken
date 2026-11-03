package cursor

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rainhuang0220/whereToken/internal/adapter"
	"github.com/rainhuang0220/whereToken/internal/adapter/testhome"
	"github.com/rainhuang0220/whereToken/internal/event"
)

func TestPBKDF2HMACSHA1MatchesRFC(t *testing.T) {
	got := pbkdf2HMACSHA1([]byte("password"), []byte("salt"), 1, 20)
	want, _ := hex.DecodeString("0c60c80f961f0e71f3a9b524af6012062fe037a6")
	if hex.EncodeToString(got) != hex.EncodeToString(want) {
		t.Fatalf("pbkdf2 %x", got)
	}
}

func TestChromeFixtureDecryptsAndIsNotABearer(t *testing.T) {
	const sealed = "cursor-safe-storage:v1:djEwS7GGvjEQVQOTzdGXxP7jizwnyRN64Yp0JveShwYGTFw="
	blob, ok := sealedBlob(sealed)
	if !ok {
		t.Fatal("fixture envelope")
	}
	plain, err := decryptChromeCBCRaw(blob, deriveChromeKey("fixture-password"))
	if err != nil || plain != "header.payload.sig" {
		t.Fatalf("plain=%q err=%v", plain, err)
	}
	if _, err := decryptChromeCBC(blob, deriveChromeKey("fixture-password")); err == nil {
		t.Fatal("non-jwt plaintext was accepted as a bearer")
	}
	if _, err := decryptChromeCBCRaw(blob, deriveChromeKey("wrong")); err == nil {
		t.Fatal("wrong password opened the fixture")
	}
}

func TestChromeCBCRoundTripKeepsJWT(t *testing.T) {
	sealed := sealChromeCBC("fixture-password", fakeJWT)
	blob, ok := sealedBlob(sealed)
	if !ok {
		t.Fatal("envelope")
	}
	got, err := decryptChromeCBC(blob, deriveChromeKey("fixture-password"))
	if err != nil || got != fakeJWT {
		t.Fatalf("got=%q err=%v", got, err)
	}
}

func TestChromeGCMRoundTripKeepsJWT(t *testing.T) {
	key := bytesRepeat(32, 7)
	sealed := sealChromeGCM(key, fakeJWT)
	blob, ok := sealedBlob(sealed)
	if !ok {
		t.Fatal("envelope")
	}
	got, err := decryptChromeGCM(blob, key)
	if err != nil || got != fakeJWT {
		t.Fatalf("got=%q err=%v", got, err)
	}
}

func TestConsentFileDoesNotCallTheAccountAPIFromTests(t *testing.T) {
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		hits++
	}))
	t.Cleanup(srv.Close)
	dir := t.TempDir()
	consent := filepath.Join(dir, "cursor-connect.json")
	if err := WriteConsent(consent); err != nil {
		t.Fatal(err)
	}
	db := writeVscdb(t, dir, nil, nil)
	putItem(t, db, authAccessTokenKey, "cursor-safe-storage:v1:QUJDRA")
	err := (Adapter{
		HTTP:        srv.Client(),
		APIBase:     srv.URL,
		ConsentPath: consent,
		AllowPrompt: true,
	}).Parse(adapter.SourceRoot{ID: "cursor", Path: db}, func(event.UsageEvent) {}, func(event.TurnEvent) {})
	if err == nil || hits != 0 {
		t.Fatalf("err=%v hits=%d", err, hits)
	}
	if err.Error() != errAccountUsageUnavailable.Error() {
		t.Fatalf("err=%v", err)
	}
}

func TestInjectedUnsealUsesFilteredMonth(t *testing.T) {
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		if r.Header.Get("Authorization") != "Bearer "+fakeJWT {
			http.Error(w, "no", http.StatusUnauthorized)
			return
		}
		if strings.Contains(r.URL.Path, "GetFilteredUsageEvents") {
			w.Write([]byte(`{"totalUsageEventsCount":1,"usageEventsDisplay":[{"timestamp":"1770000000000","model":"gpt-5","tokenUsage":{"inputTokens":5,"outputTokens":1,"cacheReadTokens":2,"cacheWriteTokens":3}}]}`))
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)
	dir := t.TempDir()
	consent := filepath.Join(dir, "cursor-connect.json")
	if err := WriteConsent(consent); err != nil {
		t.Fatal(err)
	}
	db := writeVscdb(t, dir, nil, nil)
	putItem(t, db, authAccessTokenKey, "cursor-safe-storage:v1:QUJDRA")
	var evs []event.UsageEvent
	err := (Adapter{
		HTTP:        srv.Client(),
		APIBase:     srv.URL,
		ConsentPath: consent,
		Probe:       true,
		Now:         func() time.Time { return time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC) },
		Unseal:      func(string) (string, error) { return fakeJWT, nil },
	}).Parse(adapter.SourceRoot{ID: "cursor", Path: db}, func(e event.UsageEvent) {
		evs = append(evs, e)
	}, func(event.TurnEvent) {})
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 1 || !strings.Contains(paths[0], "GetFilteredUsageEvents") {
		t.Fatalf("paths=%v", paths)
	}
	var total int64
	for _, e := range evs {
		if e.Derivation == event.DeriveProviderAPI {
			total += e.Miss + e.CacheRead + e.CacheCreate + e.Output
		}
	}
	if total != 11 {
		t.Fatalf("total=%d events=%+v", total, evs)
	}
	wins := TakeAccountCoverage(db)
	if len(wins) != 1 || wins[0].From != "2026-11-01" || wins[0].To != "2026-11-02" {
		t.Fatalf("windows=%+v", wins)
	}
}

func TestAcceptSubjectBlocksFetch(t *testing.T) {
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { hits++ }))
	t.Cleanup(srv.Close)
	dir := t.TempDir()
	db := writeVscdb(t, dir, nil, nil)
	putItem(t, db, authAccessTokenKey, fakeJWT)
	err := (Adapter{
		HTTP:    srv.Client(),
		APIBase: srv.URL,
		AcceptSubject: func(string) error {
			return errAccountMismatch
		},
	}).Parse(adapter.SourceRoot{ID: "cursor", Path: db}, func(event.UsageEvent) {}, func(event.TurnEvent) {})
	if err == nil || hits != 0 || !strings.Contains(err.Error(), KindAccountMismatch) {
		t.Fatalf("err=%v hits=%d", err, hits)
	}
	if strings.Contains(strings.ToLower(err.Error()), "auth") {
		t.Fatal(err)
	}
}

func TestStoredSessionKindDoesNotReturnTheToken(t *testing.T) {
	dir := t.TempDir()
	path := writeVscdb(t, dir, nil, nil)
	putItem(t, path, authAccessTokenKey, "cursor-safe-storage:v1:QUJDRA")
	kind := StoredSessionKind(testhome.New(dir))
	if kind != "sealed" || strings.Contains(kind, "QUJD") || strings.Contains(kind, "eyJ") {
		t.Fatalf("kind=%s", kind)
	}
}

func TestMonthWindowsDoNotMultiplyOnePage(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if strings.Contains(r.URL.Path, "GetFilteredUsageEvents") {
			w.Write([]byte(`{"totalUsageEventsCount":1,"usageEventsDisplay":[{"timestamp":"1770000000000","model":"gpt-5","conversationId":"c","tokenUsage":{"inputTokens":5}}]}`))
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)
	evs, err := (Adapter{
		HTTP:    srv.Client(),
		APIBase: srv.URL,
		Now:     func() time.Time { return time.Date(2026, 8, 15, 12, 0, 0, 0, time.UTC) },
	}).fetchUsageWithToken("root", fakeJWT)
	if err != nil {
		t.Fatal(err)
	}
	if calls < 2 {
		t.Fatalf("calls=%d", calls)
	}
	var total int64
	for _, e := range evs {
		total += e.Miss + e.Output
	}
	if total != 5 {
		t.Fatalf("total=%d events=%d", total, len(evs))
	}
}

func TestEarlierMonthSurvivesLaterPaginationFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Start int64 `json:"startDate"`
			Page  int   `json:"page"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		start := time.UnixMilli(body.Start).UTC()
		if start.Year() == 2026 && start.Month() == time.March {
			if body.Page > 1 {
				http.Error(w, "boom", http.StatusInternalServerError)
				return
			}
			w.Write([]byte(`{"totalUsageEventsCount":2,"usageEventsDisplay":[{"timestamp":"1772409600000","model":"gpt-5","tokenUsage":{"inputTokens":9}}]}`))
			return
		}
		day := start.Format("2006-01-02")
		w.Write([]byte(`{"totalUsageEventsCount":1,"usageEventsDisplay":[{"timestamp":"` + day + `T00:00:00Z","model":"gpt-5","conversationId":"` + day + `","tokenUsage":{"inputTokens":4}}]}`))
	}))
	t.Cleanup(srv.Close)
	evs, err := (Adapter{
		HTTP:    srv.Client(),
		APIBase: srv.URL,
		Now:     func() time.Time { return time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC) },
	}).fetchUsageWithToken("root", fakeJWT)
	if err == nil || len(evs) == 0 {
		t.Fatalf("err=%v events=%d", err, len(evs))
	}
	wins := TakeAccountCoverage("root")
	if len(wins) == 0 {
		t.Fatal("successful months were dropped")
	}
	for _, w := range wins {
		if strings.HasPrefix(w.From, "2026-03") {
			t.Fatalf("failed March window was stored: %+v", wins)
		}
	}
}

func TestPageLimitSplitsALongWindow(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		var body struct {
			Start int64 `json:"startDate"`
			End   int64 `json:"endDate"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.End-body.Start > int64((10*24*time.Hour)/time.Millisecond) {
			w.Write([]byte(`{"totalUsageEventsCount":3,"usageEventsDisplay":[{"timestamp":"1770000000000","model":"gpt-5","tokenUsage":{"inputTokens":1}}]}`))
			return
		}
		id := time.UnixMilli(body.Start).UTC().Format("2006-01-02")
		w.Write([]byte(`{"totalUsageEventsCount":1,"usageEventsDisplay":[{"timestamp":"` + id + `T12:00:00Z","model":"gpt-5","conversationId":"` + id + `","tokenUsage":{"inputTokens":2}}]}`))
	}))
	t.Cleanup(srv.Close)
	evs, err := (Adapter{
		HTTP:     srv.Client(),
		APIBase:  srv.URL,
		Probe:    true,
		PageSize: 1,
		PageCap:  1,
		Now:      func() time.Time { return time.Date(2026, 3, 20, 0, 0, 0, 0, time.UTC) },
	}).fetchUsageWithToken("split", fakeJWT)
	if err != nil {
		t.Fatal(err)
	}
	if calls < 3 {
		t.Fatalf("calls=%d", calls)
	}
	var total int64
	for _, e := range evs {
		total += e.Miss
	}
	if total == 0 {
		t.Fatal("split produced no tokens")
	}
}

func sealChromeCBC(password, plain string) string {
	key := deriveChromeKey(password)
	block, err := aes.NewCipher(key)
	if err != nil {
		panic(err)
	}
	src := pkcs7Pad([]byte(plain))
	out := make([]byte, len(src))
	cipher.NewCBCEncrypter(block, bytesRepeatSpace(aes.BlockSize)).CryptBlocks(out, src)
	blob := append([]byte("v10"), out...)
	return "cursor-safe-storage:v1:" + base64.StdEncoding.EncodeToString(blob)
}

func sealChromeGCM(key []byte, plain string) string {
	block, err := aes.NewCipher(key)
	if err != nil {
		panic(err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		panic(err)
	}
	nonce := bytesRepeat(gcm.NonceSize(), 9)
	payload := gcm.Seal(nil, nonce, []byte(plain), nil)
	blob := append(append([]byte("v10"), nonce...), payload...)
	return "cursor-safe-storage:v1:" + base64.StdEncoding.EncodeToString(blob)
}

func pkcs7Pad(in []byte) []byte {
	n := aes.BlockSize - len(in)%aes.BlockSize
	out := append([]byte(nil), in...)
	for i := 0; i < n; i++ {
		out = append(out, byte(n))
	}
	return out
}

func bytesRepeat(n int, b byte) []byte {
	out := make([]byte, n)
	for i := range out {
		out[i] = b
	}
	return out
}
