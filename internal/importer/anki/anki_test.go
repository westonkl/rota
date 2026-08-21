package anki

import (
	"archive/zip"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
	"rota/internal/db"
)

func TestHTMLToMarkdown(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Basic tags",
			input:    "<b>Bold</b> and <i>Italic</i> and <code>code</code>",
			expected: "**Bold** and *Italic* and `code`",
		},
		{
			name:     "Line breaks and divs",
			input:    "Line 1<br>Line 2<div>Line 3</div>",
			expected: "Line 1\nLine 2\nLine 3",
		},
		{
			name:     "Code block",
			input:    "<pre><code>fmt.Println(\"Hello\")</code></pre>",
			expected: "```\nfmt.Println(\"Hello\")\n```",
		},
		{
			name:     "Strip image tag",
			input:    "Check this: <img src=\"diagram.png\"> and continue",
			expected: "Check this: and continue",
		},
		{
			name:     "HTML entities",
			input:    "Go &amp; Rust &lt;3 &quot;Fast&quot;",
			expected: "Go & Rust <3 \"Fast\"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := HTMLToMarkdown(tt.input)
			if actual != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, actual)
			}
		})
	}
}

func TestConvertAnkiClozeToRota(t *testing.T) {
	input := "Speech is produced in {{c1::Broca's area}} and understood in {{c2::Wernicke's area::hint}}."
	expected := "Speech is produced in [Broca's area] and understood in [Wernicke's area|hint]."

	actual := ConvertAnkiClozeToRota(input)
	if actual != expected {
		t.Errorf("expected %q, got %q", expected, actual)
	}
}

func TestImportSyntheticAPKG(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "anki_test_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	// 1. Create a synthetic Anki SQLite database
	dbPath := filepath.Join(tempDir, "collection.anki21")
	ankiDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}

	schema := `
	CREATE TABLE col (
		id INTEGER PRIMARY KEY,
		models TEXT,
		decks TEXT
	);
	CREATE TABLE notes (
		id INTEGER PRIMARY KEY,
		mid INTEGER,
		flds TEXT,
		tags TEXT
	);
	CREATE TABLE cards (
		id INTEGER PRIMARY KEY,
		nid INTEGER,
		did INTEGER,
		ord INTEGER,
		type INTEGER,
		queue INTEGER,
		due INTEGER,
		ivl INTEGER,
		factor INTEGER,
		reps INTEGER,
		lapses INTEGER
	);
	`
	if _, err := ankiDB.Exec(schema); err != nil {
		t.Fatal(err)
	}

	modelsJSON := `{"1":{"id":1,"name":"Basic","type":0,"flds":[{"name":"Front","ord":0},{"name":"Back","ord":1}],"tmpls":[{"name":"Card 1","ord":0,"qfmt":"{{Front}}","afmt":"{{Back}}"}]}}`
	decksJSON := `{"1":{"id":1,"name":"Go Concurrency"}}`
	if _, err := ankiDB.Exec("INSERT INTO col VALUES (1, ?, ?)", modelsJSON, decksJSON); err != nil {
		t.Fatal(err)
	}

	// Insert notes
	flds := "What is a Goroutine?\x1fA lightweight thread managed by Go runtime."
	if _, err := ankiDB.Exec("INSERT INTO notes VALUES (100, 1, ?, ?)", flds, "golang concurrency"); err != nil {
		t.Fatal(err)
	}
	if _, err := ankiDB.Exec("INSERT INTO cards VALUES (200, 100, 1, 0, 0, 0, 0, 0, 2500, 0, 0)"); err != nil {
		t.Fatal(err)
	}
	ankiDB.Close()

	// 2. Package database into synthetic .apkg zip file
	apkgPath := filepath.Join(tempDir, "test_deck.apkg")
	apkgFile, err := os.Create(apkgPath)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(apkgFile)

	dbData, err := os.ReadFile(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	w, err := zw.Create("collection.anki21")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(dbData); err != nil {
		t.Fatal(err)
	}

	// Add media map
	mw, err := zw.Create("media")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := mw.Write([]byte("{}")); err != nil {
		t.Fatal(err)
	}

	zw.Close()
	apkgFile.Close()

	// 3. Test Import with Rota Store
	store, err := db.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	importer := NewImporter(store)
	outDir := filepath.Join(tempDir, "output")

	result, err := importer.Import(ImportOptions{
		APKGPath:  apkgPath,
		OutputDir: outDir,
	})
	if err != nil {
		t.Fatalf("Import failed: %v", err)
	}

	if result.TotalCards != 1 {
		t.Errorf("expected 1 card, got %d", result.TotalCards)
	}
	if result.TotalDecks != 1 {
		t.Errorf("expected 1 deck, got %d", result.TotalDecks)
	}
	if len(result.GeneratedFiles) != 1 {
		t.Fatalf("expected 1 generated file, got %d", len(result.GeneratedFiles))
	}

	// Verify markdown file content
	content, err := os.ReadFile(result.GeneratedFiles[0])
	if err != nil {
		t.Fatal(err)
	}
	if len(content) == 0 {
		t.Error("generated markdown file is empty")
	}
}
