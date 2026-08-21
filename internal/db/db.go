package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

const schema = `
CREATE TABLE IF NOT EXISTS cards (
    id TEXT PRIMARY KEY,
    hash TEXT UNIQUE NOT NULL,
    deck TEXT NOT NULL,
    file_path TEXT NOT NULL,
    line_number INTEGER NOT NULL,
    card_type TEXT NOT NULL,
    prompt TEXT NOT NULL,
    answer TEXT NOT NULL,
    extra TEXT,
    tags TEXT,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_cards_deck ON cards(deck);
CREATE INDEX IF NOT EXISTS idx_cards_deck_nocase ON cards(deck COLLATE NOCASE);
CREATE INDEX IF NOT EXISTS idx_cards_file_path ON cards(file_path);
CREATE INDEX IF NOT EXISTS idx_cards_hash ON cards(hash);

CREATE TABLE IF NOT EXISTS card_fsrs (
    card_id TEXT PRIMARY KEY,
    due DATETIME NOT NULL,
    stability REAL NOT NULL,
    difficulty REAL NOT NULL,
    elapsed_days INTEGER NOT NULL,
    scheduled_days INTEGER NOT NULL,
    reps INTEGER NOT NULL,
    lapses INTEGER NOT NULL,
    state INTEGER NOT NULL,
    last_review DATETIME,
    FOREIGN KEY(card_id) REFERENCES cards(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_card_fsrs_due_state ON card_fsrs(due, state);
CREATE INDEX IF NOT EXISTS idx_card_fsrs_state ON card_fsrs(state);

CREATE TABLE IF NOT EXISTS review_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    card_id TEXT NOT NULL,
    rating INTEGER NOT NULL,
    state INTEGER NOT NULL,
    due DATETIME NOT NULL,
    stability REAL NOT NULL,
    difficulty REAL NOT NULL,
    elapsed_days INTEGER NOT NULL,
    last_elapsed INTEGER NOT NULL,
    scheduled_days INTEGER NOT NULL,
    review_time DATETIME NOT NULL,
    FOREIGN KEY(card_id) REFERENCES cards(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_review_logs_card ON review_logs(card_id);
CREATE INDEX IF NOT EXISTS idx_review_logs_time ON review_logs(review_time);

CREATE TABLE IF NOT EXISTS sync_files (
    file_path TEXT PRIMARY KEY,
    last_modified DATETIME NOT NULL,
    card_count INTEGER NOT NULL,
    content_hash TEXT NOT NULL
);
`

// Store handles SQLite persistence for rota.
type Store struct {
	db *sql.DB
}

// Open initializes or connects to the SQLite database.
func Open(dbPath string) (*Store, error) {
	if dbPath != ":memory:" {
		dir := filepath.Dir(dbPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create database directory %s: %w", dir, err)
		}
	}

	// Open connection with pragma settings
	dsn := fmt.Sprintf("%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)", dbPath)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database %s: %w", dbPath, err)
	}

	// Configure pool
	db.SetMaxOpenConns(1) // Single writer for SQLite concurrency safety

	// Create tables if not present
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to execute database schema: %w", err)
	}

	return &Store{db: db}, nil
}

func deduplicateCards(db *sql.DB) error {
	_, err := db.Exec(`
		DELETE FROM cards WHERE id NOT IN (
			SELECT id FROM (
				SELECT id, ROW_NUMBER() OVER (PARTITION BY hash ORDER BY updated_at DESC, created_at DESC) as rn
				FROM cards
			) WHERE rn = 1
		);
	`)
	return err
}

// Close closes the database connection.
func (s *Store) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// DB returns the underlying sql.DB instance.
func (s *Store) DB() *sql.DB {
	return s.db
}
