package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"rota/internal/importer/anki"
	"rota/internal/tui"
)

var (
	flagImportOutDir  string
	flagImportDeck    string
	flagImportHistory bool
)

var importCmd = &cobra.Command{
	Use:   "import [format] <file>",
	Short: "Import flashcards from external spaced repetition apps (default: Anki .apkg)",
	Long: `Import flashcards from other formats into clean Markdown notes and index them into rota.

Default format: anki (.apkg)

Examples:
  rota import Computer_Science.apkg
  rota import anki Biology.apkg --out ./notes/decks/
  rota import deck.apkg --deck "Algorithms"
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return cmd.Help()
		}

		var format, filePath string
		if len(args) == 1 {
			// Default format is anki
			format = "anki"
			filePath = args[0]
		} else {
			format = strings.ToLower(args[0])
			filePath = args[1]
		}

		switch format {
		case "anki":
			return runAnkiImport(filePath)
		default:
			// If format wasn't explicitly recognized but args[0] is a valid file, fallback to anki
			if _, err := os.Stat(args[0]); err == nil {
				return runAnkiImport(args[0])
			}
			return fmt.Errorf("unknown import format %q. Supported formats: anki", format)
		}
	},
}

func runAnkiImport(apkgPath string) error {
	if _, err := os.Stat(apkgPath); err != nil {
		return fmt.Errorf("anki file not found: %s", apkgPath)
	}

	fmt.Println(tui.StyleTitle.Render("✦ Importing Anki package..."))

	imp := anki.NewImporter(store)
	result, err := imp.Import(anki.ImportOptions{
		APKGPath:     apkgPath,
		OutputDir:    flagImportOutDir,
		DeckOverride: flagImportDeck,
		WithHistory:  flagImportHistory,
	})
	if err != nil {
		return fmt.Errorf("import failed: %w", err)
	}

	// Print Summary
	fmt.Println()
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(tui.ColorPrimary)
	labelStyle := lipgloss.NewStyle().Foreground(tui.ColorSubtle)
	valStyle := lipgloss.NewStyle().Bold(true).Foreground(tui.ColorSuccess)

	fmt.Println(titleStyle.Render("Import Summary:"))
	fmt.Printf("  %s %s\n", labelStyle.Render("Decks created   :"), valStyle.Render(fmt.Sprintf("%d", result.TotalDecks)))
	fmt.Printf("  %s %s\n", labelStyle.Render("Notes processed :"), valStyle.Render(fmt.Sprintf("%d", result.TotalNotes)))
	fmt.Printf("  %s %s\n", labelStyle.Render("Cards imported  :"), valStyle.Render(fmt.Sprintf("%d", result.TotalCards)))

	if len(result.GeneratedFiles) > 0 {
		fmt.Println("\n" + labelStyle.Render("Generated Markdown Notes:"))
		for _, f := range result.GeneratedFiles {
			rel, err := filepath.Rel(".", f)
			if err != nil {
				rel = f
			}
			fmt.Printf("  • %s\n", lipgloss.NewStyle().Foreground(tui.ColorEasy).Render(rel))
		}
	}

	fmt.Println("\n" + lipgloss.NewStyle().Foreground(tui.ColorSuccess).Bold(true).Render("✔ All cards synchronized and ready to drill!"))
	return nil
}

func init() {
	importCmd.Flags().StringVarP(&flagImportOutDir, "out", "o", "./decks", "Directory to write generated Markdown deck notes")
	importCmd.Flags().StringVar(&flagImportDeck, "deck", "", "Override deck name for all imported cards")
	importCmd.Flags().BoolVar(&flagImportHistory, "with-history", false, "Import card review intervals & history")

	rootCmd.AddCommand(importCmd)
}
