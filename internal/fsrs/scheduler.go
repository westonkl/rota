package fsrs

import (
	"fmt"
	"math"
	"time"

	gofsrs "github.com/open-spaced-repetition/go-fsrs/v4"
)

// Scheduler coordinates FSRS calculations and scheduling previews.
type Scheduler struct {
	fsrs *gofsrs.FSRS
}

// Config holds options for the FSRS scheduler.
type Config struct {
	RequestRetention float64
	MaximumInterval  float64
	Weights          *gofsrs.Weights
	EnableFuzz       bool
}

// DefaultConfig returns recommended FSRS defaults.
func DefaultConfig() Config {
	return Config{
		RequestRetention: 0.90,
		MaximumInterval:  36500,
		EnableFuzz:       true,
	}
}

// NewScheduler creates an FSRS scheduler instance.
func NewScheduler(cfg Config) *Scheduler {
	param := gofsrs.DefaultParam()
	if cfg.RequestRetention > 0 {
		param.RequestRetention = cfg.RequestRetention
	}
	if cfg.MaximumInterval > 0 {
		param.MaximumInterval = cfg.MaximumInterval
	}
	if cfg.Weights != nil {
		param.W = *cfg.Weights
	}
	param.EnableFuzz = cfg.EnableFuzz

	engine := gofsrs.NewFSRS(param)
	return &Scheduler{
		fsrs: engine,
	}
}

// Next calculates the next card state and review log given a rating.
func (s *Scheduler) Next(card gofsrs.Card, now time.Time, rating gofsrs.Rating) (gofsrs.Card, gofsrs.ReviewLog) {
	info, err := s.fsrs.Next(card, now, rating)
	if err != nil {
		return card, gofsrs.ReviewLog{}
	}
	return info.Card, info.ReviewLog
}

// Repeat returns the scheduling options for all 4 ratings (Again, Hard, Good, Easy).
func (s *Scheduler) Repeat(card gofsrs.Card, now time.Time) gofsrs.RecordLog {
	records, err := s.fsrs.Repeat(card, now)
	if err != nil {
		return make(gofsrs.RecordLog)
	}
	return records
}

// GetRetrievability returns the probability of recalling the card at time `now`.
func (s *Scheduler) GetRetrievability(card gofsrs.Card, now time.Time) float64 {
	r, err := s.fsrs.Retrievability(card, now)
	if err != nil {
		return 0.0
	}
	return r
}

// PreviewIntervals returns a map of human-readable interval strings for each rating button.
func (s *Scheduler) PreviewIntervals(card gofsrs.Card, now time.Time) map[gofsrs.Rating]string {
	records := s.Repeat(card, now)
	previews := make(map[gofsrs.Rating]string)

	for rating, info := range records {
		diff := info.Card.Due.Sub(now)
		previews[rating] = FormatDuration(diff)
	}

	return previews
}

// FormatDuration formats a time duration into a compact flashcard interval (e.g. 10m, 1d, 4d, 1.2m, 2y).
func FormatDuration(d time.Duration) string {
	if d <= 0 {
		return "<1m"
	}

	minutes := d.Minutes()
	if minutes < 1 {
		return "<1m"
	}
	if minutes < 60 {
		return fmt.Sprintf("%dm", int(math.Round(minutes)))
	}

	hours := d.Hours()
	if hours < 24 {
		return fmt.Sprintf("%dh", int(math.Round(hours)))
	}

	days := hours / 24.0
	if days < 30 {
		return fmt.Sprintf("%dd", int(math.Round(days)))
	}

	months := days / 30.4375
	if months < 12 {
		if months < 3 {
			return fmt.Sprintf("%.1fmo", months)
		}
		return fmt.Sprintf("%dmo", int(math.Round(months)))
	}

	years := days / 365.25
	return fmt.Sprintf("%.1fy", years)
}

// RatingName returns the capitalized string representation of a rating.
func RatingName(r gofsrs.Rating) string {
	switch r {
	case gofsrs.Again:
		return "Again"
	case gofsrs.Hard:
		return "Hard"
	case gofsrs.Good:
		return "Good"
	case gofsrs.Easy:
		return "Easy"
	default:
		return "Unknown"
	}
}

// StateName returns the human readable name of a card state.
func StateName(st gofsrs.State) string {
	switch st {
	case gofsrs.New:
		return "New"
	case gofsrs.Learning:
		return "Learning"
	case gofsrs.Review:
		return "Review"
	case gofsrs.Relearning:
		return "Relearning"
	default:
		return "Unknown"
	}
}
