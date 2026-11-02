package cli

import (
	"fmt"
	"runtime"
	"strings"

	"github.com/rainhuang0220/whereToken/internal/adapter"
	"github.com/rainhuang0220/whereToken/internal/adapter/cursor"
	"github.com/rainhuang0220/whereToken/internal/event"
	"github.com/rainhuang0220/whereToken/internal/metric"
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
		Probe:       true,
		Now:         a.Now,
	}
	roots := ad.Discover(home)
	if len(roots) == 0 {
		fmt.Fprintln(a.Stdout, "cursor connect: on")
		fmt.Fprintln(a.Stdout, "session: missing")
		return ExitOK
	}
	var evs []event.UsageEvent
	var apiErr error
	for _, root := range roots {
		err := ad.Parse(root, func(e event.UsageEvent) {
			evs = append(evs, e)
		}, func(event.TurnEvent) {})
		if err != nil && apiErr == nil {
			apiErr = err
		}
	}
	var api []event.UsageEvent
	var from, to string
	for _, e := range evs {
		if e.Derivation != event.DeriveProviderAPI {
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
	if apiErr != nil {
		fmt.Fprintln(a.Stderr, apiErr.Error())
		if len(api) == 0 {
			return ExitFail
		}
	}
	if strings.Contains(strings.ToLower(fmt.Sprint(apiErr)), "eyj") {
		return ExitFail
	}
	return ExitOK
}
