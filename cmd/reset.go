package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"rota/internal/tui"
)

var flagResetYes bool

var resetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Reset all FSRS scheduling progress and review logs",
	RunE: func(cmd *cobra.Command, args []string) error {
		if !flagResetYes {
			fmt.Printf("%s This will reset all card review history, intervals, and streaks back to New state.\n",
				lipgloss.NewStyle().Foreground(tui.ColorWarning).Bold(true).Render("Warning:"))
			fmt.Print("Are you sure you want to proceed? [y/N]: ")

			reader := bufio.NewReader(os.Stdin)
			input, _ := reader.ReadString('\n')
			input = strings.TrimSpace(strings.ToLower(input))

			if input != "y" && input != "yes" {
				fmt.Println("Aborted.")
				return nil
			}
		}

		if err := store.ResetProgress(); err != nil {
			return fmt.Errorf("failed to reset progress: %w", err)
		}

		fmt.Printf("\n%s All review progress and logs have been reset.\n\n",
			lipgloss.NewStyle().Foreground(tui.ColorSuccess).Bold(true).Render("✓ Success:"))
		return nil
	},
}

func init() {
	resetCmd.Flags().BoolVarP(&flagResetYes, "yes", "y", false, "Confirm reset without interactive prompt")
	rootCmd.AddCommand(resetCmd)
}
