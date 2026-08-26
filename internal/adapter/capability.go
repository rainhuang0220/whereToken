package adapter

// Level is a declared source capability. "yes" means the collector can
// produce that signal. "unavailable" means the tool is known but that
// signal cannot be read safely. "unknown" means the format cannot say.
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
