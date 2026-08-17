package anki

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"rota/internal/card"
	"rota/internal/db"
)

// ImportOptions defines settings for the Anki importer.
type ImportOptions struct {
	APKGPath     string
	OutputDir    string
	MediaDir     string
	WithHistory  bool
	DeckOverride string
}

// Importer handles importing Anki packages into Rota markdown files and SQLite.
type Importer struct {
	store *db.Store
}

// NewImporter creates a new Anki Importer.
func NewImporter(store *db.Store) *Importer {
	return &Importer{store: store}
}

var invalidFilenameChars = regexp.MustCompile(`[<>:"/\\|?*]`)

// Import executes the import process and returns summary results.
func (imp *Importer) Import(opts ImportOptions) (*ImportResult, error) {
	if opts.OutputDir == "" {
		opts.OutputDir = "./decks"
	}
	if opts.MediaDir == "" {
		opts.MediaDir = filepath.Join(opts.OutputDir, "media")
	}

	if err := os.MkdirAll(opts.OutputDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory %s: %w", opts.OutputDir, err)
	}

	// Calculate media relative prefix for markdown images
	mediaRelPrefix := "./media/"
	if rel, err := filepath.Rel(opts.OutputDir, opts.MediaDir); err == nil {
		mediaRelPrefix = "./" + rel + "/"
		mediaRelPrefix = filepath.ToSlash(mediaRelPrefix)
	}

	// 1. Read Anki package
	pkg, err := ReadAPKG(opts.APKGPath, opts.MediaDir)
	if err != nil {
		return nil, fmt.Errorf("failed to parse anki package: %w", err)
	}

	// 2. Convert notes to cards
	converted := pkg.ConvertToCards(mediaRelPrefix)
	if len(converted) == 0 {
		return &ImportResult{
			TotalDecks:     len(pkg.Decks),
			TotalNotes:     len(pkg.Notes),
			TotalCards:     0,
			MediaExtracted: pkg.MediaCount,
		}, nil
	}

	// 3. Group cards by deck
	cardsByDeck := make(map[string][]*ConvertedCard)
	for _, c := range converted {
		deck := c.DeckName
		if opts.DeckOverride != "" {
			deck = opts.DeckOverride
		}
		cardsByDeck[deck] = append(cardsByDeck[deck], c)
	}

	var generatedFiles []string

	// 4. Generate Markdown files per deck
	for deckName, cards := range cardsByDeck {
		// Clean file name
		safeDeckName := invalidFilenameChars.ReplaceAllString(deckName, "_")
		safeDeckName = strings.TrimSpace(safeDeckName)
		if safeDeckName == "" {
			safeDeckName = "anki_imported_deck"
		}

		filePath := filepath.Join(opts.OutputDir, safeDeckName+".md")
		var sb strings.Builder

		// Frontmatter
		sb.WriteString("---\n")
		sb.WriteString(fmt.Sprintf("deck: %q\n", deckName))
		sb.WriteString("---\n\n")
		sb.WriteString(fmt.Sprintf("# %s\n\n", deckName))

		for _, c := range cards {
			// Tags string if present
			tagStr := ""
			if len(c.Tags) > 0 {
				var prefixed []string
				for _, t := range c.Tags {
					prefixed = append(prefixed, "#"+t)
				}
				tagStr = " " + strings.Join(prefixed, " ")
			}

			if c.CardType == "cloze" {
				sb.WriteString(fmt.Sprintf("C: %s%s\n", c.Prompt, tagStr))
				if strings.TrimSpace(c.Answer) != "" {
					sb.WriteString(fmt.Sprintf("<!-- extra: %s -->\n", c.Answer))
				}
			} else {
				sb.WriteString(fmt.Sprintf("Q: %s%s\n", c.Prompt, tagStr))
				sb.WriteString(fmt.Sprintf("A: %s\n", c.Answer))
			}
			sb.WriteString("\n---\n\n")
		}

		if err := os.WriteFile(filePath, []byte(sb.String()), 0644); err != nil {
			return nil, fmt.Errorf("failed to write deck file %s: %w", filePath, err)
		}

		generatedFiles = append(generatedFiles, filePath)

		// 5. If store is provided, immediately sync cards to SQLite
		if imp.store != nil {
			parsedCards, err := card.ParseFile(filePath)
			if err == nil && len(parsedCards) > 0 {
				_, _ = imp.store.SyncFileCards(filePath, parsedCards)
			}
		}
	}

	return &ImportResult{
		TotalDecks:     len(cardsByDeck),
		TotalNotes:     len(pkg.Notes),
		TotalCards:     len(converted),
		MediaExtracted: pkg.MediaCount,
		GeneratedFiles: generatedFiles,
	}, nil
}
