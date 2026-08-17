package tui

import (
	"strings"
	"sync"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
)

var (
	rendererMap = make(map[int]*glamour.TermRenderer)
	rendererMu  sync.Mutex
)

// RenderMarkdown renders markdown text using Glamour with fixed standard style.
// Standard style avoids querying stdin/OSC escape sequences which can deadlock Bubble Tea.
func RenderMarkdown(in string, width int) string {
	in = strings.TrimSpace(in)
	if in == "" {
		return ""
	}

	wrapWidth := width - 6
	if wrapWidth < 20 {
		wrapWidth = 20
	}

	rendererMu.Lock()
	r, found := rendererMap[wrapWidth]
	if !found {
		var err error
		r, err = glamour.NewTermRenderer(
			glamour.WithStandardStyle("dark"),
			glamour.WithWordWrap(wrapWidth),
			glamour.WithPreservedNewLines(),
		)
		if err == nil {
			rendererMap[wrapWidth] = r
		}
	}
	rendererMu.Unlock()

	if r == nil {
		return in
	}

	out, err := r.Render(in)
	if err != nil {
		return in
	}

	return strings.TrimSpace(out)
}

// RenderClozeBack highlights masked cloze deletions in the answer.
func RenderClozeBack(answer string, width int) string {
	return RenderMarkdown(answer, width)
}

// ProgressBar renders a clean block progress bar: [██████░░░░]
func ProgressBar(current, total, width int) string {
	if total <= 0 {
		return ""
	}
	if width <= 0 {
		width = 15
	}
	if current > total {
		current = total
	}

	ratio := float64(current) / float64(total)
	filled := int(ratio * float64(width))
	empty := width - filled

	bar := strings.Repeat("█", filled) + strings.Repeat("░", empty)
	return lipgloss.NewStyle().Foreground(ColorPrimary).Render(bar)
}
