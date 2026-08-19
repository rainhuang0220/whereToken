package index

import (
	"errors"
	"testing"

	"github.com/rainhuang0220/whereToken/internal/event"
)

func TestForwardEmitsCachedEventsOnParseError(t *testing.T) {
	want := errors.New("jsonl line exceeds")
	var evs []event.UsageEvent
	err := Forward(
		[]event.UsageEvent{{Source: "claude", RequestID: "keep", Miss: 10}},
		[]event.TurnEvent{{Source: "claude"}},
		want,
		func(e event.UsageEvent) { evs = append(evs, e) },
		func(event.TurnEvent) {},
	)
	if !errors.Is(err, want) {
		t.Fatalf("err=%v", err)
	}
	if len(evs) != 1 || evs[0].Miss != 10 {
		t.Fatalf("cached events must still emit: %+v", evs)
	}
}
