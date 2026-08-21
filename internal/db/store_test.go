package db

import (
	"testing"
	"time"

	gofsrs "github.com/open-spaced-repetition/go-fsrs/v4"
	"rota/internal/card"
)

func TestStoreSyncAndReview(t *testing.T) {
	store, err := Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open memory db: %v", err)
	}
	defer store.Close()

	now := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)

	// 1. Sync cards
	c1 := &card.Card{
		ID:         "card1",
		Hash:       "hash1",
		Deck:       "golang",
		FilePath:   "/path/to/notes.md",
		LineNumber: 10,
		Type:       card.TypeQA,
		Prompt:     "What is a goroutine?",
		Answer:     "A lightweight thread.",
		Tags:       []string{"go", "concurrency"},
	}
	c2 := &card.Card{
		ID:         "card2",
		Hash:       "hash2",
		Deck:       "golang",
		FilePath:   "/path/to/notes.md",
		LineNumber: 20,
		Type:       card.TypeCloze,
		Prompt:     "Channels can be [...] or [...].",
		Answer:     "Channels can be **[buffered]** or **[unbuffered]**.",
		Tags:       []string{"go"},
	}

	syncRes, err := store.SyncFileCards("/path/to/notes.md", []*card.Card{c1, c2})
	if err != nil {
		t.Fatalf("sync error: %v", err)
	}
	if syncRes.Added != 2 {
		t.Errorf("expected 2 cards added, got %d", syncRes.Added)
	}

	// 2. Query due cards (both should be due since they are New)
	due, err := store.GetDueCards("", 10, now)
	if err != nil {
		t.Fatalf("get due cards error: %v", err)
	}
	if len(due) != 2 {
		t.Fatalf("expected 2 due cards, got %d", len(due))
	}

	// 3. Record review for card1
	nextFSRS := &card.FSRSState{
		Due:           now.Add(24 * time.Hour),
		Stability:     2.0,
		Difficulty:    5.0,
		ElapsedDays:   0,
		ScheduledDays: 1,
		Reps:          1,
		Lapses:        0,
		State:         gofsrs.Learning,
		LastReview:    now,
	}
	err = store.RecordReview(c1.ID, gofsrs.Good, nextFSRS, now)
	if err != nil {
		t.Fatalf("record review error: %v", err)
	}

	// 4. Query due cards again right now (only card2 should be due now)
	dueAfter, err := store.GetDueCards("", 10, now)
	if err != nil {
		t.Fatalf("get due cards error: %v", err)
	}
	if len(dueAfter) != 1 {
		t.Errorf("expected 1 due card after review, got %d", len(dueAfter))
	}

	// 5. Check stats summary
	stats, err := store.GetStatsSummary(now)
	if err != nil {
		t.Fatalf("get stats error: %v", err)
	}
	if stats.TotalCards != 2 {
		t.Errorf("expected 2 total cards, got %d", stats.TotalCards)
	}
	if stats.TotalReviews != 1 {
		t.Errorf("expected 1 total review, got %d", stats.TotalReviews)
	}
	if stats.CurrentStreak != 1 {
		t.Errorf("expected streak of 1, got %d", stats.CurrentStreak)
	}

	// 6. Test Undo review
	err = store.UndoLastReview(c1.ID)
	if err != nil {
		t.Fatalf("undo review error: %v", err)
	}

	dueAfterUndo, err := store.GetDueCards("", 10, now)
	if err != nil {
		t.Fatalf("get due cards error: %v", err)
	}
	if len(dueAfterUndo) != 2 {
		t.Errorf("expected 2 due cards after undo, got %d", len(dueAfterUndo))
	}
}
