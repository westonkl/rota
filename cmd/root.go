package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/westonkl/rota/internal/config"
	"github.com/westonkl/rota/internal/db"
	"github.com/westonkl/rota/internal/fsrs"
	"github.com/westonkl/rota/internal/tui"
)

var (
	flagDBPath    string
	flagVaultPath string
	flagRetention float64
	flagTheme     string

	store     *db.Store
	scheduler *fsrs.Scheduler
)

var rootCmd = &cobra.Command{
	Use:   "rota",
	Short: "rota - Fast, minimalist terminal spaced repetition for markdown notes",
	Long: `rota is a lightweight terminal flashcard application built with Go.
It parses markdown notes for Q/A and cloze flashcards, schedules reviews with the Free Spaced Repetition Scheduler (FSRS), and tracks your learning progress in SQLite.

Usage:
  rota drill [path]      Start an interactive review session
  rota scan  [path]      Scan and index markdown cards into SQLite
  rota stats             View learning streak, retention, and progress
  rota list              List all indexed cards and due dates
  rota add               Add a new card to a markdown file
  rota check [path]      Lint markdown files for invalid card formats
  rota import <file>     Import flashcards (default: Anki .apkg) into Markdown & SQLite
  rota reset             Reset learning progress
`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Set TUI theme
		tui.SetTheme(flagTheme)

		// Don't initialize DB for help or version commands
		if cmd.Name() == "help" || cmd.Name() == "version" {
			return nil
		}

		dbPath := config.ResolveDBPath(flagDBPath)
		var err error
		store, err = db.Open(dbPath)
		if err != nil {
			return fmt.Errorf("failed to open database at %s: %w", dbPath, err)
		}

		fsrsCfg := fsrs.DefaultConfig()
		if flagRetention > 0 {
			fsrsCfg.RequestRetention = flagRetention
		}
		scheduler = fsrs.NewScheduler(fsrsCfg)

		return nil
	},
	PersistentPostRun: func(cmd *cobra.Command, args []string) {
		if store != nil {
			store.Close()
		}
	},
	Run: func(cmd *cobra.Command, args []string) {
		// Default to running drill
		drillCmd.Run(cmd, args)
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&flagDBPath, "db", "d", "", "Path to SQLite database file (default ~/.local/share/rota/rota.db)")
	rootCmd.PersistentFlags().StringVarP(&flagVaultPath, "path", "p", ".", "Path to directory containing markdown notes")
	rootCmd.PersistentFlags().Float64VarP(&flagRetention, "retention", "r", 0.9, "Target retention rate for FSRS (default 0.9 or 90%)")
	rootCmd.PersistentFlags().StringVar(&flagTheme, "theme", "auto", "TUI theme: auto, light, or dark (or set ROTA_THEME)")
}
