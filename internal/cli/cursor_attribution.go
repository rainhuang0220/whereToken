package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/rainhuang0220/whereToken/internal/adapter"
	"github.com/rainhuang0220/whereToken/internal/adapter/cursor"
	"github.com/rainhuang0220/whereToken/internal/proclock"
	"github.com/rainhuang0220/whereToken/internal/scan"
)

func (a *App) cursorBackfill(flags Flags, home adapter.Home) int {
	from, err := time.Parse("2006-01-02", flags.From)
	to, toErr := time.Parse("2006-01-02", flags.To)
	if err != nil || toErr != nil || to.Before(from) || to.Sub(from) > 2*366*24*time.Hour || flags.Offline || flags.Today || flags.Since != "" || flags.Tool != "" || flags.Vendor != "" || flags.Model != "" {
		fmt.Fprintln(a.Stderr, "cursor backfill requires --from YYYY-MM-DD --to YYYY-MM-DD (inclusive UTC; maximum two years), and an online connected account")
		return ExitUsage
	}
	unlock, ok, err := proclock.TryLock(a.scanLockPath(home))
	if err != nil || !ok {
		fmt.Fprintln(a.Stderr, "Cursor 归属回填未执行：扫描锁正忙或不可用")
		return ExitFail
	}
	defer unlock()
	fmt.Fprintln(a.Stderr, "按完整 UTC 窗口读取当前已关联账号的真实模型明细；仅回填现有分量以内的归属，不改变历史 Token 总量。系统可能请求一次 Cursor 会话授权。")
	got, fetchErr := scan.BackfillCursorAttribution(home, cursor.Adapter{ConsentPath: cursor.ConsentPath(home), AllowPrompt: true, Now: a.Now}, flags.From, flags.To)
	if flags.JSON {
		if err := json.NewEncoder(a.Stdout).Encode(got); err != nil {
			return ExitFail
		}
	} else {
		fmt.Fprintf(a.Stdout, "UTC requested: %s..%s\ncompleted windows: %d\nmetered rows: %d\nmeasured tokens: %d\nallocated days: %d\nrejected days: %d\nmissing baseline days: %d\nbaseline before: %d\nbaseline after: %d\n", flags.From, flags.To, len(got.Windows), got.MeteredRows, got.MeasuredTokens, got.AllocatedDays, got.RejectedDays, got.MissingDays, got.Before, got.After)
	}
	if fetchErr != nil {
		fmt.Fprintln(a.Stderr, fetchErr.Error())
		return ExitFail
	}
	return ExitOK
}
