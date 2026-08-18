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
	if h.start.IsZero() {
		h.start = h.clock()
	}
	h.last = p
	h.hasLast = true
	h.ensureTickerLocked()
	h.paintLocked()
}

func (h *scanHUD) Clear() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.stopTickerLocked()
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
