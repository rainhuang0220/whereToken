package cli

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/rainhuang0220/whereToken/internal/adapter"
	"github.com/rainhuang0220/whereToken/internal/adapter/cursor"
	"github.com/rainhuang0220/whereToken/internal/credstore"
	"github.com/rainhuang0220/whereToken/internal/event"
	"github.com/rainhuang0220/whereToken/internal/price"
	"github.com/rainhuang0220/whereToken/internal/publicprofile"
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
	tok, devID, key, err := a.syncCreds(home)
	if err != nil {
		return 0, 0, err
	}
	res := a.doScan(home, flags.Quiet, flags.Offline, flags.ASCII)
	return a.syncResult(flags, home, res, tok, devID, key)
}

func (a *App) syncCreds(home adapter.Home) (token, deviceID string, key []byte, err error) {
	cs := a.credStore(home)
	token, err = cs.Get(credstore.KeyDeviceToken)
	if err != nil {
		return "", "", nil, fmt.Errorf("not logged in; run wheretoken login")
	}
	deviceID, _ = cs.Get(credstore.KeyDeviceID)
	hmacB64, err := cs.Get(credstore.KeyHMAC)
	if err != nil {
		return "", "", nil, fmt.Errorf("missing pairing key; run wheretoken login")
	}
	key, err = base64.RawURLEncoding.DecodeString(hmacB64)
	if err != nil || len(key) != 32 {
		return "", "", nil, fmt.Errorf("invalid pairing key; run wheretoken login")
	}
	return token, deviceID, key, nil
}

func (a *App) syncResult(flags Flags, home adapter.Home, res scan.Result, tok, devID string, key []byte) (days, sources int, err error) {
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
	if err := a.uploadPublicProjection(flags, home, res, tok); err != nil {
		fmt.Fprintf(a.Stderr, "public profile not updated: %s\n", err.Error())
	}
	return len(batch.DailyModelUsage), n, nil
}

func (a *App) maybeHostedSync(flags Flags, home adapter.Home, res scan.Result) {
	tok, devID, key, err := a.syncCreds(home)
	if err != nil {
		return
	}
	if _, _, err := a.syncResult(flags, home, res, tok, devID, key); err != nil {
		fmt.Fprintf(a.Stderr, "hosted sync skipped: %s\n", err.Error())
	}
}

func (a *App) startHostedRefresh(home adapter.Home, offline bool) {
	go func() {
		ticker := time.NewTicker(15 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			if _, _, _, err := a.syncCreds(home); err != nil {
				continue
			}
			if _, _, err := a.syncOnce(Flags{Quiet: true, Offline: offline}, home); err != nil {
				fmt.Fprintf(a.Stderr, "hosted sync skipped: %s\n", err.Error())
			}
		}
	}()
}

func (a *App) uploadPublicProjection(flags Flags, home adapter.Home, res scan.Result, token string) error {
	if flags.Offline || res.Offline {
		return nil
	}
	snap, err := publicprofile.Build(publicprofile.Input{
		Events:       res.Events,
		Turns:        res.Turns,
		Now:          a.Now(),
		Loc:          a.Loc,
		Version:      a.Version,
		PortraitSeed: a.PortraitSeed(home),
		Offline:      false,
		Errors:       res.Errors,
	})
	if err != nil {
		return err
	}
	if snap.DataStatus == publicprofile.StatusUnavailable {
		return nil
	}
	if err := publicprofile.Validate(snap); err != nil {
		return err
	}
	raw, err := publicprofile.Marshal(snap)
	if err != nil {
		return err
	}
	put, err := a.putSanitizedProjection(context.Background(), token, raw)
	if err != nil {
		return err
	}
	if put.Status >= 400 {
		return fmt.Errorf("HTTP %d", put.Status)
	}
	return nil
}

func (a *App) putSanitizedProjection(ctx context.Context, token string, raw []byte) (publicprofile.PutResult, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, a.hostedBase()+"/api/v1/sync/public-profile", strings.NewReader(string(raw)))
	if err != nil {
		return publicprofile.PutResult{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := a.doHTTP(req)
	if err != nil {
		return publicprofile.PutResult{}, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	if err != nil {
		return publicprofile.PutResult{}, err
	}
	kept := false
	if resp.StatusCode < 400 && len(body) > 0 {
		var env struct {
			Kept string `json:"kept"`
		}
		if json.Unmarshal(body, &env) == nil && env.Kept == "previous" {
			kept = true
		}
	}
	return publicprofile.PutResult{Status: resp.StatusCode, KeptPrevious: kept}, nil
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
