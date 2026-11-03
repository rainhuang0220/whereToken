package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"

	"github.com/rainhuang0220/whereToken/internal/adapter"
	"github.com/rainhuang0220/whereToken/internal/adapter/cursor"
	"github.com/rainhuang0220/whereToken/internal/event"
	"github.com/rainhuang0220/whereToken/internal/ledger"
	"github.com/rainhuang0220/whereToken/internal/metric"
	"github.com/rainhuang0220/whereToken/internal/scan"
	"golang.org/x/term"
)

func (a *App) runCursor(flags Flags, home adapter.Home) int {
	switch flags.CursorAction {
	case "connect":
		return a.cursorConnect(home)
	case "disconnect":
		return a.cursorDisconnect(home)
	case "", "status":
		return a.cursorStatus(home)
	default:
		fmt.Fprintf(a.Stderr, "unknown cursor action %q\ntry `wheretoken --help`\n", flags.CursorAction)
		return ExitUsage
	}
}

func (a *App) cursorStatus(home adapter.Home) int {
	path := cursor.ConsentPath(home)
	on := "off"
	if cursor.ConsentOn(path) {
		on = "on"
	}
	fmt.Fprintf(a.Stdout, "cursor connect: %s\n", on)
	fmt.Fprintf(a.Stdout, "session: %s\n", cursor.StoredSessionKind(home))
	if on == "off" && cursor.StoredSessionKind(home) == "sealed" {
		fmt.Fprintln(a.Stdout, "账号用量不可用：Cursor 把会话封存在系统密钥中，本程序不读取该密钥")
	}
	return ExitOK
}

func (a *App) cursorDisconnect(home adapter.Home) int {
	if err := cursor.RemoveConsent(cursor.ConsentPath(home)); err != nil {
		fmt.Fprintln(a.Stderr, err.Error())
		return ExitFail
	}
	fmt.Fprintln(a.Stdout, "cursor connect: off")
	fmt.Fprintln(a.Stdout, "后续扫描不再读取 Cursor 会话。")
	fmt.Fprintln(a.Stdout, "本程序的同意记录已删除。系统里已经记住的允许还在。要去掉那项允许，在钥匙串访问中移除本程序后再运行 wheretoken cursor connect。")
	return ExitOK
}

func (a *App) cursorConnect(home adapter.Home) int {
	path := cursor.ConsentPath(home)
	fmt.Fprintln(a.Stderr, "这一步会读取操作系统保护的 Cursor 会话，用来统计账号 token。系统可能会询问是否允许本程序读取。允许一次后，同一份未改动的程序在后台不再询问。未签名的升级可能会再问一次。同一开发者签名的升级通常不必再问。")
	if runtime.GOOS == "linux" {
		fmt.Fprintln(a.Stderr, "Linux 上的密封会话无法读取。明文 JWT 仍可统计。连接不会把密封会话变成可用账本。")
	}
	if runtime.GOOS == "windows" {
		fmt.Fprintln(a.Stderr, "Windows 上的密封会话解封仍是实验性的，尚未在真实 DPAPI 登录上验收，不能当作已经全面支持。")
	}
	if err := cursor.WriteConsent(path); err != nil {
		fmt.Fprintln(a.Stderr, err.Error())
		return ExitFail
	}
	ad := cursor.Adapter{
		ConsentPath: path,
		AllowPrompt: true,
		Probe:       false,
		Now:         a.Now,
	}
	roots := ad.Discover(home)
	if len(roots) == 0 {
		fmt.Fprintln(a.Stdout, "cursor connect: on")
		fmt.Fprintln(a.Stdout, "session: missing")
		return ExitOK
	}
	sub := ad.Subject(home)
	lp, err := ledger.PathFor(home)
	if err != nil {
		fmt.Fprintln(a.Stderr, "账号用量不可用：UNBOUND_HISTORY")
		return ExitFail
	}
	lg, err := ledger.Open(lp)
	if err != nil {
		fmt.Fprintln(a.Stderr, "这次没有读取账号用量。")
		return ExitFail
	}
	code, stop := a.confirmCursorHistory(lg, sub)
	_ = lg.Close()
	if stop {
		return code
	}
	return a.finishCursorConnect(home, ad)
}

func (a *App) confirmCursorHistory(lg *ledger.Ledger, sub string) (int, bool) {
	has, err := lg.HasObservation("cursor")
	if err != nil {
		fmt.Fprintln(a.Stderr, "这次没有读取账号用量。")
		return ExitFail, true
	}
	if !has {
		return 0, false
	}
	if sub == "" {
		fmt.Fprintln(a.Stderr, "当前账号没有可读的身份标识。这次没有关联历史。")
		return ExitFail, true
	}
	prev, err := lg.Meta(cursor.AccountSubjectMeta)
	if err != nil {
		fmt.Fprintln(a.Stderr, "账号用量不可用：UNBOUND_HISTORY")
		return ExitFail, true
	}
	if prev != "" && prev != sub {
		fmt.Fprintln(a.Stderr, cursor.AccountMismatchError().Error())
		return ExitFail, true
	}
	if prev == sub {
		return 0, false
	}
	decision, err := lg.Meta(cursor.HistoryDecisionMeta)
	if err != nil {
		fmt.Fprintln(a.Stderr, "账号用量不可用：UNBOUND_HISTORY")
		return ExitFail, true
	}
	who, err := lg.Meta(cursor.HistoryDecisionSubjectMeta)
	if err != nil {
		fmt.Fprintln(a.Stderr, "账号用量不可用：UNBOUND_HISTORY")
		return ExitFail, true
	}
	if decision == cursor.HistoryConfirmed && who == sub {
		return 0, false
	}
	if decision == cursor.HistoryDeclined && who == sub {
		return 0, false
	}
	if who != "" && who != sub {
		fmt.Fprintln(a.Stderr, "当前账号和已经记下的选择不是同一个。旧历史保持独立，这次没有写入。")
		fmt.Fprintln(a.Stderr, cursor.HistoryHeldError().Error())
		return ExitFail, true
	}
	prompt, err := scan.DescribeCursorHistory(lg, sub)
	if err != nil {
		fmt.Fprintln(a.Stderr, "账号用量不可用：UNBOUND_HISTORY")
		return ExitFail, true
	}
	fmt.Fprint(a.Stderr, prompt)
	line, ok := a.readHistoryAnswer()
	if !ok {
		fmt.Fprintln(a.Stderr, "请在终端里输入 yes 或 no。这次没有自动关联。")
		return ExitFail, true
	}
	if err := scan.ApplyCursorHistory(lg, sub, line, a.Now()); err != nil {
		fmt.Fprintln(a.Stderr, err.Error())
		return ExitFail, true
	}
	return 0, false
}

func (a *App) readHistoryAnswer() (string, bool) {
	interactive := false
	switch {
	case a.StdinInteractive != nil:
		interactive = a.StdinInteractive()
	case a.Stdin != nil:
		interactive = true
	default:
		interactive = term.IsTerminal(int(os.Stdin.Fd()))
	}
	if !interactive {
		return "", false
	}
	var r io.Reader = os.Stdin
	if a.Stdin != nil {
		r = a.Stdin
	}
	line, err := bufio.NewReader(r).ReadString('\n')
	if err != nil && strings.TrimSpace(line) == "" {
		return "", false
	}
	return line, true
}

func (a *App) finishCursorConnect(home adapter.Home, ad cursor.Adapter) int {
	var res scan.Result
	if a.Scan != nil {
		res = a.Scan(home)
	} else {
		res = scan.Run(home, []adapter.Adapter{ad})
	}
	var api []event.UsageEvent
	var from, to string
	for _, e := range res.Events {
		if e.Source != "cursor" || e.Derivation != event.DeriveProviderAPI {
			continue
		}
		api = append(api, e)
		if e.Timestamp.IsZero() || e.RequestID == "cursor-api:measured-zero" {
			continue
		}
		day := e.Timestamp.UTC().Format("2006-01-02")
		if from == "" || day < from {
			from = day
		}
		if to == "" || day > to {
			to = day
		}
	}
	fmt.Fprintln(a.Stdout, "cursor connect: on")
	fmt.Fprintf(a.Stdout, "session: %s\n", cursor.StoredSessionKind(home))
	if from != "" {
		fmt.Fprintf(a.Stdout, "window: %s..%s\n", from, to)
	}
	sum := metric.Aggregate(api, nil)
	fmt.Fprintf(a.Stdout, "events: %d\n", len(api))
	fmt.Fprintf(a.Stdout, "input: %d\n", sum.All.Miss)
	fmt.Fprintf(a.Stdout, "cache_read: %d\n", sum.All.CacheRead)
	fmt.Fprintf(a.Stdout, "cache_write: %d\n", sum.All.CacheCreate)
	fmt.Fprintf(a.Stdout, "output: %d\n", sum.All.Output)
	fmt.Fprintf(a.Stdout, "total: %d\n", sum.All.Total())
	apiErr := strings.Join(res.Errors, "\n")
	if apiErr != "" {
		fmt.Fprintln(a.Stderr, apiErr)
		if len(api) == 0 {
			return ExitFail
		}
	}
	if strings.Contains(strings.ToLower(apiErr), "eyj") {
		return ExitFail
	}
	return ExitOK
}
