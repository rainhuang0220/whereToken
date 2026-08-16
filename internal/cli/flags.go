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

	var toolFlag, vendorFlag, modelFlag string
	var claude, kimi, codex, opencode, trae, cursor bool
	fs := newFlagSet(&f, &toolFlag, &vendorFlag, &modelFlag, &claude, &kimi, &codex, &opencode, &trae, &cursor)
	if err := parseFlagSet(fs, &f, rest); err != nil {
		return Flags{}, err
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
		leftover, err := applyTrailingCommand(&f, extra)
		if err != nil {
			return Flags{}, err
		}
		if f.Command == CommandCompletion || f.Help || f.Version {
			if len(leftover) > 0 {
				return Flags{}, usageError{msg: fmt.Sprintf("unexpected extra argument %q\ntry `wheretoken --help`", leftover[0])}
			}
			return f, nil
		}
		if len(leftover) > 0 {
			fs2 := newFlagSet(&f, &toolFlag, &vendorFlag, &modelFlag, &claude, &kimi, &codex, &opencode, &trae, &cursor)
			if err := parseFlagSet(fs2, &f, leftover); err != nil {
				return Flags{}, err
			}
			if extra2 := fs2.Args(); len(extra2) > 0 {
				return Flags{}, usageError{msg: fmt.Sprintf("unexpected extra argument %q\ntry `wheretoken --help`", extra2[0])}
			}
		}
	}

	if f.Width < 0 {
		return Flags{}, usageError{msg: "invalid --width (must be >= 0)\ntry `wheretoken --help`"}
	}
	if f.Port <= 0 || f.Port > 65535 {
		return Flags{}, usageError{msg: "invalid --port\ntry `wheretoken --help`"}
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
	if strings.TrimSpace(toolFlag) != "" {
		id, ok := metric.LookupSource(toolFlag)
		if !ok {
			return Flags{}, unknownName("tool", toolFlag, suggestKnown(toolFlag, metric.KnownSourceIDs()), metric.KnownSourceIDs())
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

	if strings.TrimSpace(vendorFlag) != "" {
		id, ok := vendor.LookupName(vendorFlag)
		if !ok {
			return Flags{}, unknownName("vendor", vendorFlag, suggestKnown(vendorFlag, vendor.KnownIDs()), vendor.KnownIDs())
		}
		f.Vendor = id
	}
	f.Model = strings.TrimSpace(modelFlag)
	return f, nil
}

func newFlagSet(f *Flags, toolFlag, vendorFlag, modelFlag *string, claude, kimi, codex, opencode, trae, cursor *bool) *flag.FlagSet {
	fs := flag.NewFlagSet("wheretoken", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.BoolVar(&f.Help, "h", f.Help, "")
	fs.BoolVar(&f.Help, "help", f.Help, "")
	fs.BoolVar(&f.Version, "version", f.Version, "")
	fs.BoolVar(&f.Version, "V", f.Version, "")
	fs.BoolVar(&f.JSON, "json", f.JSON, "")
	fs.BoolVar(&f.Today, "today", f.Today, "")
	fs.BoolVar(&f.ASCII, "ascii", f.ASCII, "")
	fs.BoolVar(&f.NoColor, "no-color", f.NoColor, "")
	fs.BoolVar(&f.Quiet, "quiet", f.Quiet, "")
	fs.BoolVar(&f.Quiet, "q", f.Quiet, "")
	fs.BoolVar(&f.Offline, "offline", f.Offline, "")
	fs.StringVar(&f.Home, "home", f.Home, "")
	fs.IntVar(&f.Port, "port", f.Port, "")
	fs.IntVar(&f.Width, "width", f.Width, "")
	fs.StringVar(toolFlag, "tool", *toolFlag, "")
	fs.StringVar(vendorFlag, "vendor", *vendorFlag, "")
	fs.StringVar(modelFlag, "model", *modelFlag, "")
	fs.BoolVar(claude, "claude", *claude, "")
	fs.BoolVar(kimi, "kimi", *kimi, "")
	fs.BoolVar(codex, "codex", *codex, "")
	fs.BoolVar(opencode, "opencode", *opencode, "")
	fs.BoolVar(trae, "trae", *trae, "")
	fs.BoolVar(cursor, "cursor", *cursor, "")
	return fs
}

func parseFlagSet(fs *flag.FlagSet, f *Flags, args []string) error {
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			f.Help = true
			return nil
		}
		return usageError{msg: strings.TrimSpace(err.Error()) + "\ntry `wheretoken --help`"}
	}
	return nil
}

func applyTrailingCommand(f *Flags, extra []string) ([]string, error) {
	rest := extra[1:]
	switch extra[0] {
	case "help":
		f.Help = true
		return rest, nil
	case "version":
		f.Version = true
		return rest, nil
	case "serve":
		f.Command = CommandServe
		return rest, nil
	case "scan":
		f.Command = CommandScan
		f.JSON = true
		return rest, nil
	case "sources":
		f.Command = CommandSources
		return rest, nil
	case "completion":
		f.Command = CommandCompletion
		if len(rest) > 0 && !strings.HasPrefix(rest[0], "-") {
			f.CompletionShell = rest[0]
			rest = rest[1:]
		}
		return rest, nil
	default:
		return nil, usageError{msg: fmt.Sprintf("unknown command %q\ntry `wheretoken --help`", extra[0])}
	}
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
