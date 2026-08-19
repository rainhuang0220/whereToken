package community

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rainhuang0220/whereToken/internal/adapter/testhome"
	"github.com/rainhuang0220/whereToken/internal/event"
	"github.com/rainhuang0220/whereToken/internal/price"
)

func testHandler(minN int) *Handler {
	h := NewHandler(NewStore(minN))
	h.now = func() time.Time { return time.Date(2026, 8, 19, 15, 0, 0, 0, time.UTC) }
	return h
}

func TestStoreCompetitionAndThreshold(t *testing.T) {
	s := NewStore(3)
	day := "2026-08-19"
	put := func(id string, tokens int64) {
		t.Helper()
		if err := s.Put(Upload{
			ParticipantID: id, Period: day, Tokens: tokens, ClientVersion: "0.5.0",
		}); err != nil {
			t.Fatal(err)
		}
	}
	a := "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee"
	b := "bbbbbbbb-bbbb-4ccc-8ddd-eeeeeeeeeeee"
	c := "cccccccc-bbbb-4ccc-8ddd-eeeeeeeeeeee"
	put(a, 100)
	put(b, 100)
	put(c, 80)
	// 100/100/80 → #1 #1 #3
	for _, tc := range []struct {
		id      string
		rank    int
		display string
	}{
		{a, 1, "#1 / 3"},
		{b, 1, "#1 / 3"},
		{c, 3, "#3 / 3"},
	} {
		st := s.Rank(tc.id, PeriodToday, day, MetricTokens)
		if st.Status != StatusOK || st.Rank != tc.rank || st.Display != tc.display {
			t.Fatalf("%s %+v want #%d", tc.id[:8], st, tc.rank)
		}
	}

	s2 := NewStore(20)
	put2 := func(id string, tokens int64) {
		if err := s2.Put(Upload{ParticipantID: id, Period: day, Tokens: tokens, ClientVersion: "0.5.0"}); err != nil {
			t.Fatal(err)
		}
	}
	put2(a, 100)
	put2(b, 90)
	put2(c, 80)
	st := s2.Rank(a, PeriodToday, day, MetricTokens)
	if st.Status != StatusInsufficientParticipants || st.Rank != 0 || st.Display != "" {
		t.Fatalf("threshold %+v", st)
	}
	if Caption(st) != "—" || strings.Contains(Caption(st), "#1 / 3") {
		t.Fatalf("must hide #1 / 3: caption=%q", Caption(st))
	}
}

func TestStoreAllTimeSumsDailyAndStartsAtJoin(t *testing.T) {
	s := NewStore(2)
	id := "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee"
	other := "bbbbbbbb-bbbb-4ccc-8ddd-eeeeeeeeeeee"
	if err := s.Put(Upload{ParticipantID: id, Period: "2026-08-18", Tokens: 10, ClientVersion: "0.5.0"}); err != nil {
		t.Fatal(err)
	}
	if err := s.Put(Upload{ParticipantID: id, Period: "2026-08-19", Tokens: 5, ClientVersion: "0.5.0"}); err != nil {
		t.Fatal(err)
	}
	if err := s.Put(Upload{ParticipantID: other, Period: "2026-08-19", Tokens: 100, ClientVersion: "0.5.0"}); err != nil {
		t.Fatal(err)
	}
	st := s.Rank(id, PeriodAll, "", MetricTokens)
	if st.Rank != 2 || st.Participants != 2 {
		t.Fatalf("%+v", st)
	}
	// today only ranks people who uploaded that local date
	st = s.Rank(id, PeriodToday, "2026-08-18", MetricTokens)
	if st.Participants != 1 || st.Status != StatusInsufficientParticipants {
		t.Fatalf("yesterday-only %+v", st)
	}
}

func TestStoreZeroTokensNotRanked(t *testing.T) {
	s := NewStore(1)
	id := "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee"
	if err := s.Put(Upload{ParticipantID: id, Period: "2026-08-19", Tokens: 0, ClientVersion: "0.5.0"}); err != nil {
		t.Fatal(err)
	}
	st := s.Rank(id, PeriodToday, "2026-08-19", MetricTokens)
	if st.Status != StatusInsufficientParticipants && st.Status != StatusNotRanked {
		t.Fatalf("%+v", st)
	}
	if st.Rank != 0 {
		t.Fatalf("zero usage must not be #0: %+v", st)
	}
}

func TestStoreCostScoreRoundsAndDropsDust(t *testing.T) {
	s := NewStore(1)
	id := "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee"
	dust := 0.0000004 // 0.4 µUSD — truncation and rounding both skip
	if err := s.Put(Upload{
		ParticipantID: id, Period: "2026-08-19", Tokens: 10,
		EstimatedCostUSD: &dust, CostStatus: price.StatusComplete, ClientVersion: "0.5.0",
	}); err != nil {
		t.Fatal(err)
	}
	st := s.Rank(id, PeriodToday, "2026-08-19", MetricCost)
	if st.Rank != 0 || st.Display != "" {
		t.Fatalf("dust cost must not be a podium: %+v", st)
	}
	half := 0.0000015 // 1.5 µUSD rounds to 2
	if err := s.Put(Upload{
		ParticipantID: id, Period: "2026-08-19", Tokens: 10,
		EstimatedCostUSD: &half, CostStatus: price.StatusComplete, ClientVersion: "0.5.0",
	}); err != nil {
		t.Fatal(err)
	}
	st = s.Rank(id, PeriodToday, "2026-08-19", MetricCost)
	if st.Status != StatusOK || st.Rank != 1 {
		t.Fatalf("rounded micro must rank: %+v", st)
	}
}

func TestStoreAllTimeCostUnavailable(t *testing.T) {
	s := NewStore(1)
	id := "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee"
	usd := 1.0
	if err := s.Put(Upload{
		ParticipantID: id, Period: "2026-08-19", Tokens: 10,
		EstimatedCostUSD: &usd, CostStatus: price.StatusComplete, ClientVersion: "0.5.0",
	}); err != nil {
		t.Fatal(err)
	}
	st := s.Rank(id, PeriodAll, "", MetricCost)
	if st.Status != StatusUnavailable || st.Rank != 0 {
		t.Fatalf("%+v", st)
	}
}

func TestStoreLeaveRemovesParticipant(t *testing.T) {
	s := NewStore(1)
	id := "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee"
	if err := s.Put(Upload{ParticipantID: id, Period: "2026-08-19", Tokens: 10, ClientVersion: "0.5.0"}); err != nil {
		t.Fatal(err)
	}
	if err := s.Leave(id); err != nil {
		t.Fatal(err)
	}
	st := s.Rank(id, PeriodToday, "2026-08-19", MetricTokens)
	if st.Rank != 0 || st.Display != "" || Caption(st) != "—" {
		t.Fatalf("left id must not keep a place: %+v", st)
	}
	if st.Status == StatusOptedOut {
		t.Fatal("remote rank must not advertise opted_out — that is a leave oracle")
	}
}

func TestStoreLeaveRankMatchesNeverSeen(t *testing.T) {
	s := NewStore(2)
	seen := "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee"
	other := "bbbbbbbb-bbbb-4ccc-8ddd-eeeeeeeeeeee"
	never := "cccccccc-bbbb-4ccc-8ddd-eeeeeeeeeeee"
	if err := s.Put(Upload{ParticipantID: seen, Period: "2026-08-19", Tokens: 10, ClientVersion: "0.5.0"}); err != nil {
		t.Fatal(err)
	}
	if err := s.Put(Upload{ParticipantID: other, Period: "2026-08-19", Tokens: 20, ClientVersion: "0.5.0"}); err != nil {
		t.Fatal(err)
	}
	if err := s.Leave(seen); err != nil {
		t.Fatal(err)
	}
	left := s.Rank(seen, PeriodToday, "2026-08-19", MetricTokens)
	unknown := s.Rank(never, PeriodToday, "2026-08-19", MetricTokens)
	if left.Status != unknown.Status || left.Rank != unknown.Rank || left.Display != unknown.Display || left.Participants != unknown.Participants {
		t.Fatalf("leave oracle: left=%+v never=%+v", left, unknown)
	}
	if left.Status == StatusOptedOut || unknown.Status == StatusOptedOut {
		t.Fatal("GET rank must not distinguish a departed UUID from a never-seen one")
	}
	if err := s.Leave(never); err != nil {
		t.Fatal(err)
	}
	s.mu.Lock()
	if s.left[never] {
		s.mu.Unlock()
		t.Fatal("leave of unknown uuid must not record opted_out")
	}
	s.mu.Unlock()
	after := s.Rank(never, PeriodToday, "2026-08-19", MetricTokens)
	if after.Status != unknown.Status || after.Status == StatusOptedOut {
		t.Fatalf("unknown leave changed rank: %+v want %+v", after, unknown)
	}
}

func TestHandlerFakeServerIntegration(t *testing.T) {
	h := testHandler(3)
	srv := httptest.NewServer(h.Mux())
	t.Cleanup(srv.Close)

	ids := []string{
		"aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee",
		"bbbbbbbb-bbbb-4ccc-8ddd-eeeeeeeeeeee",
		"cccccccc-bbbb-4ccc-8ddd-eeeeeeeeeeee",
	}
	tokens := []int64{300, 200, 200}
	for i, id := range ids {
		body := fmt.Sprintf(`{"participant_id":%q,"period":"2026-08-19","utc_offset_minutes":480,"tokens":%d,"client_version":"0.5.0"}`, id, tokens[i])
		res, err := http.Post(srv.URL+"/v1/community/usage", "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode != http.StatusNoContent {
			t.Fatalf("upload %d", res.StatusCode)
		}
	}
	res, err := http.Get(srv.URL + "/v1/community/rank?participant_id=" + ids[0] + "&period=2026-08-19&metric=tokens")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var st Standing
	if err := json.NewDecoder(res.Body).Decode(&st); err != nil {
		t.Fatal(err)
	}
	if st.Status != StatusOK || st.Rank != 1 || st.Display != "#1 / 3" {
		t.Fatalf("%+v", st)
	}

	res, err = http.Get(srv.URL + "/v1/community/rank?participant_id=" + ids[1] + "&period=2026-08-19&metric=tokens")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if err := json.NewDecoder(res.Body).Decode(&st); err != nil {
		t.Fatal(err)
	}
	if st.Rank != 2 { // 300, 200, 200 → #1, #2, #2
		t.Fatalf("tie for second: %+v", st)
	}

	// no user enumeration
	for _, path := range []string{"/v1/community/users", "/v1/community/participants", "/v1/community"} {
		res, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode != http.StatusNotFound {
			t.Fatalf("%s → %d", path, res.StatusCode)
		}
	}

	// reject raw-looking payload
	res, err = http.Post(srv.URL+"/v1/community/usage", "application/json", strings.NewReader(`{"participant_id":"`+ids[0]+`","period":"2026-08-19","tokens":1,"client_version":"0.5.0","prompt":"hi"}`))
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("prompt upload %d", res.StatusCode)
	}
}

func TestHandlerUnavailableCostOmitsUSD(t *testing.T) {
	h := testHandler(1)
	srv := httptest.NewServer(h.Mux())
	t.Cleanup(srv.Close)
	id := "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee"
	tests := []struct {
		name string
		body string
		want int
	}{
		{
			name: "omitted usd accepted",
			body: fmt.Sprintf(`{"participant_id":%q,"period":"2026-08-19","tokens":10,"cost_status":"unavailable","client_version":"0.5.0"}`, id),
			want: http.StatusNoContent,
		},
		{
			name: "explicit zero rejected",
			body: fmt.Sprintf(`{"participant_id":%q,"period":"2026-08-19","tokens":10,"cost_status":"unavailable","estimated_cost_usd":0,"client_version":"0.5.0"}`, id),
			want: http.StatusBadRequest,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			res, err := http.Post(srv.URL+"/v1/community/usage", "application/json", strings.NewReader(tc.body))
			if err != nil {
				t.Fatal(err)
			}
			res.Body.Close()
			if res.StatusCode != tc.want {
				t.Fatalf("status %d want %d", res.StatusCode, tc.want)
			}
		})
	}
	h.Store.mu.Lock()
	row := h.Store.days[id]["2026-08-19"]
	h.Store.mu.Unlock()
	if row.CostStatus != price.StatusUnavailable || row.CostUSD != nil {
		t.Fatalf("store must not keep $0: %+v", row)
	}
}

func TestHandlerRejectsHugeAndUnknown(t *testing.T) {
	h := testHandler(1)
	srv := httptest.NewServer(h.Mux())
	t.Cleanup(srv.Close)
	id := "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee"
	body := fmt.Sprintf(`{"participant_id":%q,"period":"2026-08-19","tokens":%d,"client_version":"0.5.0"}`, id, MaxTokens+1)
	res, err := http.Post(srv.URL+"/v1/community/usage", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("huge %d", res.StatusCode)
	}
	res, err = http.Get(srv.URL + "/v1/community/rank")
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("missing id %d", res.StatusCode)
	}
}

func TestClientSyncRoundTripAndOffline(t *testing.T) {
	h := testHandler(1)
	srv := httptest.NewServer(h.Mux())
	t.Cleanup(srv.Close)
	id := "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee"
	c := &Client{
		BaseURL:  srv.URL,
		File:     &File{ParticipantID: id, Enabled: true},
		Version:  "0.5.0",
		MinCache: time.Hour,
		HTTP:     srv.Client(),
	}
	loc := time.UTC
	now := time.Date(2026, 8, 19, 15, 0, 0, 0, loc)
	events := []event.UsageEvent{{
		Vendor: "anthropic", Model: "claude-opus-4.6", Miss: 1_000_000, Timestamp: now,
	}}
	view := c.Sync(context.Background(), events, now, loc)
	if view.Today.Status != StatusOK || view.Today.Rank != 1 || view.Today.Display != "#1 / 1" {
		t.Fatalf("%+v", view.Today)
	}

	off := &Client{BaseURL: srv.URL, File: &File{ParticipantID: id, Enabled: true}, Offline: true}
	v := off.Sync(context.Background(), events, now, loc)
	if v.Today.Status != StatusOffline || v.Today.Rank != 0 {
		t.Fatalf("offline %+v", v)
	}

	disabled := &Client{BaseURL: srv.URL, File: &File{ParticipantID: id, Enabled: false}}
	v = disabled.Sync(context.Background(), events, now, loc)
	if v.Enabled || v.Today.Status != StatusOptedOut {
		t.Fatalf("opt-out %+v", v)
	}

	none := &Client{File: &File{ParticipantID: id, Enabled: true}, Version: "0.5.0"}
	v = none.Sync(context.Background(), events, now, loc)
	if v.Today.Status != StatusServiceUnconfigured || v.Today.Rank != 0 {
		t.Fatalf("unconfigured %+v", v)
	}
}

func TestClientNetworkFailureDoesNotBreak(t *testing.T) {
	c := &Client{
		BaseURL: "http://127.0.0.1:1",
		File:    &File{ParticipantID: "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee", Enabled: true},
		Version: "0.5.0",
		HTTP:    &http.Client{Timeout: 50 * time.Millisecond},
	}
	now := time.Date(2026, 8, 19, 15, 0, 0, 0, time.UTC)
	events := []event.UsageEvent{{Vendor: "anthropic", Model: "claude-opus-4.6", Miss: 10, Timestamp: now}}
	v := c.Sync(context.Background(), events, now, time.UTC)
	if v.Today.Status != StatusNetworkError || v.Today.Rank != 0 || v.Today.Display != "" {
		t.Fatalf("%+v", v)
	}
	if Caption(v.Today) != "—" || strings.Contains(Caption(v.Today), "#0") {
		t.Fatalf("podium caption=%q", Caption(v.Today))
	}
	raw, err := json.Marshal(v.Today)
	if err != nil {
		t.Fatal(err)
	}
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		t.Fatal(err)
	}
	if obj["status"] != StatusNetworkError {
		t.Fatalf("%s", raw)
	}
	if _, ok := obj["rank"]; ok {
		t.Fatalf("rank must be omitted, not a 0 podium: %s", raw)
	}
	if _, ok := obj["display"]; ok {
		t.Fatalf("display must be omitted: %s", raw)
	}
}

func TestClientUploadOmitsUnavailableCost(t *testing.T) {
	var saw []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/community/usage" {
			saw, _ = io.ReadAll(r.Body)
			w.WriteHeader(http.StatusNoContent)
			return
		}
		_ = json.NewEncoder(w).Encode(Standing{Status: StatusOK, Rank: 1, Participants: 20, Display: "#1 / 20", SelfReported: true})
	}))
	t.Cleanup(srv.Close)
	c := &Client{
		BaseURL:  srv.URL,
		File:     &File{ParticipantID: "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee", Enabled: true},
		Version:  "0.5.0",
		MinCache: time.Hour,
		HTTP:     srv.Client(),
	}
	now := time.Date(2026, 8, 19, 15, 0, 0, 0, time.UTC)
	events := []event.UsageEvent{{Vendor: "moonshot", Model: "k3", Miss: 1000, Timestamp: now}}
	_ = c.Sync(context.Background(), events, now, time.UTC)
	if len(saw) == 0 {
		t.Fatal("no upload")
	}
	var obj map[string]any
	if err := json.Unmarshal(saw, &obj); err != nil {
		t.Fatal(err)
	}
	if _, ok := obj["estimated_cost_usd"]; ok {
		t.Fatalf("unavailable cost must omit estimated_cost_usd: %s", saw)
	}
	if obj["cost_status"] != price.StatusUnavailable {
		t.Fatalf("%s", saw)
	}
	if _, err := DecodeUpload(saw); err != nil {
		t.Fatal(err)
	}
}

func TestClientDoesNotUploadRawEvents(t *testing.T) {
	var saw []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/community/usage" {
			saw, _ = io.ReadAll(r.Body)
			w.WriteHeader(http.StatusNoContent)
			return
		}
		_ = json.NewEncoder(w).Encode(Standing{Status: StatusOK, Rank: 1, Participants: 20, Display: "#1 / 20", SelfReported: true})
	}))
	t.Cleanup(srv.Close)
	c := &Client{
		BaseURL:  srv.URL,
		File:     &File{ParticipantID: "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee", Enabled: true},
		Version:  "0.5.0",
		MinCache: time.Hour,
		HTTP:     srv.Client(),
	}
	now := time.Date(2026, 8, 19, 15, 0, 0, 0, time.UTC)
	events := []event.UsageEvent{{
		Source: "claude", Vendor: "anthropic", Model: "claude-opus-4.6",
		RequestID: "secret-request", SessionID: "secret-session",
		Workspace: "/Users/rain/secret/repo/file.go", Miss: 1000, Timestamp: now,
	}}
	_ = c.Sync(context.Background(), events, now, time.UTC)
	if len(saw) == 0 {
		t.Fatal("no upload")
	}
	s := string(saw)
	for _, bad := range []string{"secret-request", "secret-session", "secret/repo", "prompt", "Users/rain"} {
		if strings.Contains(s, bad) {
			t.Fatalf("leaked %q in %s", bad, s)
		}
	}
	var obj map[string]any
	if err := json.Unmarshal(saw, &obj); err != nil {
		t.Fatal(err)
	}
	if _, ok := obj["events"]; ok {
		t.Fatal("events key")
	}
}

func TestNewParticipantIDIsRandomUUID(t *testing.T) {
	a, err := NewParticipantID()
	if err != nil {
		t.Fatal(err)
	}
	b, err := NewParticipantID()
	if err != nil {
		t.Fatal(err)
	}
	if a == b {
		t.Fatal("ids collided")
	}
	if !uuidRe.MatchString(a) {
		t.Fatalf("%s", a)
	}
	for _, leak := range []string{"rainhuang", "local", "host"} {
		if strings.Contains(strings.ToLower(a), leak) {
			t.Fatalf("id looks identified: %s", a)
		}
	}
}

func TestConfigRoundTripDefaultOn(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "community.json")
	f, err := LoadOrCreate(path, "2026-08-19")
	if err != nil {
		t.Fatal(err)
	}
	if !f.Enabled || !uuidRe.MatchString(f.ParticipantID) {
		t.Fatalf("%+v", f)
	}
	if err := f.SetEnabled(path, false); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil || got.Enabled || got.ParticipantID != f.ParticipantID {
		t.Fatalf("%+v %v", got, err)
	}
}

func TestResolveSkipsFileAndUploadWithoutURL(t *testing.T) {
	dir := t.TempDir()
	home := testhome.New(dir)
	now := time.Date(2026, 8, 19, 15, 0, 0, 0, time.UTC)
	events := []event.UsageEvent{{Vendor: "anthropic", Model: "claude-opus-4.6", Miss: 10, Timestamp: now}}
	v := Resolve(Request{
		Home: home, Getenv: func(string) string { return "" },
		Version: "0.5.0", Now: now, Loc: time.UTC,
	}, events)
	if v.Today.Status != StatusServiceUnconfigured || v.Today.Rank != 0 || Caption(v.Today) != "—" {
		t.Fatalf("%+v", v.Today)
	}
	if _, err := os.Stat(ConfigPath(home)); !os.IsNotExist(err) {
		t.Fatalf("must not write community.json without a URL: %v", err)
	}

	v = Resolve(Request{
		Home: home, Getenv: func(string) string { return "" },
		OptOut: true, Now: now, Loc: time.UTC,
	}, events)
	if v.Enabled || v.Today.Status != StatusOptedOut || Caption(v.Today) != "—" {
		t.Fatalf("opt-out %+v", v)
	}

	v = Resolve(Request{
		Home: home, Getenv: func(string) string { return "" },
		Offline: true, Now: now, Loc: time.UTC,
	}, events)
	if v.Today.Status != StatusOffline || v.Today.Rank != 0 {
		t.Fatalf("offline %+v", v)
	}
}

func TestLeaveEndpoint(t *testing.T) {
	h := testHandler(1)
	srv := httptest.NewServer(h.Mux())
	t.Cleanup(srv.Close)
	id := "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee"
	body := fmt.Sprintf(`{"participant_id":%q,"period":"2026-08-19","tokens":10,"client_version":"0.5.0"}`, id)
	res, err := http.Post(srv.URL+"/v1/community/usage", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	res, err = http.Post(srv.URL+"/v1/community/leave", "application/json", bytes.NewReader([]byte(`{"participant_id":"`+id+`"}`)))
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusNoContent {
		t.Fatalf("leave %d", res.StatusCode)
	}
	never := "bbbbbbbb-bbbb-4ccc-8ddd-eeeeeeeeeeee"
	leftRes, err := http.Get(srv.URL + "/v1/community/rank?participant_id=" + id + "&period=2026-08-19&metric=tokens")
	if err != nil {
		t.Fatal(err)
	}
	defer leftRes.Body.Close()
	var left Standing
	if err := json.NewDecoder(leftRes.Body).Decode(&left); err != nil {
		t.Fatal(err)
	}
	unknownRes, err := http.Get(srv.URL + "/v1/community/rank?participant_id=" + never + "&period=2026-08-19&metric=tokens")
	if err != nil {
		t.Fatal(err)
	}
	defer unknownRes.Body.Close()
	var unknown Standing
	if err := json.NewDecoder(unknownRes.Body).Decode(&unknown); err != nil {
		t.Fatal(err)
	}
	if leftRes.StatusCode != unknownRes.StatusCode || left.Status != unknown.Status || left.Rank != unknown.Rank || left.Display != unknown.Display {
		t.Fatalf("leave oracle via GET /rank: left=%d %+v never=%d %+v", leftRes.StatusCode, left, unknownRes.StatusCode, unknown)
	}
	if left.Status == StatusOptedOut {
		t.Fatal("remote rank must not advertise opted_out")
	}
	// unknown leave is the same 204 as a real leave
	res, err = http.Post(srv.URL+"/v1/community/leave", "application/json", bytes.NewReader([]byte(`{"participant_id":"`+never+`"}`)))
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusNoContent {
		t.Fatalf("unknown leave %d", res.StatusCode)
	}
	h.Store.mu.Lock()
	recorded := h.Store.left[never]
	h.Store.mu.Unlock()
	if recorded {
		t.Fatal("unknown leave must not record opted_out")
	}
}

type countingTransport struct {
	posts int
	reqs  int
}

func (c *countingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	c.reqs++
	if req.Method == http.MethodPost {
		c.posts++
	}
	return nil, fmt.Errorf("offline must not dial")
}

func TestOfflineClientDoesNotHTTPPost(t *testing.T) {
	tr := &countingTransport{}
	id := "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee"
	c := &Client{
		BaseURL: "http://community.example",
		File:    &File{ParticipantID: id, Enabled: true},
		Version: "0.5.0",
		Offline: true,
		HTTP:    &http.Client{Transport: tr},
	}
	now := time.Date(2026, 8, 19, 15, 0, 0, 0, time.UTC)
	events := []event.UsageEvent{{
		Vendor: "anthropic", Model: "claude-opus-4.6", Miss: 1_000_000, Timestamp: now,
	}}
	v := c.Sync(context.Background(), events, now, time.UTC)
	if v.Today.Status != StatusOffline {
		t.Fatalf("%+v", v.Today)
	}
	if err := c.Leave(context.Background()); err != nil {
		t.Fatal(err)
	}
	if tr.posts != 0 || tr.reqs != 0 {
		t.Fatalf("offline issued HTTP posts=%d reqs=%d", tr.posts, tr.reqs)
	}
}

func TestHandlerRankRoutes(t *testing.T) {
	h := testHandler(1)
	srv := httptest.NewServer(h.Mux())
	t.Cleanup(srv.Close)
	tests := []struct {
		method string
		path   string
		want   int
	}{
		{http.MethodGet, "/v1/community/rank", http.StatusBadRequest},
		{http.MethodGet, "/v1/community/rank?period=2026-08-19", http.StatusBadRequest},
		{http.MethodGet, "/v1/community/rank?participant_id=", http.StatusBadRequest},
		{http.MethodGet, "/v1/community/rank?participant_id=not-a-uuid&period=2026-08-19", http.StatusBadRequest},
		{http.MethodGet, "/v1/community/users", http.StatusNotFound},
		{http.MethodGet, "/v1/community/participants", http.StatusNotFound},
		{http.MethodGet, "/v1/community", http.StatusNotFound},
	}
	for _, tc := range tests {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			req, err := http.NewRequest(tc.method, srv.URL+tc.path, nil)
			if err != nil {
				t.Fatal(err)
			}
			res, err := srv.Client().Do(req)
			if err != nil {
				t.Fatal(err)
			}
			res.Body.Close()
			if res.StatusCode != tc.want {
				t.Fatalf("got %d want %d", res.StatusCode, tc.want)
			}
		})
	}
}

func TestHandlerDoesNotPersistRemoteAddrIntoStore(t *testing.T) {
	store := NewStore(1)
	h := NewHandler(store)
	h.now = func() time.Time { return time.Date(2026, 8, 19, 15, 0, 0, 0, time.UTC) }
	id := "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee"
	body := fmt.Sprintf(`{"participant_id":%q,"period":"2026-08-19","tokens":10,"client_version":"0.5.0"}`, id)
	req := httptest.NewRequest(http.MethodPost, "/v1/community/usage", strings.NewReader(body))
	req.RemoteAddr = "198.51.100.77:54321"
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Forwarded-For", "203.0.113.50")
	rec := httptest.NewRecorder()
	h.Mux().ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status %d", rec.Code)
	}

	dump := dumpStore(store)
	for _, leak := range []string{"198.51.100.77", "203.0.113.50", "RemoteAddr"} {
		if strings.Contains(dump, leak) {
			t.Fatalf("store persisted %q: %s", leak, dump)
		}
	}
	store.mu.Lock()
	for pid := range store.days {
		if pid != id {
			store.mu.Unlock()
			t.Fatalf("unexpected participant key %q", pid)
		}
	}
	for pid := range store.hits {
		if pid != id {
			store.mu.Unlock()
			t.Fatalf("hits keyed by %q not participant_id", pid)
		}
	}
	store.mu.Unlock()

	req = httptest.NewRequest(http.MethodGet, "/v1/community/rank?participant_id="+id+"&period=2026-08-19&metric=tokens", nil)
	req.RemoteAddr = "198.51.100.77:54321"
	rec = httptest.NewRecorder()
	h.Mux().ServeHTTP(rec, req)
	out := rec.Body.String()
	for _, leak := range []string{"198.51.100.77", "203.0.113.50"} {
		if strings.Contains(out, leak) {
			t.Fatalf("rank leaked %q: %s", leak, out)
		}
	}
}

func dumpStore(s *Store) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	raw, err := json.Marshal(struct {
		Days map[string]map[string]dayRow `json:"days"`
		Left map[string]bool              `json:"left"`
		Hits map[string][]time.Time       `json:"hits"`
	}{s.days, s.left, s.hits})
	if err != nil {
		return err.Error()
	}
	return string(raw)
}

func TestStoreMinParticipantsIgnoresZeroTokenRow(t *testing.T) {
	s := NewStore(20)
	day := "2026-08-19"
	for i := 0; i < 19; i++ {
		id := fmt.Sprintf("aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeee%02d", i)
		if err := s.Put(Upload{ParticipantID: id, Period: day, Tokens: 10, ClientVersion: "0.5.0"}); err != nil {
			t.Fatal(err)
		}
	}
	lead := "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeee00"
	st := s.Rank(lead, PeriodToday, day, MetricTokens)
	if st.Status != StatusInsufficientParticipants || st.Rank != 0 || st.Display != "" {
		t.Fatalf("19 %+v", st)
	}
	if err := s.Put(Upload{ParticipantID: "bbbbbbbb-bbbb-4ccc-8ddd-eeeeeeeeeeee", Period: day, Tokens: 10, ClientVersion: "0.5.0"}); err != nil {
		t.Fatal(err)
	}
	st = s.Rank(lead, PeriodToday, day, MetricTokens)
	if st.Status != StatusOK || st.Display != "#1 / 20" {
		t.Fatalf("20 %+v", st)
	}
	if err := s.Put(Upload{ParticipantID: "cccccccc-bbbb-4ccc-8ddd-eeeeeeeeeeee", Period: day, Tokens: 0, ClientVersion: "0.5.0"}); err != nil {
		t.Fatal(err)
	}
	st = s.Rank(lead, PeriodToday, day, MetricTokens)
	if st.Participants != 20 || st.Display != "#1 / 20" {
		t.Fatalf("zero row must not join the board: %+v", st)
	}
}

func TestClientZeroTokenDayDoesNotUpload(t *testing.T) {
	var posts int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/community/usage" {
			posts++
			w.WriteHeader(http.StatusNoContent)
			return
		}
		_ = json.NewEncoder(w).Encode(Standing{Status: StatusOK, Rank: 1, Participants: 20, Display: "#1 / 20", SelfReported: true})
	}))
	t.Cleanup(srv.Close)
	c := &Client{
		BaseURL:  srv.URL,
		File:     &File{ParticipantID: "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee", Enabled: true},
		Version:  "0.5.0",
		MinCache: time.Millisecond,
		HTTP:     srv.Client(),
	}
	now := time.Date(2026, 8, 19, 15, 0, 0, 0, time.UTC)
	_ = c.Sync(context.Background(), []event.UsageEvent{{
		Vendor: "anthropic", Model: "claude-opus-4.6", Miss: 1000, Timestamp: now.Add(-24 * time.Hour),
	}}, now, time.UTC)
	if posts != 0 {
		t.Fatalf("yesterday-only must not upload today: posts=%d", posts)
	}
}

func TestClientCacheSkipsSecondUpload(t *testing.T) {
	var posts int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/community/usage" {
			posts++
			w.WriteHeader(http.StatusNoContent)
			return
		}
		_ = json.NewEncoder(w).Encode(Standing{Status: StatusOK, Rank: 1, Participants: 20, Display: "#1 / 20", SelfReported: true})
	}))
	t.Cleanup(srv.Close)
	c := &Client{
		BaseURL:  srv.URL,
		File:     &File{ParticipantID: "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee", Enabled: true},
		Version:  "0.5.0",
		MinCache: time.Hour,
		HTTP:     srv.Client(),
	}
	now := time.Date(2026, 8, 19, 15, 0, 0, 0, time.UTC)
	events := []event.UsageEvent{{Vendor: "anthropic", Model: "claude-opus-4.6", Miss: 1000, Timestamp: now}}
	_ = c.Sync(context.Background(), events, now, time.UTC)
	_ = c.Sync(context.Background(), events, now, time.UTC)
	if posts != 1 {
		t.Fatalf("cache failed: posts=%d", posts)
	}
}

func TestHandlerRejectsStalePeriod(t *testing.T) {
	h := testHandler(1)
	id := "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee"
	body := fmt.Sprintf(`{"participant_id":%q,"period":"2020-01-01","tokens":10,"client_version":"0.5.0"}`, id)
	req := httptest.NewRequest(http.MethodPost, "/v1/community/usage", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.Mux().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("stale period %d", rec.Code)
	}
}
