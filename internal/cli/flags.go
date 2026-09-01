package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/rainhuang0220/whereToken/internal/metric"
	"github.com/rainhuang0220/whereToken/internal/vendor"
)

const (
	CommandReport     = "report"
	CommandServe      = "serve"
	CommandScan       = "scan"
	CommandSources    = "sources"
	CommandDoctor     = "doctor"
	CommandRebuild    = "rebuild"
	CommandUpdate     = "update"
	CommandUninstall  = "uninstall"
	CommandCompletion = "completion"
	CommandCommunity  = "community"
	CommandPricing    = "pricing"
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
	Since, From, To string
	Port            int
	Width           int
	CompletionShell string
	RankPeriod      string
	NoCommunity     bool
	CommunityAction string
}

type usageError struct {
	msg string
}

func (e usageError) Error() string { return e.msg }

func IsUsage(err error) bool {
	var u usageError
	return errors.As(err, &u)
}

func Parse(args []string) (Flags, error) {
	f := Flags{Command: CommandReport, Port: 8787, RankPeriod: "today"}
	if len(args) == 0 {
		return f, nil
	}
	rest := args
	if !strings.HasPrefix(args[0], "-") {
		if !applyCommandWord(&f, args[0]) {
			return Flags{}, usageError{msg: fmt.Sprintf("unknown command %q\ntry `wheretoken --help`", args[0])}
		}
		if f.Help || f.Version {
			return f, nil
		}
		rest = args[1:]
	}

	var tf toolFlags
	fs := newFlagSet(&f, &tf)
	if err := parseFlagSet(fs, &f, rest); err != nil {
		return Flags{}, err
	}

	if f.Command == CommandCompletion {
		if err := parseCompletionTail(&f, fs.Args()); err != nil {
			return Flags{}, err
		}
		return f, nil
	}
	if extra := fs.Args(); len(extra) > 0 {
		if f.Command == CommandCommunity {
			return finishCommunity(&f, extra)
		}
		leftover, err := applyTrailingCommand(&f, extra)
		if err != nil {
			return Flags{}, err
		}
		if f.Command == CommandCommunity {
			return finishCommunity(&f, leftover)
		}
		if f.Command == CommandCompletion {
			if err := parseCompletionTail(&f, leftover); err != nil {
				return Flags{}, err
			}
			return f, nil
		}
		if f.Help || f.Version {
			if len(leftover) > 0 {
				return Flags{}, usageError{msg: fmt.Sprintf("unexpected extra argument %q\ntry `wheretoken --help`", leftover[0])}
			}
			return f, nil
		}
		if len(leftover) > 0 {
			fs2 := newFlagSet(&f, &tf)
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
	if err := checkPort(f.Port); err != nil {
		return Flags{}, err
	}

	tools := tf.selected()
	if strings.TrimSpace(tf.tool) != "" {
		id, ok := metric.LookupSource(tf.tool)
		if !ok {
			return Flags{}, unknownName("tool", tf.tool, suggestKnown(tf.tool, metric.KnownSourceIDs()), metric.KnownSourceIDs())
		}
		tools = append(tools, id)
	}
	uniq := unique(tools)
	if len(uniq) > 1 {
		return Flags{}, usageError{msg: fmt.Sprintf("conflicting tools: %s", strings.Join(uniq, ", "))}
	}
	if len(uniq) == 1 {
		f.Tool = uniq[0]
	}

	if strings.TrimSpace(tf.vendor) != "" {
		id, ok := vendor.LookupName(tf.vendor)
		if !ok {
			return Flags{}, unknownName("vendor", tf.vendor, suggestKnown(tf.vendor, vendor.KnownIDs()), vendor.KnownIDs())
		}
		f.Vendor = id
	}
	f.Model = strings.TrimSpace(tf.model)
	if f.Command == CommandScan && (f.Today || f.Tool != "" || f.Vendor != "" || f.Model != "" || f.Since != "" || f.From != "" || f.To != "") {
		return Flags{}, usageError{msg: "scan --json is the observatory payload; table filters belong on `wheretoken --json`\ntry `wheretoken --help`"}
	}
	if f.Command == CommandPricing && (f.Today || f.Since != "" || f.From != "" || f.To != "" || f.Tool != "" || f.Offline || f.NoCommunity) {
		return Flags{}, usageError{msg: "pricing reads the built-in price card; only --vendor/--model/--json/--width apply\ntry `wheretoken --help`"}
	}
	if f.Today && (f.Since != "" || f.From != "" || f.To != "") {
		return Flags{}, usageError{msg: "use only one of --today, --since, or --from/--to\ntry `wheretoken --help`"}
	}
	if f.Since != "" && (f.From != "" || f.To != "") {
		return Flags{}, usageError{msg: "use only one of --today, --since, or --from/--to\ntry `wheretoken --help`"}
	}
	if f.Since != "" {
		if _, err := metric.ParseSince(f.Since); err != nil {
			return Flags{}, usageError{msg: err.Error() + "\ntry `wheretoken --help`"}
		}
	}
	if f.From != "" {
		if _, err := metric.ParseClock(f.From, time.Local, false); err != nil {
			return Flags{}, usageError{msg: err.Error() + "\ntry `wheretoken --help`"}
		}
	}
	if f.To != "" {
		if _, err := metric.ParseClock(f.To, time.Local, true); err != nil {
			return Flags{}, usageError{msg: err.Error() + "\ntry `wheretoken --help`"}
		}
	}
	if f.RankPeriod == "" {
		f.RankPeriod = "today"
	}
	switch f.RankPeriod {
	case "today", "all":
	default:
		return Flags{}, usageError{msg: "invalid --rank (today or all)\ntry `wheretoken --help`"}
	}
	if f.Command == CommandCommunity && f.CommunityAction == "" {
		f.CommunityAction = "status"
	}
	return f, nil
}

func checkPort(port int) error {
	if port <= 0 || port > 65535 {
		return usageError{msg: "invalid --port\ntry `wheretoken --help`"}
	}
	return nil
}

var shorthandIDs = []string{"claude", "kimi", "grok", "minimax", "openclaw", "codex", "opencode", "trae", "cursor"}

type toolFlags struct {
	tool, vendor, model string
	shorthand           map[string]*bool
}

func (t *toolFlags) bind(fs *flag.FlagSet) {
	fs.StringVar(&t.tool, "tool", t.tool, "")
	fs.StringVar(&t.vendor, "vendor", t.vendor, "")
	fs.StringVar(&t.model, "model", t.model, "")
	if t.shorthand == nil {
		t.shorthand = make(map[string]*bool, len(shorthandIDs))
		for _, id := range shorthandIDs {
			t.shorthand[id] = new(bool)
		}
	}
	for _, id := range shorthandIDs {
		fs.BoolVar(t.shorthand[id], id, *t.shorthand[id], "")
	}
}

func (t *toolFlags) selected() []string {
	var tools []string
	for _, id := range shorthandIDs {
		if p := t.shorthand[id]; p != nil && *p {
			tools = append(tools, id)
		}
	}
	return tools
}

func newFlagSet(f *Flags, tf *toolFlags) *flag.FlagSet {
	fs := flag.NewFlagSet("wheretoken", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.BoolVar(&f.Help, "h", f.Help, "")
	fs.BoolVar(&f.Help, "help", f.Help, "")
	fs.BoolVar(&f.Version, "version", f.Version, "")
	fs.BoolVar(&f.Version, "V", f.Version, "")
	fs.BoolVar(&f.JSON, "json", f.JSON, "")
	fs.BoolVar(&f.Today, "today", f.Today, "")
	fs.StringVar(&f.Since, "since", f.Since, "")
	fs.StringVar(&f.From, "from", f.From, "")
	fs.StringVar(&f.To, "to", f.To, "")
	fs.BoolVar(&f.ASCII, "ascii", f.ASCII, "")
	fs.BoolVar(&f.NoColor, "no-color", f.NoColor, "")
	fs.BoolVar(&f.Quiet, "quiet", f.Quiet, "")
	fs.BoolVar(&f.Quiet, "q", f.Quiet, "")
	fs.BoolVar(&f.Offline, "offline", f.Offline, "")
	fs.StringVar(&f.Home, "home", f.Home, "")
	fs.IntVar(&f.Port, "port", f.Port, "")
	fs.IntVar(&f.Width, "width", f.Width, "")
	fs.StringVar(&f.RankPeriod, "rank", f.RankPeriod, "")
	fs.BoolVar(&f.NoCommunity, "no-community", f.NoCommunity, "")
	tf.bind(fs)
	return fs
}

func parseFlagSet(fs *flag.FlagSet, f *Flags, args []string) error {
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			f.Help = true
			return nil
		}
		return usageError{msg: flagUsageMessage(err) + "\ntry `wheretoken --help`"}
	}
	return nil
}

func flagUsageMessage(err error) string {
	msg := strings.TrimSpace(err.Error())
	const undefined = "flag provided but not defined: "
	if strings.HasPrefix(msg, undefined) {
		return "unknown flag " + quote(canonicalFlag(strings.TrimPrefix(msg, undefined)))
	}
	const needsArg = "flag needs an argument: "
	if strings.HasPrefix(msg, needsArg) {
		return "flag " + quote(canonicalFlag(strings.TrimPrefix(msg, needsArg))) + " needs a value"
	}
	const invalid = "invalid value "
	if strings.HasPrefix(msg, invalid) {
		rest := strings.TrimPrefix(msg, invalid)
		val, after, ok := strings.Cut(rest, " for flag ")
		if ok {
			name, _, _ := strings.Cut(after, ":")
			return "invalid " + quote(canonicalFlag(name)) + " value " + strings.TrimSpace(val)
		}
	}
	return msg
}

func canonicalFlag(name string) string {
	name = strings.TrimSpace(name)
	if strings.HasPrefix(name, "-") && !strings.HasPrefix(name, "--") && len(name) > 2 {
		return "-" + name
	}
	return name
}

func parseCompletionTail(f *Flags, args []string) error {
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		f.CompletionShell = args[0]
		args = args[1:]
	}
	if len(args) == 0 {
		return nil
	}
	var tf toolFlags
	fs := newFlagSet(f, &tf)
	if err := parseFlagSet(fs, f, args); err != nil {
		return err
	}
	if extra := fs.Args(); len(extra) > 0 {
		return usageError{msg: fmt.Sprintf("unexpected extra argument %q", extra[0])}
	}
	return nil
}

func applyCommandWord(f *Flags, word string) bool {
	switch word {
	case "help":
		f.Help = true
	case "version":
		f.Version = true
	case "serve":
		f.Command = CommandServe
	case "scan":
		f.Command = CommandScan
		f.JSON = true
	case "sources":
		f.Command = CommandSources
	case "doctor":
		f.Command = CommandDoctor
	case "rebuild":
		f.Command = CommandRebuild
	case "update", "upgrade":
		f.Command = CommandUpdate
	case "uninstall":
		f.Command = CommandUninstall
	case "completion":
		f.Command = CommandCompletion
	case "community":
		f.Command = CommandCommunity
	case "pricing":
		f.Command = CommandPricing
	default:
		return false
	}
	return true
}

func applyTrailingCommand(f *Flags, extra []string) ([]string, error) {
	rest := extra[1:]
	if !applyCommandWord(f, extra[0]) {
		return nil, usageError{msg: fmt.Sprintf("unknown command %q\ntry `wheretoken --help`", extra[0])}
	}
	if f.Command == CommandCompletion && len(rest) > 0 && !strings.HasPrefix(rest[0], "-") {
		f.CompletionShell = rest[0]
		rest = rest[1:]
	}
	return rest, nil
}

func finishCommunity(f *Flags, extra []string) (Flags, error) {
	if f.CommunityAction == "" && len(extra) > 0 && !strings.HasPrefix(extra[0], "-") {
		switch extra[0] {
		case "status", "on", "off", "serve":
			f.CommunityAction = extra[0]
			extra = extra[1:]
		default:
			return Flags{}, usageError{msg: fmt.Sprintf("unknown community action %q\ntry `wheretoken --help`", extra[0])}
		}
	}
	if f.CommunityAction == "" {
		f.CommunityAction = "status"
	}
	if len(extra) == 0 {
		return *f, nil
	}
	var tf toolFlags
	fs := newFlagSet(f, &tf)
	if err := parseFlagSet(fs, f, extra); err != nil {
		return Flags{}, err
	}
	if leftover := fs.Args(); len(leftover) > 0 {
		return Flags{}, usageError{msg: fmt.Sprintf("unexpected extra argument %q\ntry `wheretoken --help`", leftover[0])}
	}
	if err := checkPort(f.Port); err != nil {
		return Flags{}, err
	}
	return *f, nil
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
