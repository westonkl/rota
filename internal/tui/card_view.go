package tui

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
)

var (
	rendererMap  = make(map[string]*glamour.TermRenderer)
	rendererMu   sync.Mutex
	currentTheme = "auto"
)

// SetTheme sets the active TUI theme ("auto", "light", "dark").
func SetTheme(theme string) {
	rendererMu.Lock()
	defer rendererMu.Unlock()
	currentTheme = strings.ToLower(strings.TrimSpace(theme))
}

// IsLightMode determines whether light mode styling should be used.
func IsLightMode() bool {
	if currentTheme == "light" {
		return true
	}
	if currentTheme == "dark" {
		return false
	}
	if env := os.Getenv("ROTA_THEME"); env != "" {
		return strings.ToLower(env) == "light"
	}
	if colorFGBG := os.Getenv("COLORFGBG"); colorFGBG != "" {
		parts := strings.Split(colorFGBG, ";")
		if len(parts) >= 2 {
			bg := parts[len(parts)-1]
			// In COLORFGBG, bg numbers 7, 15, or high values indicate light background
			if bg == "15" || bg == "7" || bg == "11" || bg == "14" {
				return true
			}
		}
	}
	return !lipgloss.HasDarkBackground()
}

// RenderMarkdown renders markdown text using Glamour with the active theme style.
func RenderMarkdown(in string, width int) string {
	in = strings.TrimSpace(in)
	if in == "" {
		return ""
	}

	wrapWidth := max(width-6, 20)

	styleName := "dark"
	if IsLightMode() {
		styleName = "light"
	}

	cacheKey := fmt.Sprintf("%s:%d", styleName, wrapWidth)

	rendererMu.Lock()
	r, found := rendererMap[cacheKey]
	if !found {
		var err error
		r, err = glamour.NewTermRenderer(
			glamour.WithStandardStyle(styleName),
			glamour.WithWordWrap(wrapWidth),
			glamour.WithPreservedNewLines(),
		)
		if err == nil {
			rendererMap[cacheKey] = r
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
