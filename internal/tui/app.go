package tui

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	gofsrs "github.com/open-spaced-repetition/go-fsrs/v4"

	"rota/internal/card"
	"rota/internal/db"
	"rota/internal/fsrs"
)

type SessionState int

const (
	StateReviewing SessionState = iota
	StateFinished
	StateEmpty
)

type ReviewedItem struct {
	Card      *card.Card
	Rating    gofsrs.Rating
	PrevFSRS  card.FSRSState
	NextFSRS  card.FSRSState
	Timestamp time.Time
}

// AppModel represents the Bubble Tea state machine for a review session.
type AppModel struct {
	store     *db.Store
	scheduler *fsrs.Scheduler

	cards     []*card.Card
	currIndex int
	revealed  bool

	history []ReviewedItem

	// Session Metrics
	startTime  time.Time
	againCount int
	hardCount  int
	goodCount  int
	easyCount  int

	state  SessionState
	width  int
	height int

	statusMessage string
}

// NewAppModel creates and initializes a review session model.
func NewAppModel(store *db.Store, scheduler *fsrs.Scheduler, cards []*card.Card) *AppModel {
	state := StateReviewing
	if len(cards) == 0 {
		state = StateEmpty
	}

	return &AppModel{
		store:     store,
		scheduler: scheduler,
		cards:     cards,
		currIndex: 0,
		revealed:  false,
		state:     state,
		startTime: time.Now(),
		width:     80,
		height:    24,
	}
}

func (m *AppModel) Init() tea.Cmd {
	return nil
}

func (m *AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		if msg.Width > 0 {
			m.width = msg.Width
		}
		if msg.Height > 0 {
			m.height = msg.Height
		}
		return m, nil

	case tea.KeyMsg:
		key := msg.String()

		// 1. Quit handlers
		if msg.Type == tea.KeyCtrlC || msg.Type == tea.KeyEsc || key == "q" || key == "Q" || key == "esc" {
			return m, tea.Quit
		}

		// 2. Space / Enter to reveal card or advance
		if msg.Type == tea.KeySpace || key == " " || key == "space" {
			if m.state == StateReviewing && !m.revealed {
				m.revealed = true
				m.statusMessage = ""
				return m, nil
			}
		}

		if msg.Type == tea.KeyEnter || key == "enter" || key == "\r" || key == "\n" {
			if m.state == StateReviewing {
				if !m.revealed {
					m.revealed = true
					m.statusMessage = ""
					return m, nil
				}
				// Default to Good (3) on Enter if already revealed
				return m.handleRating(gofsrs.Good)
			}
			if m.state == StateFinished || m.state == StateEmpty {
				return m, tea.Quit
			}
		}

		// 3. Grading (when card is revealed)
		if m.state == StateReviewing && m.revealed {
			switch {
			case key == "1" || key == "a" || key == "A" || msg.Type == tea.KeyLeft:
				return m.handleRating(gofsrs.Again)
			case key == "2" || key == "h" || key == "H" || msg.Type == tea.KeyDown:
				return m.handleRating(gofsrs.Hard)
			case key == "3" || key == "g" || key == "G" || msg.Type == tea.KeyUp:
				return m.handleRating(gofsrs.Good)
			case key == "4" || key == "e" || key == "E" || key == "d" || key == "D" || msg.Type == tea.KeyRight:
				return m.handleRating(gofsrs.Easy)
			}
		}

		// 4. Undo
		if (key == "u" || key == "U") && m.state == StateReviewing {
			return m.handleUndo()
		}

		// 5. Open in editor
		if (key == "o" || key == "O") && m.state == StateReviewing && len(m.cards) > 0 && m.currIndex < len(m.cards) {
			return m, m.openInEditorCmd()
		}
	}

	return m, nil
}

func (m *AppModel) handleRating(rating gofsrs.Rating) (tea.Model, tea.Cmd) {
	if m.currIndex >= len(m.cards) {
		return m, nil
	}

	curr := m.cards[m.currIndex]
	now := time.Now().UTC()

	var fsrsCard gofsrs.Card
	if curr.FSRS != nil {
		fsrsCard = curr.FSRS.ToFSRSCard()
	} else {
		fsrsCard = gofsrs.NewCard()
	}

	nextCard, _ := m.scheduler.Next(fsrsCard, now, rating)
	nextState := card.FromFSRSCard(nextCard)

	// Save to DB
	if err := m.store.RecordReview(curr.ID, rating, nextState, now); err != nil {
		m.statusMessage = fmt.Sprintf("Error saving review: %v", err)
	}

	// Update counts
	switch rating {
	case gofsrs.Again:
		m.againCount++
		// If "Again", re-queue card at end of session so user masters it today!
		m.cards = append(m.cards, curr)
	case gofsrs.Hard:
		m.hardCount++
	case gofsrs.Good:
		m.goodCount++
	case gofsrs.Easy:
		m.easyCount++
	}

	// Push to undo history
	prev := card.NewDefaultFSRSState()
	if curr.FSRS != nil {
		prev = curr.FSRS
	}
	m.history = append(m.history, ReviewedItem{
		Card:      curr,
		Rating:    rating,
		PrevFSRS:  *prev,
		NextFSRS:  *nextState,
		Timestamp: now,
	})

	// Advance index
	curr.FSRS = nextState
	m.currIndex++
	m.revealed = false
	m.statusMessage = ""

	if m.currIndex >= len(m.cards) {
		m.state = StateFinished
	}

	return m, nil
}

func (m *AppModel) handleUndo() (tea.Model, tea.Cmd) {
	if len(m.history) == 0 || m.currIndex == 0 {
		m.statusMessage = "Nothing to undo"
		return m, nil
	}

	last := m.history[len(m.history)-1]
	m.history = m.history[:len(m.history)-1]

	// Revert in DB
	if err := m.store.UndoLastReview(last.Card.ID); err != nil {
		m.statusMessage = fmt.Sprintf("Undo error: %v", err)
		return m, nil
	}

	// Decrement count
	switch last.Rating {
	case gofsrs.Again:
		m.againCount--
		// If it was again and added to end of slice, pop it
		if len(m.cards) > 0 && m.cards[len(m.cards)-1].ID == last.Card.ID {
			m.cards = m.cards[:len(m.cards)-1]
		}
	case gofsrs.Hard:
		m.hardCount--
	case gofsrs.Good:
		m.goodCount--
	case gofsrs.Easy:
		m.easyCount--
	}

	m.currIndex--
	m.cards[m.currIndex].FSRS = &last.PrevFSRS
	m.revealed = true
	m.statusMessage = fmt.Sprintf("Undid rating for card")

	return m, nil
}

func (m *AppModel) openInEditorCmd() tea.Cmd {
	curr := m.cards[m.currIndex]
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "nano"
	}

	var args []string
	if strings.Contains(editor, "vi") || strings.Contains(editor, "nano") {
		args = []string{fmt.Sprintf("+%d", curr.LineNumber), curr.FilePath}
	} else if strings.Contains(editor, "code") {
		args = []string{"--goto", fmt.Sprintf("%s:%d", curr.FilePath, curr.LineNumber)}
	} else {
		args = []string{curr.FilePath}
	}

	c := exec.Command(editor, args...)
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return nil
	})
}

func (m *AppModel) View() string {
	switch m.state {
	case StateEmpty:
		return m.viewEmpty()
	case StateFinished:
		return m.viewFinished()
	case StateReviewing:
		return m.viewReviewing()
	default:
		return ""
	}
}

func (m *AppModel) viewEmpty() string {
	boxWidth := min(m.width-4, 60)
	content := lipgloss.JoinVertical(
		lipgloss.Center,
		"",
		StyleTitle.Render("✦ All Caught Up! ✦"),
		"",
		lipgloss.NewStyle().Foreground(ColorSubtle).Render("No cards are currently due for review."),
		"",
		lipgloss.NewStyle().Foreground(ColorMuted).Render("Add more flashcards to your markdown notes or check back later!"),
		"",
		StyleHelp.Render("Press [q] or [Enter] to exit"),
		"",
	)

	return lipgloss.Place(
		m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		StyleCardBox.Width(boxWidth).Render(content),
	)
}

func (m *AppModel) viewFinished() string {
	boxWidth := min(m.width-4, 70)
	duration := time.Since(m.startTime).Round(time.Second)

	totalReviewed := m.againCount + m.hardCount + m.goodCount + m.easyCount
	var accuracy float64
	if totalReviewed > 0 {
		accuracy = float64(m.goodCount+m.easyCount) / float64(totalReviewed) * 100.0
	}

	header := lipgloss.JoinVertical(
		lipgloss.Center,
		StyleTitle.Render("✦ Session Completed! ✦"),
		lipgloss.NewStyle().Foreground(ColorSubtle).Render(fmt.Sprintf("Studied %d cards in %s", totalReviewed, duration)),
	)

	statsGrid := lipgloss.JoinVertical(
		lipgloss.Left,
		fmt.Sprintf("  %s %-12s : %d", StyleBtnAgain.Render("● Again"), "(lapses)", m.againCount),
		fmt.Sprintf("  %s %-12s : %d", StyleBtnHard.Render("● Hard"), "(difficult)", m.hardCount),
		fmt.Sprintf("  %s %-12s : %d", StyleBtnGood.Render("● Good"), "(passed)", m.goodCount),
		fmt.Sprintf("  %s %-12s : %d", StyleBtnEasy.Render("● Easy"), "(mastered)", m.easyCount),
		"",
		fmt.Sprintf("  %s : %.1f%%", StyleStreak.Render("Retention Rate"), accuracy),
	)

	footer := lipgloss.JoinVertical(
		lipgloss.Center,
		"",
		StyleHelp.Render("Press [q] or [Enter] to exit"),
	)

	content := lipgloss.JoinVertical(
		lipgloss.Center,
		"",
		header,
		"",
		statsGrid,
		footer,
		"",
	)

	return lipgloss.Place(
		m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		StyleSummaryCard.Width(boxWidth).Render(content),
	)
}

func (m *AppModel) viewReviewing() string {
	if m.currIndex >= len(m.cards) {
		return ""
	}

	curr := m.cards[m.currIndex]
	boxWidth := min(m.width-4, 80)

	// Top Bar: Deck Badge | Tags | Progress [███░░░] 3/10 | State Badge
	deckBadge := StyleDeckBadge.Render(curr.Deck)
	var tagPills string
	if len(curr.Tags) > 0 {
		var tagLabels []string
		for _, t := range curr.Tags {
			tagLabels = append(tagLabels, lipgloss.NewStyle().Foreground(ColorSubtle).Render("#"+t))
		}
		tagPills = "  " + strings.Join(tagLabels, " ")
	}

	progressText := fmt.Sprintf("%d/%d", m.currIndex+1, len(m.cards))
	bar := ProgressBar(m.currIndex+1, len(m.cards), 12)

	stateName := "New"
	stateStyle := StyleCountNew
	if curr.FSRS != nil {
		switch curr.FSRS.State {
		case gofsrs.Learning, gofsrs.Relearning:
			stateName = "Learn"
			stateStyle = StyleCountLearn
		case gofsrs.Review:
			stateName = "Review"
			stateStyle = StyleCountReview
		}
	}
	stateBadge := stateStyle.Render(fmt.Sprintf("● %s", stateName))

	topBar := lipgloss.JoinHorizontal(
		lipgloss.Center,
		deckBadge,
		tagPills,
		"  ",
		bar,
		" ",
		StyleProgress.Render(progressText),
		"  •  ",
		stateBadge,
	)

	// Front / Question
	promptRendered := RenderMarkdown(curr.Prompt, boxWidth)
	if promptRendered == "" {
		promptRendered = curr.Prompt
	}

	var cardBody string
	if !m.revealed {
		// Front only
		cardBody = lipgloss.JoinVertical(
			lipgloss.Left,
			promptRendered,
			"",
			StyleDivider.Render(strings.Repeat("─", boxWidth-6)),
			"",
			lipgloss.NewStyle().Foreground(ColorMuted).Italic(true).Render("  Press [Space] or [Enter] to reveal answer"),
		)
	} else {
		// Front + Answer Revealed
		var answerRendered string
		if curr.Type == card.TypeCloze {
			answerRendered = RenderClozeBack(curr.Answer, boxWidth)
		} else {
			answerRendered = RenderMarkdown(curr.Answer, boxWidth)
		}

		now := time.Now().UTC()
		fsrsCard := gofsrs.NewCard()
		if curr.FSRS != nil {
			fsrsCard = curr.FSRS.ToFSRSCard()
		}
		intervals := m.scheduler.PreviewIntervals(fsrsCard, now)

		// Grade buttons with intervals
		btn1 := fmt.Sprintf("%s %s", StyleBtnAgain.Render("[1] Again"), StyleInterval.Render("("+intervals[gofsrs.Again]+")"))
		btn2 := fmt.Sprintf("%s %s", StyleBtnHard.Render("[2] Hard"), StyleInterval.Render("("+intervals[gofsrs.Hard]+")"))
		btn3 := fmt.Sprintf("%s %s", StyleBtnGood.Render("[3] Good"), StyleInterval.Render("("+intervals[gofsrs.Good]+")"))
		btn4 := fmt.Sprintf("%s %s", StyleBtnEasy.Render("[4] Easy"), StyleInterval.Render("("+intervals[gofsrs.Easy]+")"))

		gradeRow := lipgloss.JoinHorizontal(
			lipgloss.Center,
			btn1, "   ",
			btn2, "   ",
			btn3, "   ",
			btn4,
		)

		cardBody = lipgloss.JoinVertical(
			lipgloss.Left,
			promptRendered,
			"",
			StyleDivider.Render(strings.Repeat("─", boxWidth-6)),
			"",
			StyleAnswerHeader.Render("Answer:"),
			answerRendered,
			"",
			StyleDivider.Render(strings.Repeat("─", boxWidth-6)),
			"",
			gradeRow,
		)
	}

	// Footer Help
	footerHelp := lipgloss.JoinHorizontal(
		lipgloss.Center,
		StyleHelpKey.Render("[1-4 / a,h,g,e]"), StyleHelp.Render(" Grade   "),
		StyleHelpKey.Render("[u]"), StyleHelp.Render(" Undo   "),
		StyleHelpKey.Render("[o]"), StyleHelp.Render(" Open in Editor   "),
		StyleHelpKey.Render("[q]"), StyleHelp.Render(" Exit"),
	)

	if m.statusMessage != "" {
		footerHelp = lipgloss.NewStyle().Foreground(ColorWarning).Render(m.statusMessage)
	}

	mainCard := StyleCardBox.Width(boxWidth).Render(
		lipgloss.JoinVertical(
			lipgloss.Left,
			topBar,
			"",
			cardBody,
		),
	)

	return lipgloss.Place(
		m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		lipgloss.JoinVertical(
			lipgloss.Center,
			mainCard,
			"",
			footerHelp,
		),
	)
}
