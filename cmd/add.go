package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"rota/internal/card"
	"rota/internal/tui"
)

var (
	flagAddFile     string
	flagAddQuestion string
	flagAddAnswer   string
	flagAddCloze    string
	flagAddTags     []string
)

var addCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a new flashcard to a markdown file",
	RunE: func(cmd *cobra.Command, args []string) error {
		reader := bufio.NewReader(os.Stdin)

		targetFile := flagAddFile
		if targetFile == "" {
			fmt.Print("Markdown file path (e.g. notes.md): ")
			input, _ := reader.ReadString('\n')
			targetFile = strings.TrimSpace(input)
			if targetFile == "" {
				targetFile = "cards.md"
			}
		}

		// Ensure parent directory exists
		if dir := filepath.Dir(targetFile); dir != "." && dir != "" {
			_ = os.MkdirAll(dir, 0755)
		}

		var cardType card.CardType
		var prompt, answer string
		var tags []string = flagAddTags

		if flagAddCloze != "" {
			cardType = card.TypeCloze
			prompt = flagAddCloze
		} else if flagAddQuestion != "" && flagAddAnswer != "" {
			cardType = card.TypeQA
			prompt = flagAddQuestion
			answer = flagAddAnswer
		} else {
			// Interactive mode
			fmt.Println(tui.StyleTitle.Render("\n✦ Create New Flashcard ✦"))
			fmt.Print("Card Type ([1] Q/A, [2] Cloze): ")
			typeChoice, _ := reader.ReadString('\n')
			typeChoice = strings.TrimSpace(typeChoice)

			if typeChoice == "2" || strings.ToLower(typeChoice) == "cloze" {
				cardType = card.TypeCloze
				fmt.Print("Cloze text (e.g. Speech is [produced] in [Broca's] area): ")
				prompt, _ = reader.ReadString('\n')
				prompt = strings.TrimSpace(prompt)
			} else {
				cardType = card.TypeQA
				fmt.Print("Question (Q:): ")
				prompt, _ = reader.ReadString('\n')
				prompt = strings.TrimSpace(prompt)

				fmt.Print("Answer (A:): ")
				answer, _ = reader.ReadString('\n')
				answer = strings.TrimSpace(answer)
			}

			if len(tags) == 0 {
				fmt.Print("Tags (comma or space separated, optional): ")
				tagsInput, _ := reader.ReadString('\n')
				tagsInput = strings.TrimSpace(tagsInput)
				if tagsInput != "" {
					for _, t := range strings.Fields(strings.ReplaceAll(tagsInput, ",", " ")) {
						tags = append(tags, strings.TrimPrefix(t, "#"))
					}
				}
			}
		}

		if prompt == "" {
			return fmt.Errorf("card content cannot be empty")
		}

		// Append card to target markdown file
		f, err := os.OpenFile(targetFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return fmt.Errorf("failed to open file %s: %w", targetFile, err)
		}
		defer f.Close()

		if err := card.WriteCardToWriter(f, cardType, prompt, answer, tags); err != nil {
			return fmt.Errorf("failed writing card to file: %w", err)
		}

		fmt.Printf("\n%s Successfully added card to %s!\n\n", tui.StyleTitle.Render("✓"), targetFile)

		// Sync immediately so the card is ready
		_, _, _, _, _, _, _ = RunScan([]string{targetFile})

		return nil
	},
}

func init() {
	addCmd.Flags().StringVarP(&flagAddFile, "file", "f", "", "Target markdown file path")
	addCmd.Flags().StringVarP(&flagAddQuestion, "question", "q", "", "Card question (for Q/A)")
	addCmd.Flags().StringVarP(&flagAddAnswer, "answer", "a", "", "Card answer (for Q/A)")
	addCmd.Flags().StringVarP(&flagAddCloze, "cloze", "c", "", "Cloze card text with [bracket] deletions")
	addCmd.Flags().StringSliceVarP(&flagAddTags, "tags", "t", nil, "Card tags")

	rootCmd.AddCommand(addCmd)
}
