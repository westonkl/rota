package cmd

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"rota/internal/card"
	"rota/internal/tui"
)

var scanCmd = &cobra.Command{
	Use:     "scan [path...]",
	Aliases: []string{"sync", "index"},
	Short:   "Scan and index flashcards from markdown files into SQLite",
	RunE: func(cmd *cobra.Command, args []string) error {
		targets := args
		if len(targets) == 0 {
			targets = []string{flagVaultPath}
		}

		fmt.Println(tui.StyleTitle.Render("✦ Scanning markdown files for flashcards..."))

		totalFiles, totalCards, totalAdded, totalUpdated, totalUnchanged, totalDeleted, err := RunScan(targets)
		if err != nil {
			return err
		}

		fmt.Printf("\n%s\n", tui.StyleAnswerHeader.Render("Scan Summary:"))
		fmt.Printf("  Files scanned : %d\n", totalFiles)
		fmt.Printf("  Total cards   : %d\n", totalCards)
		fmt.Printf("  Added         : %s\n", lipgloss.NewStyle().Foreground(tui.ColorSuccess).Render(fmt.Sprintf("%d", totalAdded)))
		fmt.Printf("  Updated       : %s\n", lipgloss.NewStyle().Foreground(tui.ColorWarning).Render(fmt.Sprintf("%d", totalUpdated)))
		fmt.Printf("  Unchanged     : %d\n", totalUnchanged)
		if totalDeleted > 0 {
			fmt.Printf("  Removed       : %s\n", lipgloss.NewStyle().Foreground(tui.ColorDanger).Render(fmt.Sprintf("%d", totalDeleted)))
		}
		fmt.Println()

		return nil
	},
}

// RunScan discovers and parses all markdown files in target paths and updates SQLite.
func RunScan(targets []string) (totalFiles, totalCards, totalAdded, totalUpdated, totalUnchanged, totalDeleted int, err error) {
	files := make(map[string]bool)

	for _, target := range targets {
		info, statErr := os.Stat(target)
		if statErr != nil {
			return 0, 0, 0, 0, 0, 0, fmt.Errorf("invalid path %s: %w", target, statErr)
		}

		if !info.IsDir() {
			if strings.HasSuffix(strings.ToLower(target), ".md") {
				abs, _ := filepath.Abs(target)
				files[abs] = true
			}
			continue
		}

		walkErr := filepath.WalkDir(target, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			// Skip hidden directories like .git, .rota, etc.
			if d.IsDir() && strings.HasPrefix(d.Name(), ".") && d.Name() != "." {
				return filepath.SkipDir
			}
			if !d.IsDir() && strings.HasSuffix(strings.ToLower(d.Name()), ".md") {
				abs, _ := filepath.Abs(path)
				files[abs] = true
			}
			return nil
		})
		if walkErr != nil {
			return 0, 0, 0, 0, 0, 0, fmt.Errorf("error walking path %s: %w", target, walkErr)
		}
	}

	for f := range files {
		parsed, parseErr := card.ParseFile(f)
		if parseErr != nil {
			fmt.Fprintf(os.Stderr, "  Warning: error parsing %s: %v\n", f, parseErr)
			continue
		}

		syncRes, syncErr := store.SyncFileCards(f, parsed)
		if syncErr != nil {
			return 0, 0, 0, 0, 0, 0, fmt.Errorf("failed syncing cards for %s: %w", f, syncErr)
		}

		totalFiles++
		totalCards += len(parsed)
		totalAdded += syncRes.Added
		totalUpdated += syncRes.Updated
		totalUnchanged += syncRes.Unchanged
		totalDeleted += syncRes.Deleted
	}

	return totalFiles, totalCards, totalAdded, totalUpdated, totalUnchanged, totalDeleted, nil
}

func init() {
	rootCmd.AddCommand(scanCmd)
}
