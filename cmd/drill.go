package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	gofsrs "github.com/open-spaced-repetition/go-fsrs/v4"
	"github.com/spf13/cobra"

	"rota/internal/card"
	"rota/internal/db"
	"rota/internal/tui"
)

var (
	flagDeck       string
	flagLimit      int
	flagAllCards   bool
	flagNewOnly    bool
	flagPlainMode  bool
	flagNoAutoSync bool
)

var drillCmd = &cobra.Command{
	Use:     "drill [path...]",
	Aliases: []string{"review", "study"},
	Short:   "Start an interactive flashcard review session",
	RunE: func(cmd *cobra.Command, args []string) error {
		targets := args
		if len(targets) == 0 {
			targets = []string{flagVaultPath}
		}

		// 1. Auto-sync if not disabled
		if !flagNoAutoSync {
			_, _, _, _, _, _, _ = RunScan(targets)
		}

		// 2. Fetch cards
		now := time.Now().UTC()
		var cards []*card.Card
		var err error

		if flagAllCards {
			filter := db.CardFilter{
				Deck:  flagDeck,
				Limit: flagLimit,
			}
			cards, err = store.ListCards(filter)
		} else if flagNewOnly {
			newState := gofsrs.New
			filter := db.CardFilter{
				Deck:  flagDeck,
				State: &newState,
				Limit: flagLimit,
			}
			cards, err = store.ListCards(filter)
		} else {
			cards, err = store.GetDueCards(flagDeck, flagLimit, now)
		}

		if err != nil {
			return fmt.Errorf("failed to fetch review cards: %w", err)
		}

		if flagPlainMode {
			return runPlainSession(cards)
		}

		// 3. Launch Bubble Tea TUI
		app := tui.NewAppModel(store, scheduler, cards)
		p := tea.NewProgram(app, tea.WithAltScreen())
		if _, err := p.Run(); err != nil {
			return fmt.Errorf("error running review session: %w", err)
		}

		return nil
	},
}

func runPlainSession(cards []*card.Card) error {
	if len(cards) == 0 {
		fmt.Println("No cards currently due for review.")
		return nil
	}

	reader := bufio.NewReader(os.Stdin)
	now := time.Now().UTC()

	fmt.Printf("Starting plain review session (%d cards)\n\n", len(cards))

	for i, c := range cards {
		fmt.Printf("--- Card %d/%d [%s] ---\n", i+1, len(cards), c.Deck)
		fmt.Printf("Q: %s\n\n", c.Prompt)
		fmt.Print("Press [Enter] to reveal answer...")
		_, _ = reader.ReadString('\n')

		fmt.Printf("\nA: %s\n\n", c.Answer)

		var fsrsCard gofsrs.Card
		if c.FSRS != nil {
			fsrsCard = c.FSRS.ToFSRSCard()
		} else {
			fsrsCard = gofsrs.NewCard()
		}
		intervals := scheduler.PreviewIntervals(fsrsCard, now)

		fmt.Printf("Grade: [1] Again (%s)  [2] Hard (%s)  [3] Good (%s)  [4] Easy (%s)  [q] Quit: ",
			intervals[gofsrs.Again], intervals[gofsrs.Hard], intervals[gofsrs.Good], intervals[gofsrs.Easy])

		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(strings.ToLower(input))

		if input == "q" {
			break
		}

		var rating gofsrs.Rating
		switch input {
		case "1", "a", "again":
			rating = gofsrs.Again
		case "2", "h", "hard":
			rating = gofsrs.Hard
		case "4", "e", "easy":
			rating = gofsrs.Easy
		default:
			rating = gofsrs.Good
		}

		nextCard, _ := scheduler.Next(fsrsCard, now, rating)
		nextState := card.FromFSRSCard(nextCard)
		if err := store.RecordReview(c.ID, rating, nextState, now); err != nil {
			fmt.Printf("Error saving review: %v\n", err)
		}
		fmt.Println()
	}

	fmt.Println("Session ended.")
	return nil
}

func init() {
	drillCmd.Flags().StringVarP(&flagDeck, "deck", "k", "", "Filter reviews by deck name")
	drillCmd.Flags().IntVarP(&flagLimit, "limit", "n", 0, "Maximum number of cards to review in session")
	drillCmd.Flags().BoolVarP(&flagAllCards, "all", "a", false, "Review all cards (cram mode, ignores due dates)")
	drillCmd.Flags().BoolVar(&flagNewOnly, "new", false, "Review only new cards")
	drillCmd.Flags().BoolVar(&flagPlainMode, "plain", false, "Run in plain text mode (non-TUI)")
	drillCmd.Flags().BoolVar(&flagNoAutoSync, "no-sync", false, "Skip automatic markdown scan before drilling")

	rootCmd.AddCommand(drillCmd)
}
