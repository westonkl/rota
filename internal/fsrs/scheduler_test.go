package fsrs

import (
	"testing"
	"time"

	gofsrs "github.com/open-spaced-repetition/go-fsrs/v3"
)

func TestSchedulerTransitions(t *testing.T) {
	s := NewScheduler(DefaultConfig())
	card := gofsrs.NewCard()
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	// New card transitions
	nextCard, log := s.Next(card, now, gofsrs.Good)
	if nextCard.State != gofsrs.Learning && nextCard.State != gofsrs.Review {
		t.Errorf("unexpected next state: %v", nextCard.State)
	}
	if log.Rating != gofsrs.Good {
		t.Errorf("expected rating Good, got %v", log.Rating)
	}

	// Test interval preview
	previews := s.PreviewIntervals(card, now)
	if len(previews) != 4 {
		t.Fatalf("expected 4 previews, got %d", len(previews))
	}
	if previews[gofsrs.Again] == "" || previews[gofsrs.Good] == "" {
		t.Errorf("empty preview strings: %+v", previews)
	}
}

func TestFormatDuration(t *testing.T) {
	cases := []struct {
		d        time.Duration
		expected string
	}{
		{5 * time.Minute, "5m"},
		{2 * time.Hour, "2h"},
		{3 * 24 * time.Hour, "3d"},
		{45 * 24 * time.Hour, "1.5mo"},
		{400 * 24 * time.Hour, "1.1y"},
	}

	for _, c := range cases {
		got := FormatDuration(c.d)
		if got != c.expected {
			t.Errorf("FormatDuration(%v) = %s, expected %s", c.d, got, c.expected)
		}
	}
}
