package cmd

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/westonkl/rota/internal/card"
	"github.com/westonkl/rota/internal/tui"
)

var flagScanForce bool

var scanCmd = &cobra.Command{
	Use:     "scan [path...]",
	Aliases: []string{"sync", "index"},
	Short:   "Scan and index flashcards from markdown files into SQLite (incremental)",
	RunE: func(cmd *cobra.Command, args []string) error {
		targets := args
		if len(targets) == 0 {
			targets = []string{flagVaultPath}
		}

		fmt.Println(tui.StyleTitle.Render("✦ Scanning markdown files for flashcards..."))

		totalFiles, totalCards, totalAdded, totalUpdated, totalUnchanged, totalDeleted, err := RunScan(targets, flagScanForce)
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

// RunScan discovers and parses markdown files in target paths, updating SQLite incrementally.
func RunScan(targets []string, force bool) (totalFiles, totalCards, totalAdded, totalUpdated, totalUnchanged, totalDeleted int, err error) {
	files := make(map[string]os.FileInfo)

	for _, target := range targets {
		info, statErr := os.Stat(target)
		if statErr != nil {
			return 0, 0, 0, 0, 0, 0, fmt.Errorf("invalid path %s: %w", target, statErr)
		}

		if !info.IsDir() {
			if strings.HasSuffix(strings.ToLower(target), ".md") {
				abs, _ := filepath.Abs(target)
				if realPath, err := filepath.EvalSymlinks(abs); err == nil {
					abs = realPath
				}
				files[abs] = info
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
				if realPath, err := filepath.EvalSymlinks(abs); err == nil {
					abs = realPath
				}
				if fileInfo, err := d.Info(); err == nil {
					files[abs] = fileInfo
				}
			}
			return nil
		})
		if walkErr != nil {
			return 0, 0, 0, 0, 0, 0, fmt.Errorf("error walking path %s: %w", target, walkErr)
		}
	}

	syncedFiles, _ := store.GetSyncedFiles()

	for f, info := range files {
		// Incremental check: if file is already tracked and mtime hasn't changed, skip parsing
		if !force && syncedFiles != nil {
			synced, exists := syncedFiles[f]
			if !exists {
				synced, exists = syncedFiles[strings.ToLower(f)]
			}
			if exists && synced.CardCount > 0 {
				// ModTime comparison (truncate to seconds for SQLite timestamp precision)
				fileModTime := info.ModTime().UTC().Truncate(1 * 1000000000) // 1s
				syncedModTime := synced.LastModified.UTC().Truncate(1 * 1000000000)
				if !fileModTime.After(syncedModTime) {
					totalFiles++
					totalCards += synced.CardCount
					totalUnchanged += synced.CardCount
					continue
				}
			}
		}

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

	if orphanDeleted, err := store.CleanOrphanFiles(); err == nil && orphanDeleted > 0 {
		totalDeleted += orphanDeleted
	}

	return totalFiles, totalCards, totalAdded, totalUpdated, totalUnchanged, totalDeleted, nil
}

func init() {
	scanCmd.Flags().BoolVarP(&flagScanForce, "force", "f", false, "Force re-scanning and re-parsing all files regardless of modification time")
	rootCmd.AddCommand(scanCmd)
}
