package index

import "sync"

type Delta struct {
	Source string
	Mode   string
	Added  int
}

var (
	statMu sync.Mutex
	deltas []Delta
)

func ResetDeltas() {
	statMu.Lock()
	deltas = nil
	statMu.Unlock()
}

func Deltas() []Delta {
	statMu.Lock()
	out := append([]Delta(nil), deltas...)
	statMu.Unlock()
	return out
}

func note(source, mode string, added int) {
	if source == "" {
		return
	}
	statMu.Lock()
	defer statMu.Unlock()
	for i := range deltas {
		if deltas[i].Source == source {
			if mode == ModeFull || (mode == ModeIncremental && deltas[i].Mode != ModeFull) {
				deltas[i].Mode = mode
			}
			if mode == ModeUnchanged && deltas[i].Mode == "" {
				deltas[i].Mode = ModeUnchanged
			}
			if mode != ModeUnchanged {
				if deltas[i].Mode == ModeUnchanged {
					deltas[i].Mode = mode
				}
				deltas[i].Added += added
			} else if deltas[i].Mode == "" {
				deltas[i].Mode = ModeUnchanged
			}
			return
		}
	}
	deltas = append(deltas, Delta{Source: source, Mode: mode, Added: added})
}
