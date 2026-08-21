package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"rota/internal/tui"
)

var statsCmd = &cobra.Command{
	Use:     "stats",
	Aliases: []string{"status", "summary"},
	Short:   "View study statistics, recall rate, streaks, and deck breakdown",
	RunE: func(cmd *cobra.Command, args []string) error {
		now := time.Now().UTC()
		stats, err := store.GetStatsSummary(now)
		if err != nil {
			return fmt.Errorf("failed to load statistics: %w", err)
		}

		view := tui.RenderStatsView(stats)
		fmt.Print(view)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(statsCmd)
}
