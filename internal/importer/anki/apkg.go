package anki

import (
	"archive/zip"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// AnkiPackage holds all extracted notes, cards, decks, and media from an .apkg file.
type AnkiPackage struct {
	Decks       map[int64]*AnkiDeck
	Models      map[int64]*AnkiModel
	Notes       map[int64]*AnkiNote
	Cards       []*AnkiCard
	MediaMap    map[string]string
	MediaCount  int
}

// ReadAPKG parses an .apkg file from disk, extracting media to targetMediaDir.
func ReadAPKG(apkgPath string, targetMediaDir string) (*AnkiPackage, error) {
	r, err := zip.OpenReader(apkgPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open .apkg archive: %w", err)
	}
	defer r.Close()

	// 1. Extract Media if media directory specified
	var mediaMap map[string]string
	mediaCount := 0
	if targetMediaDir != "" {
		mediaMap, mediaCount, err = ExtractMedia(r.File, targetMediaDir)
		if err != nil {
			// Log/continue with warning
			fmt.Printf("Warning: failed to extract some media: %v\n", err)
		}
	}

	// 2. Locate SQLite database (collection.anki21 or collection.anki2)
	var dbFile *zip.File
	for _, f := range r.File {
		if f.Name == "collection.anki21" {
			dbFile = f
			break
		}
		if f.Name == "collection.anki2" && dbFile == nil {
			dbFile = f
		}
	}

	if dbFile == nil {
		return nil, fmt.Errorf("no collection.anki2 or collection.anki21 found in %s", apkgPath)
	}

	// Extract SQLite database to a temporary file
	tmpDB, err := os.CreateTemp("", "rota_anki_*.db")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file for anki db: %w", err)
	}
	tmpDBPath := tmpDB.Name()
	defer os.Remove(tmpDBPath)

	rc, err := dbFile.Open()
	if err != nil {
		tmpDB.Close()
		return nil, fmt.Errorf("failed to open anki db in zip: %w", err)
	}

	_, err = io.Copy(tmpDB, rc)
	rc.Close()
	tmpDB.Close()
	if err != nil {
		return nil, fmt.Errorf("failed to extract anki database: %w", err)
	}

	// 3. Query Anki SQLite Database
	db, err := sql.Open("sqlite", tmpDBPath+"?_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, fmt.Errorf("failed to connect to extracted anki db: %w", err)
	}
	defer db.Close()

	pkg := &AnkiPackage{
		Decks:      make(map[int64]*AnkiDeck),
		Models:     make(map[int64]*AnkiModel),
		Notes:      make(map[int64]*AnkiNote),
		MediaMap:   mediaMap,
		MediaCount: mediaCount,
	}

	// Read collection table (col) for models and decks JSON
	if err := readCollectionMetadata(db, pkg); err != nil {
		return nil, fmt.Errorf("failed to read anki collection metadata: %w", err)
	}

	// Read Notes
	if err := readNotes(db, pkg); err != nil {
		return nil, fmt.Errorf("failed to read anki notes: %w", err)
	}

	// Read Cards
	if err := readCards(db, pkg); err != nil {
		return nil, fmt.Errorf("failed to read anki cards: %w", err)
	}

	return pkg, nil
}

func readCollectionMetadata(db *sql.DB, pkg *AnkiPackage) error {
	var modelsJSON, decksJSON string
	err := db.QueryRow("SELECT models, decks FROM col LIMIT 1").Scan(&modelsJSON, &decksJSON)
	if err != nil {
		return err
	}

	// Parse Models
	rawModels := make(map[string]json.RawMessage)
	if err := json.Unmarshal([]byte(modelsJSON), &rawModels); err == nil {
		for idStr, raw := range rawModels {
			var m AnkiModel
			if err := json.Unmarshal(raw, &m); err == nil {
				if m.ID == 0 {
					if id, err := strconv.ParseInt(idStr, 10, 64); err == nil {
						m.ID = id
					}
				}
				pkg.Models[m.ID] = &m
			}
		}
	}

	// Parse Decks
	rawDecks := make(map[string]json.RawMessage)
	if err := json.Unmarshal([]byte(decksJSON), &rawDecks); err == nil {
		for idStr, raw := range rawDecks {
			var d AnkiDeck
			if err := json.Unmarshal(raw, &d); err == nil {
				if d.ID == 0 {
					if id, err := strconv.ParseInt(idStr, 10, 64); err == nil {
						d.ID = id
					}
				}
				// Clean deck name (Anki uses \x1f or :: for subdecks, e.g. "Biology::Cardiology")
				cleanName := strings.ReplaceAll(d.Name, "\x1f", " / ")
				cleanName = strings.ReplaceAll(cleanName, "::", " / ")
				d.Name = cleanName
				pkg.Decks[d.ID] = &d
			}
		}
	}

	return nil
}

func readNotes(db *sql.DB, pkg *AnkiPackage) error {
	rows, err := db.Query("SELECT id, mid, flds, tags FROM notes")
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var n AnkiNote
		var fldsStr, tagsStr string
		if err := rows.Scan(&n.ID, &n.Mid, &fldsStr, &tagsStr); err != nil {
			return err
		}

		// Fields are separated by \x1f (ASCII 31)
		n.Fields = strings.Split(fldsStr, "\x1f")

		// Tags are separated by spaces
		rawTags := strings.Fields(tagsStr)
		var cleanTags []string
		for _, t := range rawTags {
			t = strings.TrimSpace(t)
			if t != "" {
				cleanTags = append(cleanTags, t)
			}
		}
		n.Tags = cleanTags

		pkg.Notes[n.ID] = &n
	}

	return rows.Err()
}

func readCards(db *sql.DB, pkg *AnkiPackage) error {
	rows, err := db.Query("SELECT id, nid, did, ord, type, queue, due, ivl, factor, reps, lapses FROM cards")
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var c AnkiCard
		if err := rows.Scan(&c.ID, &c.Nid, &c.Did, &c.Ord, &c.Type, &c.Queue, &c.Due, &c.Ivl, &c.Factor, &c.Reps, &c.Lapses); err != nil {
			return err
		}
		pkg.Cards = append(pkg.Cards, &c)
	}

	return rows.Err()
}

// ConvertToCards transforms Anki notes and cards into Rota converted cards.
func (pkg *AnkiPackage) ConvertToCards(mediaRelPrefix string) []*ConvertedCard {
	var converted []*ConvertedCard
	now := time.Now().UTC()

	// Track processed (nid, ord) to avoid duplicate cards
	seen := make(map[string]bool)

	for _, card := range pkg.Cards {
		note, found := pkg.Notes[card.Nid]
		if !found || len(note.Fields) == 0 {
			continue
		}

		key := fmt.Sprintf("%d:%d", card.Nid, card.Ord)
		if seen[key] {
			continue
		}
		seen[key] = true

		deckName := "Default"
		if deck, ok := pkg.Decks[card.Did]; ok && deck.Name != "" {
			deckName = deck.Name
		}

		model, modelFound := pkg.Models[note.Mid]
		isClozeModel := modelFound && (model.Type == 1 || strings.Contains(strings.ToLower(model.Name), "cloze"))

		if isClozeModel {
			// Cloze Note Type
			text := note.Fields[0]
			extra := ""
			if len(note.Fields) > 1 {
				extra = note.Fields[1]
			}

			// Convert Cloze to Rota syntax
			mdText := HTMLToMarkdown(text, mediaRelPrefix)
			rotaCloze := ConvertAnkiClozeToRota(mdText)
			mdExtra := HTMLToMarkdown(extra, mediaRelPrefix)

			if strings.TrimSpace(rotaCloze) != "" {
				converted = append(converted, &ConvertedCard{
					DeckName:   deckName,
					CardType:   "cloze",
					Prompt:     rotaCloze,
					Answer:     mdExtra,
					Tags:       note.Tags,
					AnkiCardID: card.ID,
					Due:        calculateDueTime(card, now),
					Interval:   card.Ivl,
					Reps:       card.Reps,
					Lapses:     card.Lapses,
				})
			}
		} else {
			// Standard Question / Answer
			front := note.Fields[0]
			back := ""
			if len(note.Fields) > 1 {
				back = note.Fields[1]
			}

			// Handle Basic & Reversed cards
			if card.Ord == 1 && len(note.Fields) >= 2 {
				// Reverse card (Back as question, Front as answer)
				front, back = back, front
			}

			mdFront := HTMLToMarkdown(front, mediaRelPrefix)
			mdBack := HTMLToMarkdown(back, mediaRelPrefix)

			// If back has additional fields (e.g. Extra), append them
			if len(note.Fields) > 2 && card.Ord == 0 {
				for i := 2; i < len(note.Fields); i++ {
					if extra := HTMLToMarkdown(note.Fields[i], mediaRelPrefix); extra != "" {
						mdBack += "\n\n" + extra
					}
				}
			}

			if strings.TrimSpace(mdFront) != "" && strings.TrimSpace(mdBack) != "" {
				converted = append(converted, &ConvertedCard{
					DeckName:   deckName,
					CardType:   "qa",
					Prompt:     mdFront,
					Answer:     mdBack,
					Tags:       note.Tags,
					AnkiCardID: card.ID,
					Due:        calculateDueTime(card, now),
					Interval:   card.Ivl,
					Reps:       card.Reps,
					Lapses:     card.Lapses,
				})
			}
		}
	}

	return converted
}

func calculateDueTime(c *AnkiCard, now time.Time) time.Time {
	if c.Queue == 2 && c.Due > 0 {
		// Review queue: due is day offset relative to collection creation (or days from now)
		// For simplicity and safety, if interval > 0, set due based on interval
		if c.Ivl > 0 {
			return now.Add(time.Duration(c.Ivl) * 24 * time.Hour)
		}
	}
	return now
}
