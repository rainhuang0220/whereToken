package adapter

// Level is a declared source capability. "yes" means the collector can
// produce that signal. "unavailable" means the tool is known but that
// signal cannot be read safely. "unknown" means we have not probed yet.
type Level string

const (
	LevelYes         Level = "yes"
	LevelUnavailable Level = "unavailable"
	LevelUnknown     Level = "unknown"
)

// Caps is what a tool can expose. Discovery is not usage.
type Caps struct {
	Discovery   Level
	Usage       Level
	Cost        Level
	Model       Level
	Timestamp   Level
	Session     Level
	Cache       Level
	Reasoning   Level
	Archive     Level
	Incremental Level
}

// Probe is a cheap post-discover inspection. Finding a config dir is
// Detected; finding a ledger file is Ledger. Usage stays unavailable
// until Parse emits events (or Probe can prove a ledger exists).
type Probe struct {
	Detected bool
	Ledger   bool
	Caps     Caps
	Note     string
}

// Prober is optional. Adapters that do not implement it are probed
// from Discover + Parse results only.
type Prober interface {
	Probe(root SourceRoot) Probe
}

func InferProbe(detected, ledger bool, c Caps) Probe {
	if !detected {
		return Probe{Caps: c}
	}
	p := Probe{Detected: true, Ledger: ledger, Caps: c}
	if !ledger && c.Usage == LevelYes {
		p.Caps.Usage = LevelUnavailable
	}
	return p
}
