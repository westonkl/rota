# rota

**rota** is a fast, lightweight terminal flashcard and spaced repetition tool written in Go. It turns your plain Markdown notes into interactive flashcards, schedules reviews using the modern **Free Spaced Repetition Scheduler (FSRS)** algorithm, and persists your learning progress in an embedded SQLite database.

Inspired by [shaankhosla/repeater](https://github.com/shaankhosla/repeater), `rota` is built with a **text-first** philosophy: your Markdown notes remain the single source of truth.

---

## ✨ Features

- 📝 **Markdown-First Workflow**: Flashcards live directly inside your notes. No proprietary card editors or separate sync databases required.
- 🧠 **FSRS Algorithm**: Powered by the state-of-the-art Free Spaced Repetition Scheduler (`go-fsrs/v4`) for optimized retention (~90% recall target).
- 🗄️ **Zero-Configuration SQLite**: Fast, pure-Go SQLite persistence with automatic indexing, transactions, and meaning-only content hashing.
- 🎨 **Terminal UI**: Interactive, responsive TUI built with [Charm Bubble Tea](https://github.com/charmbracelet/bubbletea), [Lip Gloss](https://github.com/charmbracelet/lipgloss), and syntax-highlighted Markdown rendering via [Glamour](https://github.com/charmbracelet/glamour).
- 🏷️ **Q/A & Cloze Deletions**: Seamless support for multi-line Question/Answer blocks, bracket clozes `[like this]`, and Anki-style clozes `{{c1::like this}}`.
- 📊 **Detailed Metrics**: Review streaks, 14-day activity sparklines, recall accuracy rates, and per-deck breakdown statistics.
- ⚡ **Auto-Sync & Linting**: Automatically detects modified/added cards when reviewing and includes a built-in syntax checker (`rota check`).

---

## 🚀 Quick Start

### Build & Install

```bash
# Clone the repository
git clone https://github.com/westonkl/rota.git
cd rota

# Build the binary
go build -o rota .

# (Optional) Install to your PATH
go install .
```

---

## 📖 Flashcard Markdown Syntax

Flashcards can be embedded anywhere within your Markdown notes. Non-card text (headers, regular prose) is preserved and ignored.

### 1. Basic Question & Answer (Q/A)

Use `Q:` for the question and `A:` for the answer. Cards can be separated by horizontal rules (`---`):

```markdown
---
deck: "Go Concurrency"
---

Q: What is a Goroutine in Go? #go #concurrency
A: A goroutine is a lightweight thread of execution managed independently by the Go runtime scheduler.

---

Q: How do you read from a closed Go channel?
A: Reading from a closed channel returns the zero value and false:
` + "```go" + `
val, ok := <-ch
` + "```" + `
```

### 2. Cloze Deletion Cards

Use `C:` followed by brackets `[...]` or Anki-style `{{c1::...}}` around the words you want to test:

```markdown
C: In Go, communication over an unbuffered channel is [synchronous] and blocks. #channels

C: Speech is [produced] in [Broca's] area. #neuroscience
```

- During review, the card front displays `[...]` placeholders.
- When revealed, the answer words are highlighted.

### 3. Decks & Metadata

- **Frontmatter Deck**: Set `deck: "Computer Science"` at the top of the file.
- **Directory/Filename Deck**: If frontmatter is omitted, `rota` automatically infers the deck name from the file and parent directory (e.g. `notes/algorithms/sorting.md` -> `algorithms/sorting`).
- **Tags**: Add `#tag-name` anywhere in the card to tag cards for easy categorization.

---

## 🛠️ CLI Commands

### 1. Start a Review Session (`rota drill`)

```bash
# Review all currently due cards across all notes
rota drill

# Review cards from a specific file or directory
rota drill ./notes/
rota drill examples/go_concurrency.md

# Filter reviews by deck name
rota drill -k "Go Concurrency"

# Limit session size (e.g., 20 cards)
rota drill -n 20

# Review in plain text mode (non-TUI / pipe friendly)
rota drill --plain

# To review all cards regardless of due date
rota drill --all
```

**TUI Keybindings:**
- `Space` / `Enter`: Reveal answer
- `1` / `a`: **Again** (lapse / reset interval)
- `2` / `h`: **Hard** (difficult recall)
- `3` / `g` / `Enter`: **Good** (correct recall)
- `4` / `e`: **Easy** (mastered)
- `u`: Undo previous rating
- `o`: Open source card in `$EDITOR` at exact line number
- `q` / `Esc`: Save progress & exit

---

### 2. Scan & Index Markdown Cards (`rota scan`)

Scans your notes, indexes new cards, updates modified lines, and removes deleted cards:

```bash
rota scan ./notes
```

---

### 3. View Statistics (`rota stats`)

Displays your daily review streak, recall rate, 14-day activity chart, and per-deck breakdown:

```bash
rota stats
```

---

### 4. List Flashcards (`rota list`)

Inspect indexed cards and due dates in a clean table:

```bash
# List all cards
rota list

# Filter by deck
rota list -k "Go Concurrency"

# Show only due cards
rota list --due

# Search card text
rota list -s "goroutine"
```

---

### 5. Add a Flashcard (`rota add`)

Quickly append a new flashcard to a Markdown file:

```bash
# Interactive mode
rota add

# Non-interactive CLI flags
rota add -f notes/go.md -q "What is sync.Once?" -a "Guarantees a function executes only once." -t "go,sync"
```

---

### 6. Lint Markdown Files (`rota check`)

Validates markdown files for orphan `Q:` prompts without answers or malformed clozes:

```bash
rota check ./notes
```

---

### 7. Reset Progress (`rota reset`)

Resets all FSRS scheduling progress and review logs back to the initial state:

```bash
rota reset
```

---

## ⚙️ Configuration & Flags

- `-d, --db <path>`: Custom SQLite database path (default: `~/.local/share/rota/rota.db` or `.rota/rota.db`).
- `-p, --path <path>`: Default vault/notes directory (default: `.`).
- `-r, --retention <float>`: Target retention rate for FSRS (default: `0.90`).

You can also set the `ROTA_DB` environment variable:
```bash
export ROTA_DB="$HOME/Dropbox/notes/rota.db"
```

---

## 🧪 Testing

Run the test suite:

```bash
go test -v ./...
```