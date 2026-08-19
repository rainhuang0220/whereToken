package adapter

import "testing"

func TestInferProbeConfigDirIsNotUsage(t *testing.T) {
	c := Caps{Discovery: LevelYes, Usage: LevelYes}
	p := InferProbe(true, false, c)
	if !p.Detected || p.Ledger {
		t.Fatalf("detected without ledger: %+v", p)
	}
	if p.Caps.Usage != LevelUnavailable {
		t.Fatalf("config-only must not claim usage: %+v", p.Caps)
	}
}

func TestInferProbeMissingTool(t *testing.T) {
	p := InferProbe(false, false, Caps{Discovery: LevelYes})
	if p.Detected || p.Ledger {
		t.Fatalf("%+v", p)
	}
}
