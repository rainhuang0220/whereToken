package cli

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/rainhuang0220/whereToken/internal/scan"
	"github.com/rainhuang0220/whereToken/internal/table"
)

const hudTick = 120 * time.Millisecond

// hudAppearAfter skips the mascot when the command finishes almost
// immediately. It is not a sleep added to the report.
const hudAppearAfter = 140 * time.Millisecond

const reportWaitLabel = "正在整理用量…"

type scanHUD struct {
	w     io.Writer
	lines int
	start time.Time
	ascii bool
	color bool
	now   func() time.Time

	mu      sync.Mutex
	last    scan.Progress
	hasLast bool
	stop    chan struct{}
	ticking bool

	closed bool
	armed  bool
	early  chan struct{}
	wg     sync.WaitGroup
	done   sync.Once
}

func (h *scanHUD) clock() time.Time {
	if h.now != nil {
		return h.now()
	}
	return time.Now()
}

func (h *scanHUD) Show(p scan.Progress) {
	if p.Status != scan.ProgressReading {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return
	}
	if h.start.IsZero() {
		h.start = h.clock()
	}
	h.last = p
	h.hasLast = true
	h.ensureTickerLocked()
	h.paintLocked()
}

func (h *scanHUD) Clear() {
	h.finish()
}

// arm shows p after delay unless finish runs first. A fast command therefore
// does not flash a one-frame mascot. The ticker keeps the existing sprite
// moving until finish.
func (h *scanHUD) arm(delay time.Duration, p scan.Progress) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.armed || h.closed {
		return
	}
	h.armed = true
	h.early = make(chan struct{})
	early := h.early
	h.wg.Add(1)
	go func() {
		defer h.wg.Done()
		timer := time.NewTimer(delay)
		defer timer.Stop()
		select {
		case <-early:
		case <-timer.C:
			h.Show(p)
		}
	}()
}

// finish stops the ticker, erases any painted rows, and waits until the
// delayed first frame can no longer write. A second call does nothing.
func (h *scanHUD) finish() {
	h.done.Do(func() {
		h.mu.Lock()
		if h.early != nil {
			close(h.early)
		}
		h.closed = true
		h.stopTickerLocked()
		h.eraseLocked()
		h.mu.Unlock()
		h.wg.Wait()
	})
}

func (h *scanHUD) eraseLocked() {
	if h.lines <= 0 {
		return
	}
	fmt.Fprintf(h.w, "\x1b[%dA\r", h.lines)
	for i := 0; i < h.lines; i++ {
		fmt.Fprint(h.w, "\r\x1b[2K")
		if i+1 < h.lines {
			fmt.Fprint(h.w, "\n")
		}
	}
	if h.lines > 1 {
		fmt.Fprintf(h.w, "\x1b[%dA\r", h.lines-1)
	}
	fmt.Fprint(h.w, "\r")
	h.lines = 0
	h.hasLast = false
}

func (h *scanHUD) ensureTickerLocked() {
	if h.ticking {
		return
	}
	h.stop = make(chan struct{})
	h.ticking = true
	stop := h.stop
	go func() {
		t := time.NewTicker(hudTick)
		defer t.Stop()
		for {
			select {
			case <-stop:
				return
			case <-t.C:
				h.mu.Lock()
				if h.ticking && h.hasLast && h.last.Status == scan.ProgressReading {
					h.paintLocked()
				}
				h.mu.Unlock()
			}
		}
	}()
}

func (h *scanHUD) stopTickerLocked() {
	if !h.ticking {
		return
	}
	close(h.stop)
	h.ticking = false
}

func (h *scanHUD) paintLocked() {
	if !h.hasLast {
		return
	}
	elapsed := h.clock().Sub(h.start)
	cap := h.last.DisplayLabel(h.ascii)
	if h.last.Total > 0 {
		cap = fmt.Sprintf("%s  %d/%d", cap, h.last.Index, h.last.Total)
	}
	block := table.SpriteHUD(table.SpriteTick(elapsed), table.SpriteMoodTick(elapsed), cap, h.last.Index, h.last.Total, h.ascii, h.color)
	h.writeBlockLocked(block)
}

// beginWait arms the existing scan mascot for a cached report or a price
// table. Quiet, JSON, dumb terminals, and any non-interactive stdout stay
// silent. The returned stop is idempotent.
func (a *App) beginWait(flags Flags, label string) func() {
	if flags.Quiet || flags.JSON || !a.StderrTTY || !a.StdoutTTY {
		return func() {}
	}
	if a.LookupEnv != nil && a.LookupEnv("TERM") == "dumb" {
		return func() {}
	}
	ascii := flags.ASCII || table.UseASCII(false, a.GOOS, a.LookupEnv)
	h := &scanHUD{
		w:     a.Stderr,
		ascii: ascii,
		color: table.UseColor(flags.NoColor, a.StderrTTY, a.LookupEnv),
		now:   a.Now,
	}
	h.arm(hudAppearAfter, scan.Progress{Label: label, Status: scan.ProgressReading})
	return h.finish
}

func (h *scanHUD) writeBlockLocked(block string) {
	rows := strings.Split(strings.TrimSuffix(block, "\n"), "\n")
	if len(rows) == 1 && rows[0] == "" {
		rows = nil
	}
	if h.lines > 0 {
		fmt.Fprintf(h.w, "\x1b[%dA\r", h.lines)
	}
	n := len(rows)
	max := n
	if h.lines > max {
		max = h.lines
	}
	for i := 0; i < max; i++ {
		if i > 0 {
			fmt.Fprint(h.w, "\n")
		}
		fmt.Fprint(h.w, "\r\x1b[2K")
		if i < n {
			fmt.Fprint(h.w, rows[i])
		}
	}
	if max > 0 {
		fmt.Fprint(h.w, "\n")
	}
	if max > n {
		fmt.Fprintf(h.w, "\x1b[%dA\r", max-n)
	}
	h.lines = n
}
