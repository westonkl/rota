package tui

import (
	"github.com/charmbracelet/lipgloss"
)

var (
	// Base Colors
	ColorPrimary   = lipgloss.Color("#7aa2f7") // Soft Cornflower Blue
	ColorSecondary = lipgloss.Color("#bb9af7") // Soft Violet
	ColorSuccess   = lipgloss.Color("#9ece6a") // Leaf Green (Good)
	ColorEasy      = lipgloss.Color("#7dcfff") // Sky Cyan (Easy)
	ColorWarning   = lipgloss.Color("#e0af68") // Warm Amber (Hard)
	ColorDanger    = lipgloss.Color("#f7768e") // Coral Red (Again)
	ColorMuted     = lipgloss.Color("#565f89") // Subdued Grey-Blue
	ColorSubtle    = lipgloss.Color("#787c99") // Light Slate
	ColorBorder    = lipgloss.Color("#414868") // Subtle Card Border
	ColorBgCard    = lipgloss.Color("#1f2335") // Card Surface Background

	// Styles
	StyleTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorPrimary)

	StyleDeckBadge = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#ffffff")).
			Background(lipgloss.Color("#3d59a1")).
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
			Foreground(ColorSubtle).
			Faint(true)

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
