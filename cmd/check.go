package cmd

import (
	"bufio"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"rota/internal/card"
	"rota/internal/tui"
)

var checkCmd = &cobra.Command{
	Use:     "check [path...]",
	Aliases: []string{"lint", "validate"},
	Short:   "Check markdown notes for syntax errors or malformed flashcards",
	RunE: func(cmd *cobra.Command, args []string) error {
		targets := args
		if len(targets) == 0 {
			targets = []string{flagVaultPath}
		}

		fmt.Println(tui.StyleTitle.Render("✦ Linting markdown flashcards..."))

		files := make(map[string]bool)
		for _, target := range targets {
			_ = filepath.WalkDir(target, func(path string, d fs.DirEntry, walkErr error) error {
				if walkErr != nil {
					return walkErr
				}
				if d.IsDir() && strings.HasPrefix(d.Name(), ".") && d.Name() != "." {
					return filepath.SkipDir
				}
				if !d.IsDir() && strings.HasSuffix(strings.ToLower(d.Name()), ".md") {
					abs, _ := filepath.Abs(path)
					files[abs] = true
				}
				return nil
			})
		}

		issuesCount := 0
		validCardsCount := 0

		for f := range files {
			fileIssues, validCards := lintFile(f)
			validCardsCount += validCards
			if len(fileIssues) > 0 {
				rel, _ := filepath.Rel(".", f)
				fmt.Printf("\n%s %s\n", lipgloss.NewStyle().Bold(true).Render("File:"), rel)
				for _, iss := range fileIssues {
					issuesCount++
					fmt.Printf("  %s Line %d: %s\n", lipgloss.NewStyle().Foreground(tui.ColorWarning).Render("⚠"), iss.line, iss.msg)
				}
			}
		}

		fmt.Println()
		if issuesCount == 0 {
			fmt.Printf("%s All %d flashcards are valid with no syntax issues!\n\n",
				lipgloss.NewStyle().Foreground(tui.ColorSuccess).Bold(true).Render("✓ Clean:"), validCardsCount)
		} else {
			fmt.Printf("%s Found %d issue(s) across markdown files.\n\n",
				lipgloss.NewStyle().Foreground(tui.ColorWarning).Bold(true).Render("Notice:"), issuesCount)
		}

		return nil
	},
}

type lintIssue struct {
	line int
	msg  string
}

func lintFile(filePath string) ([]lintIssue, int) {
	var issues []lintIssue
	data, err := os.ReadFile(filePath)
	if err != nil {
		return []lintIssue{{line: 0, msg: fmt.Sprintf("failed to read file: %v", err)}}, 0
	}

	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	inQuestion := false
	questionLine := 0

	for idx, line := range lines {
		lineNum := idx + 1
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "Q:") || strings.HasPrefix(trimmed, "q:") {
			if inQuestion {
				issues = append(issues, lintIssue{
					line: questionLine,
					msg:  "Question is missing an answer ('A:') before next question",
				})
			}
			inQuestion = true
			questionLine = lineNum
			continue
		}

		if strings.HasPrefix(trimmed, "A:") || strings.HasPrefix(trimmed, "a:") {
			if !inQuestion {
				issues = append(issues, lintIssue{
					line: lineNum,
					msg:  "Found answer ('A:') without preceding question ('Q:')",
				})
			}
			inQuestion = false
			continue
		}

		if strings.HasPrefix(trimmed, "C:") || strings.HasPrefix(trimmed, "c:") {
			if inQuestion {
				issues = append(issues, lintIssue{
					line: questionLine,
					msg:  "Question is missing an answer ('A:') before cloze card",
				})
				inQuestion = false
			}

			// Check if cloze has brackets or {{c1::}}
			if !strings.Contains(trimmed, "[") && !strings.Contains(trimmed, "{{") {
				issues = append(issues, lintIssue{
					line: lineNum,
					msg:  "Cloze deletion has no brackets (e.g. 'C: Word is [hidden]')",
				})
			}
			continue
		}
	}

	if inQuestion {
		issues = append(issues, lintIssue{
			line: questionLine,
			msg:  "Trailing question is missing an answer ('A:')",
		})
	}

	cards, _ := card.ParseContent(data, filePath, "default")
	return issues, len(cards)
}

func init() {
	rootCmd.AddCommand(checkCmd)
}
