package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	gofsrs "github.com/open-spaced-repetition/go-fsrs/v4"
	"github.com/spf13/cobra"

	"github.com/westonkl/rota/internal/db"
	"github.com/westonkl/rota/internal/tui"
)

var (
	flagListDeck   string
	flagListDue    bool
	flagListSearch string
	flagListLimit  int
)

var listCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"cards", "ls"},
	Short:   "List indexed flashcards and scheduling details",
	RunE: func(cmd *cobra.Command, args []string) error {
		now := time.Now().UTC()
		filter := db.CardFilter{
			Deck:        flagListDeck,
			DueOnly:     flagListDue,
			SearchQuery: flagListSearch,
			Limit:       flagListLimit,
			Now:         now,
		}

		cards, err := store.ListCards(filter)
		if err != nil {
			return fmt.Errorf("failed to list cards: %w", err)
		}

		if len(cards) == 0 {
			fmt.Println("No flashcards found matching criteria.")
			return nil
		}

		fmt.Printf("\n%s\n", tui.StyleTitle.Render(fmt.Sprintf("✦ Flashcards (%d total) ✦", len(cards))))

		header := fmt.Sprintf("  %-16s %-6s %-32s %-8s %-12s", "DECK", "TYPE", "PROMPT", "STATE", "DUE IN")
		fmt.Println(lipgloss.NewStyle().Bold(true).Foreground(tui.ColorSubtle).Render(header))
		fmt.Println("  " + tui.StyleDivider.Render(strings.Repeat("─", 78)))

		for _, c := range cards {
			deckStr := c.Deck
			if len(deckStr) > 15 {
				deckStr = deckStr[:12] + "..."
			}

			promptStr := c.Prompt
			promptStr = strings.ReplaceAll(promptStr, "\n", " ")
			if len(promptStr) > 30 {
				promptStr = promptStr[:27] + "..."
			}

			stateStr := "New"
			stateStyle := tui.StyleCountNew
			dueStr := "Ready"

			if c.FSRS != nil {
				switch c.FSRS.State {
				case gofsrs.Learning, gofsrs.Relearning:
					stateStr = "Learn"
					stateStyle = tui.StyleCountLearn
				case gofsrs.Review:
					stateStr = "Review"
					stateStyle = tui.StyleCountReview
				}

				if !c.FSRS.Due.IsZero() {
					diff := c.FSRS.Due.Sub(now)
					if diff <= 0 {
						dueStr = "Now"
					} else {
						dueStr = formatListDuration(diff)
					}
				}
			}

			fmt.Printf("  %-16s %-6s %-32s %-8s %-12s\n",
				deckStr,
				string(c.Type),
				promptStr,
				stateStyle.Render(stateStr),
				dueStr,
			)
		}
		fmt.Println()
		return nil
	},
}

func formatListDuration(d time.Duration) string {
	hours := d.Hours()
	if hours < 24 {
		return fmt.Sprintf("in %dh", int(hours))
	}
	days := int(hours / 24)
	return fmt.Sprintf("in %dd", days)
}

func init() {
	listCmd.Flags().StringVarP(&flagListDeck, "deck", "k", "", "Filter by deck")
	listCmd.Flags().BoolVar(&flagListDue, "due", false, "Only list cards currently due")
	listCmd.Flags().StringVarP(&flagListSearch, "search", "s", "", "Search card text")
	listCmd.Flags().IntVarP(&flagListLimit, "limit", "n", 50, "Limit number of cards displayed")

	rootCmd.AddCommand(listCmd)
}
