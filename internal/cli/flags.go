package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/rainhuang0220/whereToken/internal/metric"
	"github.com/rainhuang0220/whereToken/internal/vendor"
)

const (
	CommandReport     = "report"
	CommandServe      = "serve"
	CommandScan       = "scan"
	CommandSources    = "sources"
	CommandCompletion = "completion"
)

const (
	ExitOK    = 0
	ExitFail  = 1
	ExitUsage = 2
)

type Flags struct {
	Command         string
	Help, Version   bool
	JSON, Today     bool
	ASCII           bool
	NoColor         bool
	Quiet           bool
	Offline         bool
	Tool, Vendor    string
	Model, Home     string
	Port            int
	Width           int
	CompletionShell string
}

type usageError struct {
	msg string
}

func (e usageError) Error() string { return e.msg }

func IsUsage(err error) bool {
	var u usageError
	return errors.As(err, &u)
}

func Usage(err error) bool { return IsUsage(err) }

func Parse(args []string) (Flags, error) {
	f := Flags{Command: CommandReport, Port: 8787}
	if len(args) == 0 {
		return f, nil
	}
	rest := args
	if !strings.HasPrefix(args[0], "-") {
		switch args[0] {
		case "help":
			f.Help = true
			return f, nil
		case "version":
			f.Version = true
			return f, nil
		case "serve":
			f.Command = CommandServe
		case "scan":
			f.Command = CommandScan
			f.JSON = true
		case "sources":
			f.Command = CommandSources
		case "completion":
			f.Command = CommandCompletion
		default:
			return Flags{}, usageError{msg: fmt.Sprintf("unknown command %q\ntry `wheretoken --help`", args[0])}
		}
		rest = args[1:]
	}

	fs := flag.NewFlagSet("wheretoken", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.BoolVar(&f.Help, "h", false, "")
	fs.BoolVar(&f.Help, "help", false, "")
	fs.BoolVar(&f.Version, "version", false, "")
	fs.BoolVar(&f.Version, "V", false, "")
	fs.BoolVar(&f.JSON, "json", f.JSON, "")
	fs.BoolVar(&f.Today, "today", false, "")
	fs.BoolVar(&f.ASCII, "ascii", false, "")
	fs.BoolVar(&f.NoColor, "no-color", false, "")
	fs.BoolVar(&f.Quiet, "quiet", false, "")
	fs.BoolVar(&f.Quiet, "q", false, "")
	fs.BoolVar(&f.Offline, "offline", false, "")
	fs.StringVar(&f.Home, "home", "", "")
	fs.IntVar(&f.Port, "port", f.Port, "")
	fs.IntVar(&f.Width, "width", 0, "")
	toolFlag := fs.String("tool", "", "")
	vendorFlag := fs.String("vendor", "", "")
	modelFlag := fs.String("model", "", "")
	var claude, kimi, codex, opencode, trae, cursor bool
	fs.BoolVar(&claude, "claude", false, "")
	fs.BoolVar(&kimi, "kimi", false, "")
	fs.BoolVar(&codex, "codex", false, "")
	fs.BoolVar(&opencode, "opencode", false, "")
	fs.BoolVar(&trae, "trae", false, "")
	fs.BoolVar(&cursor, "cursor", false, "")
	if err := fs.Parse(rest); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			f.Help = true
			return f, nil
		}
		return Flags{}, usageError{msg: strings.TrimSpace(err.Error()) + "\ntry `wheretoken --help`"}
	}

	if f.Command == CommandCompletion {
		left := fs.Args()
		if len(left) > 0 {
			f.CompletionShell = left[0]
			left = left[1:]
		}
		if len(left) > 0 {
			return Flags{}, usageError{msg: fmt.Sprintf("unexpected extra argument %q", left[0])}
		}
		return f, nil
	}
	if extra := fs.Args(); len(extra) > 0 {
		return Flags{}, usageError{msg: fmt.Sprintf("unexpected extra argument %q\ntry `wheretoken --help`", extra[0])}
	}

	var tools []string
	add := func(id string) {
		tools = append(tools, id)
	}
	if claude {
		add("claude")
	}
	if kimi {
		add("kimi")
	}
	if codex {
		add("codex")
	}
	if opencode {
		add("opencode")
	}
	if trae {
		add("trae")
	}
	if cursor {
		add("cursor")
	}
	if strings.TrimSpace(*toolFlag) != "" {
		id, ok := metric.LookupSource(*toolFlag)
		if !ok {
			return Flags{}, unknownName("tool", *toolFlag, suggestKnown(*toolFlag, metric.KnownSourceIDs()), metric.KnownSourceIDs())
		}
		add(id)
	}
	uniq := unique(tools)
	if len(uniq) > 1 {
		return Flags{}, usageError{msg: fmt.Sprintf("conflicting tools: %s", strings.Join(uniq, ", "))}
	}
	if len(uniq) == 1 {
		f.Tool = uniq[0]
	}

	if strings.TrimSpace(*vendorFlag) != "" {
		id, ok := vendor.LookupName(*vendorFlag)
		if !ok {
			return Flags{}, unknownName("vendor", *vendorFlag, suggestKnown(*vendorFlag, vendor.KnownIDs()), vendor.KnownIDs())
		}
		f.Vendor = id
	}
	f.Model = strings.TrimSpace(*modelFlag)
	return f, nil
}

func unique(in []string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, s := range in {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}
