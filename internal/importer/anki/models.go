package anki

import (
	"time"
)

// AnkiDeck represents deck metadata from Anki's collection.
type AnkiDeck struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// AnkiField represents a field definition in an Anki Model.
type AnkiField struct {
	Name string `json:"name"`
	Ord  int    `json:"ord"`
}

// AnkiTemplate represents a card template in an Anki Model.
type AnkiTemplate struct {
	Name string `json:"name"`
	Ord  int    `json:"ord"`
	QFmt string `json:"qfmt"`
	AFmt string `json:"afmt"`
}

// AnkiModel represents a note type (e.g. Basic, Cloze, Basic & Reversed).
type AnkiModel struct {
	ID    int64          `json:"id"`
	Name  string         `json:"name"`
	Type  int            `json:"type"` // 0: Standard, 1: Cloze
	Flds  []AnkiField    `json:"flds"`
	Tmpls []AnkiTemplate `json:"tmpls"`
}

// AnkiNote represents a row from the Anki notes table.
type AnkiNote struct {
	ID     int64
	Mid    int64
	Fields []string
	Tags   []string
}

// AnkiCard represents a row from the Anki cards table.
type AnkiCard struct {
	ID       int64
	Nid      int64
	Did      int64
	Ord      int
	Type     int
	Queue    int
	Due      int64
	Ivl      int64
	Factor   int64
	Reps     int64
	Lapses   int64
	ModTime  time.Time
}

// ConvertedCard is the normalized card ready for Markdown export.
type ConvertedCard struct {
	DeckName   string
	CardType   string // "qa" or "cloze"
	Prompt     string
	Answer     string
	Tags       []string
	AnkiCardID int64
	Due        time.Time
	Interval   int64
	Reps       int64
	Lapses     int64
}

// ImportResult summarizes the outcome of importing an Anki package.
type ImportResult struct {
	TotalDecks     int
	TotalNotes     int
	TotalCards     int
	MediaExtracted int
	GeneratedFiles []string
}
