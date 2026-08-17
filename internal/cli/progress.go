package cli

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/rainhuang0220/whereToken/internal/scan"
	"github.com/rainhuang0220/whereToken/internal/table"
)

type scanHUD struct {
	w     io.Writer
	lines int
	start time.Time
	ascii bool
	color bool
	now   func() time.Time
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
	if h.start.IsZero() {
		h.start = h.clock()
	}
	cap := p.DisplayLabel(h.ascii)
	if p.Total > 0 {
		cap = fmt.Sprintf("%s  %d/%d", cap, p.Index, p.Total)
	}
	block := table.SpriteBlock(table.SpriteTick(h.clock().Sub(h.start)), cap, h.ascii, h.color)
	h.paint(block)
}

func (h *scanHUD) Clear() {
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
}

func (h *scanHUD) paint(block string) {
	if h.lines > 0 {
		fmt.Fprintf(h.w, "\x1b[%dA\r", h.lines)
	}
	fmt.Fprint(h.w, block)
	n := strings.Count(block, "\n")
	if n == 0 && block != "" {
		n = 1
	}
	h.lines = n
}
