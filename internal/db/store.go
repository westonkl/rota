package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	gofsrs "github.com/open-spaced-repetition/go-fsrs/v4"
	"rota/internal/card"
)

// DeckSummary aggregates stats for a single deck.
type DeckSummary struct {
	Name          string     `json:"name"`
	TotalCards    int        `json:"total_cards"`
	NewCards      int        `json:"new_cards"`
	LearningCards int        `json:"learning_cards"`
	ReviewCards   int        `json:"review_cards"`
	DueCards      int        `json:"due_cards"`
	NextDue       *time.Time `json:"next_due,omitempty"`
}

// StatsSummary gives high-level metrics for the entire collection.
type StatsSummary struct {
	TotalCards    int                   `json:"total_cards"`
	TotalDecks    int                   `json:"total_decks"`
	TotalReviews  int                   `json:"total_reviews"`
	RetentionRate float64               `json:"retention_rate"`
	CurrentStreak int                   `json:"current_streak"`
	DueToday      int                   `json:"due_today"`
	NewToday      int                   `json:"new_today"`
	ReviewsByDay  map[string]int        `json:"reviews_by_day"`
	RatingsCount  map[gofsrs.Rating]int `json:"ratings_count"`
	DeckBreakdown []DeckSummary         `json:"deck_breakdown"`
}

// CardFilter defines query filtering for cards.
type CardFilter struct {
	Deck        string
	FilePath    string
	DueOnly     bool
	State       *gofsrs.State
	SearchQuery string
	Limit       int
	Offset      int
	Now         time.Time
}

// SyncedFileInfo stores sync metadata for a tracked markdown file.
type SyncedFileInfo struct {
	FilePath     string    `json:"file_path"`
	LastModified time.Time `json:"last_modified"`
	CardCount    int       `json:"card_count"`
	ContentHash  string    `json:"content_hash"`
}

// SyncResult tracks the outcome of syncing cards from a file.
type SyncResult struct {
	Added     int
	Updated   int
	Unchanged int
	Deleted   int
}

// SyncFileCards synchronizes a slice of parsed cards from a given file into the database.
func (s *Store) SyncFileCards(filePath string, parsed []*card.Card) (*SyncResult, error) {
	filePath = filepath.Clean(filePath)
	if abs, err := filepath.Abs(filePath); err == nil {
		filePath = abs
	}
	if realPath, err := filepath.EvalSymlinks(filePath); err == nil {
		filePath = realPath
	}

	tx, err := s.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	result := &SyncResult{}
	now := time.Now().UTC()

	// Get all existing cards for this file path (or basename) using case-insensitive path comparison
	baseName := filepath.Base(filePath)
	rows, err := tx.Query(`
		SELECT id, hash, deck, file_path, line_number, prompt, answer, tags 
		FROM cards 
		WHERE LOWER(file_path) = LOWER(?) OR LOWER(file_path) LIKE LOWER(?)
	`, filePath, "%"+baseName)
	if err != nil {
		return nil, fmt.Errorf("failed to query existing cards: %w", err)
	}
	defer rows.Close()

	existingByID := make(map[string]*card.Card)
	existingByHash := make(map[string]*card.Card)
	for rows.Next() {
		var c card.Card
		var tagsStr sql.NullString
		if err := rows.Scan(&c.ID, &c.Hash, &c.Deck, &c.FilePath, &c.LineNumber, &c.Prompt, &c.Answer, &tagsStr); err != nil {
			return nil, err
		}
		if tagsStr.Valid && tagsStr.String != "" {
			c.Tags = strings.Split(tagsStr.String, ",")
		}
		existingByID[c.ID] = &c
		existingByHash[c.Hash] = &c
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error reading cards: %w", err)
	}
	rows.Close()

	seenIDs := make(map[string]bool)

	for _, c := range parsed {
		tagsJoined := strings.Join(c.Tags, ",")

		existing, found := existingByID[c.ID]
		if !found {
			existing, found = existingByHash[c.Hash]
		}

		if found {
			seenIDs[existing.ID] = true
			// Card already exists with matching ID or Hash
			if existing.LineNumber != c.LineNumber || existing.Prompt != c.Prompt || existing.Answer != c.Answer || existing.Deck != c.Deck || existing.FilePath != filePath || strings.Join(existing.Tags, ",") != tagsJoined {
				_, err := tx.Exec(`
					UPDATE cards 
					SET deck = ?, file_path = ?, line_number = ?, prompt = ?, answer = ?, extra = ?, tags = ?, updated_at = ?
					WHERE id = ?
				`, c.Deck, filePath, c.LineNumber, c.Prompt, c.Answer, c.Extra, tagsJoined, now, existing.ID)
				if err != nil {
					return nil, fmt.Errorf("failed to update card: %w", err)
				}
				result.Updated++
			} else {
				result.Unchanged++
			}
			existingByID[existing.ID] = c
			existingByHash[existing.Hash] = c
		} else {
			seenIDs[c.ID] = true
			// Brand new card (with ON CONFLICT fallback for complete safety)
			_, err := tx.Exec(`
				INSERT INTO cards (id, hash, deck, file_path, line_number, card_type, prompt, answer, extra, tags, created_at, updated_at)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
				ON CONFLICT(id) DO UPDATE SET
					hash = excluded.hash,
					deck = excluded.deck,
					file_path = excluded.file_path,
					line_number = excluded.line_number,
					card_type = excluded.card_type,
					prompt = excluded.prompt,
					answer = excluded.answer,
					extra = excluded.extra,
					tags = excluded.tags,
					updated_at = excluded.updated_at
			`, c.ID, c.Hash, c.Deck, filePath, c.LineNumber, string(c.Type), c.Prompt, c.Answer, c.Extra, tagsJoined, now, now)
			if err != nil {
				return nil, fmt.Errorf("failed to insert new card: %w", err)
			}

			// Initialize FSRS state for new card (if not already existing)
			initialState := card.NewDefaultFSRSState()
			dueStr := initialState.Due.UTC().Format("2006-01-02 15:04:05")
			var lastRevStr sql.NullString
			if !initialState.LastReview.IsZero() {
				lastRevStr = sql.NullString{String: initialState.LastReview.UTC().Format("2006-01-02 15:04:05"), Valid: true}
			}
			_, err = tx.Exec(`
				INSERT INTO card_fsrs (card_id, due, stability, difficulty, elapsed_days, scheduled_days, reps, lapses, state, last_review)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
				ON CONFLICT(card_id) DO NOTHING
			`, c.ID, dueStr, initialState.Stability, initialState.Difficulty, initialState.ElapsedDays, initialState.ScheduledDays, initialState.Reps, initialState.Lapses, int(initialState.State), lastRevStr)
			if err != nil {
				return nil, fmt.Errorf("failed to initialize fsrs state: %w", err)
			}

			existingByID[c.ID] = c
			existingByHash[c.Hash] = c
			result.Added++
		}
	}

	// Remove cards in database that no longer exist in this file
	for id, existing := range existingByID {
		if !seenIDs[id] && (strings.EqualFold(existing.FilePath, filePath) || strings.HasSuffix(strings.ToLower(existing.FilePath), strings.ToLower(baseName))) {
			_, err := tx.Exec("DELETE FROM cards WHERE id = ?", id)
			if err != nil {
				return nil, fmt.Errorf("failed to delete removed card: %w", err)
			}
			result.Deleted++
		}
	}

	// Update sync_files tracking table
	var fileModTime time.Time
	if fi, err := os.Stat(filePath); err == nil {
		fileModTime = fi.ModTime().UTC()
	} else {
		fileModTime = now
	}
	modTimeStr := fileModTime.Format("2006-01-02 15:04:05")
	_, err = tx.Exec(`
		INSERT INTO sync_files (file_path, last_modified, card_count, content_hash)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(file_path) DO UPDATE SET
			last_modified = excluded.last_modified,
			card_count = excluded.card_count,
			content_hash = excluded.content_hash
	`, filePath, modTimeStr, len(parsed), "")
	if err != nil {
		return nil, fmt.Errorf("failed to update sync_files: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit sync transaction: %w", err)
	}

	return result, nil
}

// GetDueCards retrieves cards due for review, intelligently ordered:
// 1. Learning / Relearning cards due
// 2. Review cards due (most overdue first)
// 3. New cards
func (s *Store) GetDueCards(deck string, limit int, now time.Time) ([]*card.Card, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}

	query := `
		SELECT 
			c.id, c.hash, c.deck, c.file_path, c.line_number, c.card_type, c.prompt, c.answer, c.extra, c.tags, c.created_at, c.updated_at,
			f.due, f.stability, f.difficulty, f.elapsed_days, f.scheduled_days, f.reps, f.lapses, f.state, f.last_review
		FROM cards c
		JOIN card_fsrs f ON c.id = f.card_id
		WHERE (f.state = 0 OR f.due <= ?)
	`
	args := []any{now}

	if deck != "" {
		query += " AND (c.deck = ? OR c.deck LIKE ?)"
		args = append(args, deck, "%"+deck+"%")
	}

	// Ordering:
	// State 1,3 (Learning/Relearning) first, then State 2 (Review by due ASC), then State 0 (New cards by created_at)
	query += `
		ORDER BY 
			CASE 
				WHEN f.state IN (1, 3) THEN 1
				WHEN f.state = 2 THEN 2
				WHEN f.state = 0 THEN 3
				ELSE 4
			END ASC,
			f.due ASC,
			c.created_at ASC
	`

	if limit > 0 {
		query += " LIMIT ?"
		args = append(args, limit)
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query due cards: %w", err)
	}
	defer rows.Close()

	return scanCards(rows)
}

// ListCards queries cards with optional filtering.
func (s *Store) ListCards(filter CardFilter) ([]*card.Card, error) {
	query := `
		SELECT 
			c.id, c.hash, c.deck, c.file_path, c.line_number, c.card_type, c.prompt, c.answer, c.extra, c.tags, c.created_at, c.updated_at,
			f.due, f.stability, f.difficulty, f.elapsed_days, f.scheduled_days, f.reps, f.lapses, f.state, f.last_review
		FROM cards c
		LEFT JOIN card_fsrs f ON c.id = f.card_id
		WHERE 1=1
	`
	var args []any

	if filter.Deck != "" {
		query += " AND (c.deck = ? OR c.deck LIKE ?)"
		args = append(args, filter.Deck, "%"+filter.Deck+"%")
	}

	if filter.FilePath != "" {
		query += " AND (c.file_path = ? OR c.file_path LIKE ?)"
		args = append(args, filter.FilePath, "%"+filter.FilePath+"%")
	}

	if filter.State != nil {
		query += " AND f.state = ?"
		args = append(args, int(*filter.State))
	}

	if filter.DueOnly {
		now := filter.Now
		if now.IsZero() {
			now = time.Now().UTC()
		}
		query += " AND (f.state = 0 OR f.due <= ?)"
		args = append(args, now)
	}

	if filter.SearchQuery != "" {
		like := "%" + filter.SearchQuery + "%"
		query += " AND (c.prompt LIKE ? OR c.answer LIKE ? OR c.tags LIKE ?)"
		args = append(args, like, like, like)
	}

	query += " ORDER BY c.deck ASC, c.line_number ASC"

	if filter.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, filter.Limit)
	}
	if filter.Offset > 0 {
		query += " OFFSET ?"
		args = append(args, filter.Offset)
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list cards: %w", err)
	}
	defer rows.Close()

	return scanCards(rows)
}

// RecordReview persists an updated FSRS state and appends an entry to review_logs in an atomic transaction.
func (s *Store) RecordReview(cardID string, rating gofsrs.Rating, newState *card.FSRSState, reviewTime time.Time) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin review transaction: %w", err)
	}
	defer tx.Rollback()

	if reviewTime.IsZero() {
		reviewTime = time.Now().UTC()
	}

	dueStr := newState.Due.UTC().Format("2006-01-02 15:04:05")
	reviewTimeStr := reviewTime.UTC().Format("2006-01-02 15:04:05")

	// Update card_fsrs
	_, err = tx.Exec(`
		UPDATE card_fsrs 
		SET due = ?, stability = ?, difficulty = ?, elapsed_days = ?, scheduled_days = ?, reps = ?, lapses = ?, state = ?, last_review = ?
		WHERE card_id = ?
	`, dueStr, newState.Stability, newState.Difficulty, newState.ElapsedDays, newState.ScheduledDays, newState.Reps, newState.Lapses, int(newState.State), reviewTimeStr, cardID)
	if err != nil {
		return fmt.Errorf("failed to update card_fsrs: %w", err)
	}

	// Log review
	_, err = tx.Exec(`
		INSERT INTO review_logs (card_id, rating, state, due, stability, difficulty, elapsed_days, last_elapsed, scheduled_days, review_time)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, cardID, int(rating), int(newState.State), dueStr, newState.Stability, newState.Difficulty, newState.ElapsedDays, 0, newState.ScheduledDays, reviewTimeStr)
	if err != nil {
		return fmt.Errorf("failed to insert review_log: %w", err)
	}

	return tx.Commit()
}

// UndoLastReview reverts the latest review for a card if a log exists.
func (s *Store) UndoLastReview(cardID string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Find the latest review log
	var logID int64
	err = tx.QueryRow("SELECT id FROM review_logs WHERE card_id = ? ORDER BY review_time DESC, id DESC LIMIT 1", cardID).Scan(&logID)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("no reviews found for card %s to undo", cardID)
		}
		return err
	}

	// Delete the latest review log
	if _, err := tx.Exec("DELETE FROM review_logs WHERE id = ?", logID); err != nil {
		return err
	}

	// Recompute or fetch previous review log if one exists
	var prevDue time.Time
	var prevStability, prevDifficulty float64
	var prevElapsed, prevScheduled uint64
	var prevState int
	var prevReviewTime time.Time

	err = tx.QueryRow(`
		SELECT due, stability, difficulty, elapsed_days, scheduled_days, state, review_time
		FROM review_logs
		WHERE card_id = ?
		ORDER BY review_time DESC, id DESC LIMIT 1
	`, cardID).Scan(&prevDue, &prevStability, &prevDifficulty, &prevElapsed, &prevScheduled, &prevState, &prevReviewTime)

	if err == sql.ErrNoRows {
		// Reset to brand new initial state
		initial := card.NewDefaultFSRSState()
		_, err = tx.Exec(`
			UPDATE card_fsrs 
			SET due = ?, stability = ?, difficulty = ?, elapsed_days = ?, scheduled_days = ?, reps = 0, lapses = 0, state = ?, last_review = ?
			WHERE card_id = ?
		`, initial.Due, initial.Stability, initial.Difficulty, initial.ElapsedDays, initial.ScheduledDays, int(initial.State), initial.LastReview, cardID)
	} else if err == nil {
		// Revert to previous logged state
		_, err = tx.Exec(`
			UPDATE card_fsrs 
			SET due = ?, stability = ?, difficulty = ?, elapsed_days = ?, scheduled_days = ?, state = ?, last_review = ?
			WHERE card_id = ?
		`, prevDue, prevStability, prevDifficulty, prevElapsed, prevScheduled, prevState, prevReviewTime, cardID)
	} else {
		return err
	}

	if err != nil {
		return fmt.Errorf("failed to revert card state: %w", err)
	}

	return tx.Commit()
}

// GetDeckSummaries provides statistics per deck.
func (s *Store) GetDeckSummaries(now time.Time) ([]DeckSummary, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}

	query := `
		SELECT 
			c.deck,
			COUNT(c.id) as total,
			SUM(CASE WHEN f.state = 0 THEN 1 ELSE 0 END) as new_count,
			SUM(CASE WHEN f.state IN (1, 3) THEN 1 ELSE 0 END) as learning_count,
			SUM(CASE WHEN f.state = 2 THEN 1 ELSE 0 END) as review_count,
			SUM(CASE WHEN (f.state = 0 OR f.due <= ?) THEN 1 ELSE 0 END) as due_count,
			MIN(f.due) as next_due
		FROM cards c
		JOIN card_fsrs f ON c.id = f.card_id
		GROUP BY c.deck
		ORDER BY c.deck ASC
	`
	rows, err := s.db.Query(query, now)
	if err != nil {
		return nil, fmt.Errorf("failed to get deck summaries: %w", err)
	}
	defer rows.Close()

	var summaries []DeckSummary
	for rows.Next() {
		var sum DeckSummary
		var nextDueStr sql.NullString
		if err := rows.Scan(&sum.Name, &sum.TotalCards, &sum.NewCards, &sum.LearningCards, &sum.ReviewCards, &sum.DueCards, &nextDueStr); err != nil {
			return nil, err
		}
		if nextDueStr.Valid && nextDueStr.String != "" {
			if t, err := parseFlexTime(nextDueStr.String); err == nil {
				sum.NextDue = &t
			}
		}
		summaries = append(summaries, sum)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error reading deck summaries: %w", err)
	}

	return summaries, nil
}

// GetStatsSummary gathers comprehensive review metrics across the library.
func (s *Store) GetStatsSummary(now time.Time) (*StatsSummary, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}

	stats := &StatsSummary{
		ReviewsByDay: make(map[string]int),
		RatingsCount: make(map[gofsrs.Rating]int),
	}

	// 1. Deck summaries
	decks, err := s.GetDeckSummaries(now)
	if err != nil {
		return nil, err
	}
	stats.DeckBreakdown = decks
	stats.TotalDecks = len(decks)

	for _, d := range decks {
		stats.TotalCards += d.TotalCards
		stats.DueToday += d.DueCards
		stats.NewToday += d.NewCards
	}

	// 2. Review counts and retention rate
	reviewRows, err := s.db.Query(`
		SELECT rating, COUNT(*) 
		FROM review_logs 
		GROUP BY rating
	`)
	if err != nil {
		return nil, err
	}
	defer reviewRows.Close()

	var successfulReviews int
	for reviewRows.Next() {
		var r int
		var count int
		if err := reviewRows.Scan(&r, &count); err != nil {
			return nil, err
		}
		rating := gofsrs.Rating(r)
		stats.RatingsCount[rating] = count
		stats.TotalReviews += count
		if rating == gofsrs.Good || rating == gofsrs.Easy {
			successfulReviews += count
		}
	}
	if err := reviewRows.Err(); err != nil {
		return nil, fmt.Errorf("error reading review counts: %w", err)
	}
	reviewRows.Close()

	if stats.TotalReviews > 0 {
		stats.RetentionRate = float64(successfulReviews) / float64(stats.TotalReviews) * 100.0
	}

	// 3. Reviews by day (last 30 days)
	dayRows, err := s.db.Query(`
		SELECT strftime('%Y-%m-%d', review_time) as day, COUNT(*) 
		FROM review_logs 
		WHERE review_time >= datetime('now', '-30 days')
		GROUP BY day 
		ORDER BY day ASC
	`)
	if err != nil {
		return nil, err
	}
	defer dayRows.Close()

	for dayRows.Next() {
		var day sql.NullString
		var count int
		if err := dayRows.Scan(&day, &count); err != nil {
			return nil, err
		}
		if day.Valid && day.String != "" {
			stats.ReviewsByDay[day.String] = count
		}
	}
	if err := dayRows.Err(); err != nil {
		return nil, fmt.Errorf("error reading daily reviews: %w", err)
	}
	dayRows.Close()

	// 4. Calculate current daily streak
	streakRows, err := s.db.Query(`
		SELECT DISTINCT strftime('%Y-%m-%d', review_time) as day 
		FROM review_logs 
		ORDER BY day DESC
	`)
	if err != nil {
		return nil, err
	}
	defer streakRows.Close()

	var days []string
	for streakRows.Next() {
		var d sql.NullString
		if err := streakRows.Scan(&d); err != nil {
			return nil, err
		}
		if d.Valid && d.String != "" {
			days = append(days, d.String)
		}
	}
	if err := streakRows.Err(); err != nil {
		return nil, fmt.Errorf("error reading review streak: %w", err)
	}
	streakRows.Close()

	stats.CurrentStreak = calculateStreak(days, now)

	return stats, nil
}

func calculateStreak(days []string, now time.Time) int {
	if len(days) == 0 {
		return 0
	}

	daySet := make(map[string]bool)
	for _, d := range days {
		daySet[d] = true
	}

	todayStr := now.Format("2006-01-02")
	yesterdayStr := now.AddDate(0, 0, -1).Format("2006-01-02")

	// Streak is active if user reviewed today or yesterday
	var current time.Time
	if daySet[todayStr] {
		current = now
	} else if daySet[yesterdayStr] {
		current = now.AddDate(0, 0, -1)
	} else {
		return 0
	}

	streak := 0
	for {
		dateStr := current.Format("2006-01-02")
		if daySet[dateStr] {
			streak++
			current = current.AddDate(0, 0, -1)
		} else {
			break
		}
	}

	return streak
}

// ResetProgress resets all FSRS states and clears review logs.
func (s *Store) ResetProgress() error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec("DELETE FROM review_logs"); err != nil {
		return err
	}

	initial := card.NewDefaultFSRSState()
	_, err = tx.Exec(`
		UPDATE card_fsrs 
		SET due = ?, stability = ?, difficulty = ?, elapsed_days = ?, scheduled_days = ?, reps = 0, lapses = 0, state = ?, last_review = ?
	`, initial.Due, initial.Stability, initial.Difficulty, initial.ElapsedDays, initial.ScheduledDays, int(initial.State), initial.LastReview)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func parseFlexTime(s string) (time.Time, error) {
	formats := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02 15:04:05.999999999",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05",
		"2006-01-02",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, s); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("unable to parse time string: %s", s)
}

func scanCards(rows *sql.Rows) ([]*card.Card, error) {
	var cards []*card.Card

	for rows.Next() {
		var c card.Card
		var tagsStr, extraStr sql.NullString
		var fsrsDueStr, fsrsLastReviewStr sql.NullString
		var fsrsStability, fsrsDifficulty float64
		var fsrsElapsed, fsrsScheduled, fsrsReps, fsrsLapses uint64
		var fsrsState int

		err := rows.Scan(
			&c.ID, &c.Hash, &c.Deck, &c.FilePath, &c.LineNumber, &c.Type, &c.Prompt, &c.Answer, &extraStr, &tagsStr, &c.CreatedAt, &c.UpdatedAt,
			&fsrsDueStr, &fsrsStability, &fsrsDifficulty, &fsrsElapsed, &fsrsScheduled, &fsrsReps, &fsrsLapses, &fsrsState, &fsrsLastReviewStr,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan card row: %w", err)
		}

		if tagsStr.Valid && tagsStr.String != "" {
			c.Tags = strings.Split(tagsStr.String, ",")
		}
		if extraStr.Valid {
			c.Extra = extraStr.String
		}

		c.FSRS = &card.FSRSState{
			Stability:     fsrsStability,
			Difficulty:    fsrsDifficulty,
			ElapsedDays:   fsrsElapsed,
			ScheduledDays: fsrsScheduled,
			Reps:          fsrsReps,
			Lapses:        fsrsLapses,
			State:         gofsrs.State(fsrsState),
		}
		if fsrsDueStr.Valid && fsrsDueStr.String != "" {
			if t, err := parseFlexTime(fsrsDueStr.String); err == nil {
				c.FSRS.Due = t
			}
		}
		if fsrsLastReviewStr.Valid && fsrsLastReviewStr.String != "" {
			if t, err := parseFlexTime(fsrsLastReviewStr.String); err == nil {
				c.FSRS.LastReview = t
			}
		}

		cards = append(cards, &c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error reading card rows: %w", err)
	}

	return cards, nil
}

// CleanOrphanFiles removes database records for files that no longer exist on disk.
func (s *Store) CleanOrphanFiles() (int, error) {
	rows, err := s.db.Query("SELECT DISTINCT file_path FROM cards")
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	var toDelete []string
	for rows.Next() {
		var fp string
		if err := rows.Scan(&fp); err == nil {
			if _, err := os.Stat(fp); os.IsNotExist(err) {
				toDelete = append(toDelete, fp)
			}
		}
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	rows.Close()

	deleted := 0
	for _, fp := range toDelete {
		res, err := s.db.Exec("DELETE FROM cards WHERE file_path = ?", fp)
		if err == nil {
			if n, _ := res.RowsAffected(); n > 0 {
				deleted += int(n)
			}
		}
		_, _ = s.db.Exec("DELETE FROM sync_files WHERE file_path = ? OR LOWER(file_path) = LOWER(?)", fp, fp)
	}

	_ = deduplicateCards(s.db)
	return deleted, nil
}

// GetSyncedFiles retrieves metadata for all previously synced markdown files.
func (s *Store) GetSyncedFiles() (map[string]*SyncedFileInfo, error) {
	rows, err := s.db.Query("SELECT file_path, last_modified, card_count, content_hash FROM sync_files")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]*SyncedFileInfo)
	for rows.Next() {
		var sf SyncedFileInfo
		var lastModStr string
		if err := rows.Scan(&sf.FilePath, &lastModStr, &sf.CardCount, &sf.ContentHash); err != nil {
			return nil, err
		}
		if t, err := parseFlexTime(lastModStr); err == nil {
			sf.LastModified = t
		}
		result[sf.FilePath] = &sf
		result[strings.ToLower(sf.FilePath)] = &sf
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}
