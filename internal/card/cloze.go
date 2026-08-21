package card

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	// Matches [text] or [text|hint]
	bracketClozeRegex = regexp.MustCompile(`\[([^\]]+)\]`)
	// Matches {{c1::text}} or {{c1::text::hint}}
	ankiClozeRegex = regexp.MustCompile(`\{\{c\d+::([^}]+?)(?:::([^}]+?))?\}\}`)
)

// ClozeInfo represents extracted cloze parts for a card.
type ClozeInfo struct {
	OriginalText string
	FrontText    string
	BackText     string
	Answer       string
	Hint         string
}

// ProcessClozeText converts cloze deletion syntax into front and back representations.
func ProcessClozeText(raw string) []ClozeInfo {
	raw = strings.TrimSpace(raw)
	// Check for Anki style first
	if ankiClozeRegex.MatchString(raw) {
		return processAnkiCloze(raw)
	}

	// Check for bracket style [word]
	if bracketClozeRegex.MatchString(raw) {
		return processBracketCloze(raw)
	}

	// Fallback if no brackets found
	return []ClozeInfo{
		{
			OriginalText: raw,
			FrontText:    raw,
			BackText:     raw,
			Answer:       raw,
		},
	}
}

func processBracketCloze(raw string) []ClozeInfo {
	matches := bracketClozeRegex.FindAllStringSubmatchIndex(raw, -1)
	if len(matches) == 0 {
		return nil
	}

	var results []ClozeInfo

	// If there's 1 or more bracket clozes, let's create a cloze card where all are tested or each is tested.
	// For minimalist smooth review, if multiple clozes exist in one sentence, masking them all or providing full front/back:
	var frontBuilder strings.Builder
	var backBuilder strings.Builder
	var answers []string

	lastIdx := 0
	for _, match := range matches {
		fullStart, fullEnd := match[0], match[1]
		contentStart, contentEnd := match[2], match[3]

		frontBuilder.WriteString(raw[lastIdx:fullStart])
		backBuilder.WriteString(raw[lastIdx:fullStart])

		content := raw[contentStart:contentEnd]
		hint := ""
		if parts := strings.SplitN(content, "|", 2); len(parts) == 2 {
			content = parts[0]
			hint = parts[1]
		}

		answers = append(answers, content)

		if hint != "" {
			frontBuilder.WriteString(fmt.Sprintf("[... (%s)]", hint))
		} else {
			frontBuilder.WriteString("[...]")
		}

		backBuilder.WriteString(fmt.Sprintf("**[%s]**", content))
		lastIdx = fullEnd
	}

	frontBuilder.WriteString(raw[lastIdx:])
	backBuilder.WriteString(raw[lastIdx:])

	results = append(results, ClozeInfo{
		OriginalText: raw,
		FrontText:    frontBuilder.String(),
		BackText:     backBuilder.String(),
		Answer:       strings.Join(answers, ", "),
	})

	return results
}

func processAnkiCloze(raw string) []ClozeInfo {
	matches := ankiClozeRegex.FindAllStringSubmatchIndex(raw, -1)
	if len(matches) == 0 {
		return nil
	}

	var results []ClozeInfo
	var frontBuilder strings.Builder
	var backBuilder strings.Builder
	var answers []string

	lastIdx := 0
	for _, match := range matches {
		fullStart, fullEnd := match[0], match[1]
		contentStart, contentEnd := match[2], match[3]

		frontBuilder.WriteString(raw[lastIdx:fullStart])
		backBuilder.WriteString(raw[lastIdx:fullStart])

		content := raw[contentStart:contentEnd]
		hint := ""
		if match[4] != -1 && match[5] != -1 {
			hint = raw[match[4]:match[5]]
		}

		answers = append(answers, content)
		if hint != "" {
			frontBuilder.WriteString(fmt.Sprintf("[... (%s)]", hint))
		} else {
			frontBuilder.WriteString("[...]")
		}

		backBuilder.WriteString(fmt.Sprintf("**[%s]**", content))
		lastIdx = fullEnd
	}

	frontBuilder.WriteString(raw[lastIdx:])
	backBuilder.WriteString(raw[lastIdx:])

	results = append(results, ClozeInfo{
		OriginalText: raw,
		FrontText:    frontBuilder.String(),
		BackText:     backBuilder.String(),
		Answer:       strings.Join(answers, ", "),
	})

	return results
}
