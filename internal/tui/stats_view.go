package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	gofsrs "github.com/open-spaced-repetition/go-fsrs/v4"
	"rota/internal/db"
)

// RenderStatsView formats the stats summary into a clean terminal report.
func RenderStatsView(stats *db.StatsSummary) string {
	var b strings.Builder

	// Header
	header := StyleTitle.Render("✦ ROTA SPACED REPETITION STATISTICS ✦")
	b.WriteString("\n" + header + "\n\n")

	// Metric Cards
	cardWidth := 18
	mStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorBorder).
		Padding(0, 1).
		Width(cardWidth)

	boxStreak := mStyle.Render(fmt.Sprintf("%s\n%s", StyleStreak.Render(fmt.Sprintf("%d Days 🔥", stats.CurrentStreak)), lipgloss.NewStyle().Foreground(ColorSubtle).Render("Review Streak")))
	boxDue := mStyle.Render(fmt.Sprintf("%s\n%s", StyleCountLearn.Render(fmt.Sprintf("%d Cards", stats.DueToday)), lipgloss.NewStyle().Foreground(ColorSubtle).Render("Due Today")))
	boxRetention := mStyle.Render(fmt.Sprintf("%s\n%s", StyleCountReview.Render(fmt.Sprintf("%.1f%%", stats.RetentionRate)), lipgloss.NewStyle().Foreground(ColorSubtle).Render("Recall Rate")))
	boxTotal := mStyle.Render(fmt.Sprintf("%s\n%s", StyleCountNew.Render(fmt.Sprintf("%d Cards", stats.TotalCards)), lipgloss.NewStyle().Foreground(ColorSubtle).Render("Total Cards")))

	metricsRow := lipgloss.JoinHorizontal(
		lipgloss.Top,
		boxStreak, " ",
		boxDue, " ",
		boxRetention, " ",
		boxTotal,
	)
	b.WriteString(metricsRow + "\n\n")

	// 14-Day Activity Bar
	b.WriteString(StyleAnswerHeader.Render("Recent Activity (Last 14 Days):") + "\n")
	now := time.Now().UTC()
	var barBuilder strings.Builder
	var labelBuilder strings.Builder

	maxCount := 1
	for i := 13; i >= 0; i-- {
		day := now.AddDate(0, 0, -i).Format("2006-01-02")
		c := stats.ReviewsByDay[day]
		if c > maxCount {
			maxCount = c
		}
	}

	for i := 13; i >= 0; i-- {
		dayDate := now.AddDate(0, 0, -i)
		day := dayDate.Format("2006-01-02")
		c := stats.ReviewsByDay[day]

		var symbol string
		var style lipgloss.Style
		if c == 0 {
			symbol = "· "
			style = lipgloss.NewStyle().Foreground(ColorMuted)
		} else if c < 5 {
			symbol = "▃ "
			style = lipgloss.NewStyle().Foreground(ColorPrimary)
		} else if c < 15 {
			symbol = "▅ "
			style = lipgloss.NewStyle().Foreground(ColorSecondary)
		} else {
			symbol = "█ "
			style = lipgloss.NewStyle().Foreground(ColorSuccess)
		}

		barBuilder.WriteString(style.Render(symbol))
		labelBuilder.WriteString(lipgloss.NewStyle().Foreground(ColorMuted).Render(dayDate.Format("02")[:1] + " "))
	}
	b.WriteString("  " + barBuilder.String() + "\n")
	b.WriteString("  " + labelBuilder.String() + "\n\n")

	// Ratings Distribution
	totalLogged := stats.TotalReviews
	if totalLogged > 0 {
		b.WriteString(StyleAnswerHeader.Render("Review Rating Distribution:") + "\n")
		againP := float64(stats.RatingsCount[gofsrs.Again]) / float64(totalLogged) * 100.0
		hardP := float64(stats.RatingsCount[gofsrs.Hard]) / float64(totalLogged) * 100.0
		goodP := float64(stats.RatingsCount[gofsrs.Good]) / float64(totalLogged) * 100.0
		easyP := float64(stats.RatingsCount[gofsrs.Easy]) / float64(totalLogged) * 100.0

		b.WriteString(fmt.Sprintf("  %s %-6s : %4d (%4.1f%%)  %s\n", StyleBtnAgain.Render("Again"), "", stats.RatingsCount[gofsrs.Again], againP, renderBar(againP, ColorDanger)))
		b.WriteString(fmt.Sprintf("  %s  %-6s : %4d (%4.1f%%)  %s\n", StyleBtnHard.Render("Hard"), "", stats.RatingsCount[gofsrs.Hard], hardP, renderBar(hardP, ColorWarning)))
		b.WriteString(fmt.Sprintf("  %s  %-6s : %4d (%4.1f%%)  %s\n", StyleBtnGood.Render("Good"), "", stats.RatingsCount[gofsrs.Good], goodP, renderBar(goodP, ColorSuccess)))
		b.WriteString(fmt.Sprintf("  %s  %-6s : %4d (%4.1f%%)  %s\n\n", StyleBtnEasy.Render("Easy"), "", stats.RatingsCount[gofsrs.Easy], easyP, renderBar(easyP, ColorEasy)))
	}

	// Decks Breakdown Table
	if len(stats.DeckBreakdown) > 0 {
		b.WriteString(StyleAnswerHeader.Render("Deck Breakdown:") + "\n")
		headerLine := fmt.Sprintf("  %-24s %-8s %-8s %-8s %-8s", "DECK", "TOTAL", "NEW", "LEARN", "DUE")
		b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(ColorSubtle).Render(headerLine) + "\n")
		b.WriteString("  " + StyleDivider.Render(strings.Repeat("─", 58)) + "\n")

		for _, d := range stats.DeckBreakdown {
			deckName := d.Name
			if len(deckName) > 22 {
				deckName = deckName[:19] + "..."
			}

			dueStyle := lipgloss.NewStyle().Foreground(ColorSubtle)
			if d.DueCards > 0 {
				dueStyle = lipgloss.NewStyle().Foreground(ColorWarning).Bold(true)
			}

			row := fmt.Sprintf(
				"  %-24s %-8d %-8d %-8d %-8s",
				deckName,
				d.TotalCards,
				d.NewCards,
				d.LearningCards,
				dueStyle.Render(fmt.Sprintf("%d", d.DueCards)),
			)
			b.WriteString(row + "\n")
		}
		b.WriteString("\n")
	}

	return b.String()
}

func renderBar(percent float64, color lipgloss.TerminalColor) string {
	width := 20
	filled := int((percent / 100.0) * float64(width))
	if filled > width {
		filled = width
	}
	empty := width - filled
	if empty < 0 {
		empty = 0
	}

	bar := strings.Repeat("█", filled) + strings.Repeat("░", empty)
	return lipgloss.NewStyle().Foreground(color).Render(bar)
}
