package cli

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/rainhuang0220/whereToken/internal/adapter"
	"github.com/rainhuang0220/whereToken/internal/adapter/cursor"
	"github.com/rainhuang0220/whereToken/internal/credstore"
	"github.com/rainhuang0220/whereToken/internal/event"
	"github.com/rainhuang0220/whereToken/internal/price"
	"github.com/rainhuang0220/whereToken/internal/scan"
	"github.com/rainhuang0220/whereToken/internal/syncagg"
)

func (a *App) runSync(flags Flags, home adapter.Home) int {
	days, _, err := a.syncOnce(flags, home)
	if err != nil {
		fmt.Fprintln(a.Stderr, err.Error())
		return ExitFail
	}
	fmt.Fprintf(a.Stdout, "synced %d model-days\n", days)
	return ExitOK
}

func (a *App) syncOnce(flags Flags, home adapter.Home) (days, sources int, err error) {
	cs := a.credStore(home)
	tok, err := cs.Get(credstore.KeyDeviceToken)
	if err != nil {
		return 0, 0, fmt.Errorf("not logged in; run wheretoken login")
	}
	devID, _ := cs.Get(credstore.KeyDeviceID)
	hmacB64, err := cs.Get(credstore.KeyHMAC)
	if err != nil {
		return 0, 0, fmt.Errorf("missing pairing key; run wheretoken login")
	}
	key, err := base64.RawURLEncoding.DecodeString(hmacB64)
	if err != nil || len(key) != 32 {
		return 0, 0, fmt.Errorf("invalid pairing key; run wheretoken login")
	}
	res := a.doScan(home, flags.Quiet, flags.Offline, flags.ASCII)
	batch, err := syncagg.Build(syncagg.BuildInput{
		DeviceID:            devID,
		Timezone:            a.Loc.String(),
		PriceCatalogVersion: price.CardVersion,
		ClientVersion:       a.Version,
		GeneratedAt:         a.Now(),
		Loc:                 a.Loc,
		UserHashKey:         key,
		AccountStableID:     a.accountIDs(home),
		Revision:            a.Now().Unix(),
		IdempotencyKey:      fmt.Sprintf("%d", a.Now().UnixNano()),
		Events:              res.Events,
		Turns:               res.Turns,
		Sources:             sourceInputs(res),
	})
	if err != nil {
		return 0, 0, err
	}
	raw, err := json.Marshal(batch)
	if err != nil {
		return 0, 0, err
	}
	req, err := http.NewRequest(http.MethodPut, a.hostedBase()+"/api/v1/sync/batch", strings.NewReader(string(raw)))
	if err != nil {
		return 0, 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tok)
	resp, err := a.doHTTP(req)
	if err != nil {
		return 0, 0, err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<16))
	if resp.StatusCode >= 400 {
		return 0, 0, fmt.Errorf("sync failed: HTTP %d", resp.StatusCode)
	}
	n := 0
	for _, src := range batch.Sources {
		if src.Detected {
			n++
		}
	}
	if n == 0 {
		n = len(batch.Sources)
	}
	return len(batch.DailyModelUsage), n, nil
}

func (a *App) accountIDs(home adapter.Home) map[string]string {
	sub := cursor.Adapter{}.AccountSubject(home)
	if sub == "" {
		return map[string]string{}
	}
	return map[string]string{"cursor": sub}
}

func sourceInputs(res scan.Result) []syncagg.SourceInput {
	var out []syncagg.SourceInput
	for _, st := range scan.Diagnose(res) {
		status := "ok"
		switch {
		case strings.Contains(st.Error, "未找到本机登录态"):
			status = "auth_missing"
		case strings.Contains(st.Error, "已失效") || strings.Contains(st.Error, "加密存储"):
			status = "auth_expired"
		case !st.Detected:
			continue
		case !st.Usage && st.Quality == event.QualityAbsent:
			status = "absent"
		case st.Quality == event.QualityDegraded:
			status = "degraded"
		}
		out = append(out, syncagg.SourceInput{
			Tool:     st.ID,
			Detected: st.Detected,
			Status:   status,
			Quality:  string(st.Quality),
		})
	}
	return out
}
