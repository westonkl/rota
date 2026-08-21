package card

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/open-spaced-repetition/go-fsrs/v4"
)

// CardType identifies the kind of flashcard.
type CardType string

const (
	TypeQA    CardType = "qa"
	TypeCloze CardType = "cloze"
	TypeBasic CardType = "basic"
)

// Card represents a parsed markdown flashcard.
type Card struct {
	ID         string     `json:"id"`
	Hash       string     `json:"hash"`
	Deck       string     `json:"deck"`
	FilePath   string     `json:"file_path"`
	LineNumber int        `json:"line_number"`
	Type       CardType   `json:"type"`
	Prompt     string     `json:"prompt"`
	Answer     string     `json:"answer"`
	Extra      string     `json:"extra,omitempty"`
	Tags       []string   `json:"tags,omitempty"`
	FSRS       *FSRSState `json:"fsrs,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

// FSRSState mirrors and persists the go-fsrs Card state.
type FSRSState struct {
	Due           time.Time  `json:"due"`
	Stability     float64    `json:"stability"`
	Difficulty    float64    `json:"difficulty"`
	ElapsedDays   uint64     `json:"elapsed_days"`
	ScheduledDays uint64     `json:"scheduled_days"`
	Reps          uint64     `json:"reps"`
	Lapses        uint64     `json:"lapses"`
	State         fsrs.State `json:"state"`
	LastReview    time.Time  `json:"last_review"`
}

// ReviewLogEntry represents a recorded review session event.
type ReviewLogEntry struct {
	ID            int64       `json:"id"`
	CardID        string      `json:"card_id"`
	Rating        fsrs.Rating `json:"rating"`
	State         fsrs.State  `json:"state"`
	Due           time.Time   `json:"due"`
	Stability     float64     `json:"stability"`
	Difficulty    float64     `json:"difficulty"`
	ElapsedDays   uint64      `json:"elapsed_days"`
	LastElapsed   uint64      `json:"last_elapsed"`
	ScheduledDays uint64      `json:"scheduled_days"`
	ReviewTime    time.Time   `json:"review_time"`
}

// ComputeMeaningHash creates a normalized, stable SHA-256 hash of the card content.
// By hashing the prompt and answer (without deck coupling), cards can be renamed,
// reorganized across decks, or moved without resetting review history.
func ComputeMeaningHash(prompt, answer string) string {
	normPrompt := normalizeText(prompt)
	normAnswer := normalizeText(answer)

	input := fmt.Sprintf("%s|%s", normPrompt, normAnswer)
	h := sha256.Sum256([]byte(input))
	return hex.EncodeToString(h[:16]) // 32-character hex string
}

var whitespaceRegex = regexp.MustCompile(`\s+`)

func normalizeText(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = whitespaceRegex.ReplaceAllString(s, " ")
	return s
}

// ToFSRSCard converts our FSRSState to the library fsrs.Card.
func (f *FSRSState) ToFSRSCard() fsrs.Card {
	if f == nil {
		return fsrs.NewCard()
	}
	return fsrs.Card{
		Due:           f.Due,
		Stability:     f.Stability,
		Difficulty:    f.Difficulty,
		ScheduledDays: f.ScheduledDays,
		Reps:          f.Reps,
		Lapses:        f.Lapses,
		State:         f.State,
		LastReview:    f.LastReview,
	}
}

// FromFSRSCard creates an FSRSState from a library fsrs.Card.
func FromFSRSCard(c fsrs.Card) *FSRSState {
	return &FSRSState{
		Due:           c.Due,
		Stability:     c.Stability,
		Difficulty:    c.Difficulty,
		ScheduledDays: c.ScheduledDays,
		Reps:          c.Reps,
		Lapses:        c.Lapses,
		State:         c.State,
		LastReview:    c.LastReview,
	}
}

// NewDefaultFSRSState creates a new initial state for a fresh card.
func NewDefaultFSRSState() *FSRSState {
	card := fsrs.NewCard()
	return FromFSRSCard(card)
}
