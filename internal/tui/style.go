package tui

import (
	"github.com/charmbracelet/lipgloss"
)

var (
	// Base Colors (Adaptive for Light & Dark terminal backgrounds)
	ColorPrimary   = lipgloss.AdaptiveColor{Light: "#1d4ed8", Dark: "#7aa2f7"} // Blue
	ColorSecondary = lipgloss.AdaptiveColor{Light: "#6d28d9", Dark: "#bb9af7"} // Violet
	ColorSuccess   = lipgloss.AdaptiveColor{Light: "#15803d", Dark: "#9ece6a"} // Green (Good)
	ColorEasy      = lipgloss.AdaptiveColor{Light: "#0369a1", Dark: "#7dcfff"} // Cyan (Easy)
	ColorWarning   = lipgloss.AdaptiveColor{Light: "#b45309", Dark: "#e0af68"} // Amber (Hard)
	ColorDanger    = lipgloss.AdaptiveColor{Light: "#b91c1c", Dark: "#f7768e"} // Red (Again)
	ColorMuted     = lipgloss.AdaptiveColor{Light: "#6b7280", Dark: "#565f89"} // Grey
	ColorSubtle    = lipgloss.AdaptiveColor{Light: "#374151", Dark: "#787c99"} // Slate
	ColorBorder    = lipgloss.AdaptiveColor{Light: "#9ca3af", Dark: "#414868"} // Border
	ColorBgCard    = lipgloss.AdaptiveColor{Light: "#f3f4f6", Dark: "#1f2335"} // Surface
	ColorText      = lipgloss.AdaptiveColor{Light: "#111827", Dark: "#f3f4f6"} // Foreground

	// Styles
	StyleTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorPrimary)

	StyleDeckBadge = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#ffffff")).
			Background(lipgloss.AdaptiveColor{Light: "#1e40af", Dark: "#3d59a1"}).
			Padding(0, 1)

	StyleProgress = lipgloss.NewStyle().
			Foreground(ColorSubtle)

	StyleCountNew = lipgloss.NewStyle().
			Foreground(ColorEasy).
			Bold(true)

	StyleCountLearn = lipgloss.NewStyle().
			Foreground(ColorWarning).
			Bold(true)

	StyleCountReview = lipgloss.NewStyle().
				Foreground(ColorSuccess).
				Bold(true)

	StyleCardBox = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorBorder).
			Padding(1, 2).
			Margin(0, 0)

	StyleDivider = lipgloss.NewStyle().
			Foreground(ColorBorder)

	StyleAnswerHeader = lipgloss.NewStyle().
				Foreground(ColorSecondary).
				Bold(true)

	// Buttons for grading
	StyleBtnAgain = lipgloss.NewStyle().
			Foreground(ColorDanger).
			Bold(true).
			Padding(0, 1)

	StyleBtnHard = lipgloss.NewStyle().
			Foreground(ColorWarning).
			Bold(true).
			Padding(0, 1)

	StyleBtnGood = lipgloss.NewStyle().
			Foreground(ColorSuccess).
			Bold(true).
			Padding(0, 1)

	StyleBtnEasy = lipgloss.NewStyle().
			Foreground(ColorEasy).
			Bold(true).
			Padding(0, 1)

	StyleInterval = lipgloss.NewStyle().
			Foreground(ColorSubtle)

	StyleHelp = lipgloss.NewStyle().
			Foreground(ColorMuted)

	StyleHelpKey = lipgloss.NewStyle().
			Foreground(ColorSubtle).
			Bold(true)

	StyleSummaryCard = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(ColorPrimary).
				Padding(1, 3)

	StyleStreak = lipgloss.NewStyle().
			Foreground(ColorWarning).
			Bold(true)
)
