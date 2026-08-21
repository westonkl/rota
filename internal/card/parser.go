package card

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var (
	tagRegex        = regexp.MustCompile(`(^|\s)#([a-zA-Z0-9_\-/]+)`)
	explicitIDRegex = regexp.MustCompile(`<!--\s*(?:id:|rota:id=)\s*([a-zA-Z0-9_\-]+)\s*-->`)
	frontmatterSep  = regexp.MustCompile(`^---[ \t]*$`)
	dividerRegex    = regexp.MustCompile(`^(?:---+|\*\*\*+|===+)[ \t]*$`)
)

// ParseFile parses a markdown file and extracts all flashcards.
func ParseFile(filePath string) ([]*Card, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", filePath, err)
	}

	defaultDeck := deriveDeckFromPath(filePath)
	return ParseContent(data, filePath, defaultDeck)
}

// ParseContent parses markdown content from a byte slice.
func ParseContent(content []byte, filePath, defaultDeck string) ([]*Card, error) {
	scanner := bufio.NewScanner(bytes.NewReader(content))
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	var cards []*Card
	currentDeck := defaultDeck
	lineIdx := 0

	// 1. Check for YAML frontmatter at beginning of file
	if len(lines) > 0 && frontmatterSep.MatchString(lines[0]) {
		endIdx := -1
		for i := 1; i < len(lines); i++ {
			if frontmatterSep.MatchString(lines[i]) {
				endIdx = i
				break
			}
		}
		if endIdx != -1 {
			// Extract deck or tags from frontmatter
			for i := 1; i < endIdx; i++ {
				line := strings.TrimSpace(lines[i])
				lower := strings.ToLower(line)
				if strings.HasPrefix(lower, "deck:") || strings.HasPrefix(lower, "deck :") {
					colonIdx := strings.Index(line, ":")
					val := strings.TrimSpace(line[colonIdx+1:])
					val = strings.Trim(val, `"'`)
					if val != "" {
						currentDeck = val
					}
				}
			}
			lineIdx = endIdx + 1
		}
	}

	// 2. State machine to parse Q:/A: and C: blocks
	type parseState int
	const (
		stateNone parseState = iota
		stateQuestion
		stateAnswer
		stateCloze
	)

	state := stateNone
	var qLines []string
	var aLines []string
	var cLines []string
	var cardStartLine int
	var explicitID string

	cardCountForHash := make(map[string]int)

	flushCard := func(endLine int) {
		now := time.Now().UTC()
		if state == stateAnswer && len(qLines) > 0 && len(aLines) > 0 {
			qRaw := strings.TrimSpace(strings.Join(qLines, "\n"))
			aRaw := strings.TrimSpace(strings.Join(aLines, "\n"))
			if qRaw != "" && aRaw != "" {
				tags := extractTags(qRaw + "\n" + aRaw)
				qClean := stripTags(qRaw)
				aClean := stripTags(aRaw)
				hash := ComputeMeaningHash(qClean, aClean)
				cardCountForHash[hash]++
				count := cardCountForHash[hash]

				cardID := explicitID
				if cardID == "" {
					if count > 1 {
						cardID = fmt.Sprintf("%s_%d", hash, count)
					} else {
						cardID = hash
					}
				}

				cards = append(cards, &Card{
					ID:         cardID,
					Hash:       hash,
					Deck:       currentDeck,
					FilePath:   filePath,
					LineNumber: cardStartLine,
					Type:       TypeQA,
					Prompt:     qClean,
					Answer:     aClean,
					Tags:       tags,
					CreatedAt:  now,
					UpdatedAt:  now,
				})
			}
		} else if state == stateCloze && len(cLines) > 0 {
			cRaw := strings.TrimSpace(strings.Join(cLines, "\n"))
			if cRaw != "" {
				tags := extractTags(cRaw)
				cClean := stripTags(cRaw)
				clozes := ProcessClozeText(cClean)
				for idx, cl := range clozes {
					hash := ComputeMeaningHash(cl.FrontText, cl.BackText)
					cardCountForHash[hash]++
					count := cardCountForHash[hash]

					cardID := explicitID
					if cardID == "" {
						if len(clozes) > 1 || count > 1 {
							cardID = fmt.Sprintf("%s_%d_%d", hash, idx, count)
						} else {
							cardID = hash
						}
					}

					cards = append(cards, &Card{
						ID:         cardID,
						Hash:       hash,
						Deck:       currentDeck,
						FilePath:   filePath,
						LineNumber: cardStartLine,
						Type:       TypeCloze,
						Prompt:     cl.FrontText,
						Answer:     cl.BackText,
						Extra:      cl.Answer,
						Tags:       tags,
						CreatedAt:  now,
						UpdatedAt:  now,
					})
				}
			}
		}

		// Reset accumulators
		qLines = nil
		aLines = nil
		cLines = nil
		explicitID = ""
		state = stateNone
	}

	inTopLevelCode := false
	for i := lineIdx; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)
		lineNum := i + 1

		// Check for markdown code fences at top level
		if strings.HasPrefix(trimmed, "```") {
			if state == stateNone {
				inTopLevelCode = !inTopLevelCode
				continue
			}
		}

		if inTopLevelCode {
			continue
		}

		// Check for explicit ID comment
		if idMatch := explicitIDRegex.FindStringSubmatch(trimmed); len(idMatch) > 1 {
			explicitID = idMatch[1]
			continue
		}

		// Check for horizontal divider
		if dividerRegex.MatchString(trimmed) {
			flushCard(lineNum)
			continue
		}

		// Check for Q: prefix (case-insensitive for convenience)
		if strings.HasPrefix(trimmed, "Q:") || strings.HasPrefix(trimmed, "q:") {
			flushCard(lineNum)
			state = stateQuestion
			cardStartLine = lineNum
			qText := strings.TrimSpace(line[2:])
			if qText != "" {
				qLines = append(qLines, qText)
			}
			continue
		}

		// Check for A: prefix
		if (strings.HasPrefix(trimmed, "A:") || strings.HasPrefix(trimmed, "a:")) && (state == stateQuestion || state == stateAnswer) {
			state = stateAnswer
			aText := strings.TrimSpace(line[2:])
			if aText != "" {
				aLines = append(aLines, aText)
			}
			continue
		}

		// Check for C: prefix (Cloze)
		if strings.HasPrefix(trimmed, "C:") || strings.HasPrefix(trimmed, "c:") {
			flushCard(lineNum)
			state = stateCloze
			cardStartLine = lineNum
			cText := strings.TrimSpace(line[2:])
			if cText != "" {
				cLines = append(cLines, cText)
			}
			continue
		}

		// Continue appending multi-line card content
		switch state {
		case stateQuestion:
			qLines = append(qLines, line)
		case stateAnswer:
			// If we hit an empty line followed by markdown header or another card, we keep collecting or flush when next card starts
			aLines = append(aLines, line)
		case stateCloze:
			cLines = append(cLines, line)
		}
	}

	// Flush trailing card
	flushCard(len(lines))

	return cards, nil
}

// deriveDeckFromPath creates a clean deck name based on the file and parent directory.
func deriveDeckFromPath(filePath string) string {
	clean := filepath.Clean(filePath)
	base := filepath.Base(clean)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)

	dir := filepath.Base(filepath.Dir(clean))
	if dir != "." && dir != "/" && dir != "" && dir != "notes" && dir != "decks" && dir != "cards" {
		return fmt.Sprintf("%s/%s", dir, name)
	}
	return name
}

// extractTags pulls `#tag` identifiers out of text.
func extractTags(text string) []string {
	matches := tagRegex.FindAllStringSubmatch(text, -1)
	if len(matches) == 0 {
		return nil
	}
	tagSet := make(map[string]struct{})
	for _, m := range matches {
		if len(m) > 2 {
			tag := strings.ToLower(m[2])
			// Avoid markdown headers like # or ###
			if !strings.HasPrefix(tag, "#") && len(tag) > 0 {
				tagSet[tag] = struct{}{}
			}
		}
	}
	var tags []string
	for tag := range tagSet {
		tags = append(tags, tag)
	}
	return tags
}

// stripTags removes `#tag` annotations from display text while preserving markdown formatting.
func stripTags(text string) string {
	cleaned := tagRegex.ReplaceAllString(text, "")
	return strings.TrimSpace(cleaned)
}

// WriteCardToWriter writes a Q/A or Cloze card formatted in clean markdown.
func WriteCardToWriter(w io.Writer, cardType CardType, prompt, answer string, tags []string) error {
	var tagStr string
	if len(tags) > 0 {
		var formattedTags []string
		for _, t := range tags {
			t = strings.TrimPrefix(t, "#")
			formattedTags = append(formattedTags, "#"+t)
		}
		tagStr = " " + strings.Join(formattedTags, " ")
	}

	if cardType == TypeCloze {
		_, err := fmt.Fprintf(w, "\nC: %s%s\n", prompt, tagStr)
		return err
	}

	_, err := fmt.Fprintf(w, "\nQ: %s%s\nA: %s\n", prompt, tagStr, answer)
	return err
}
