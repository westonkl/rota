package card

import (
	"testing"
)

func TestParseContentQA(t *testing.T) {
	md := `---
deck: "Go Fundamentals"
---

# Go Notes

Here are some notes on Go.

Q: What is a Goroutine? #concurrency #go
A: A lightweight thread of execution managed by the Go runtime.

---

Q: How do you read from a channel?
A: Using the receive operator:
` + "```go\nval, ok := <-ch\n```" + `

---

C: Channels in Go can be either [buffered] or [unbuffered]. #channels
`

	cards, err := ParseContent([]byte(md), "test.md", "fallback")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(cards) != 3 {
		t.Fatalf("expected 3 cards, got %d", len(cards))
	}

	// First card
	c1 := cards[0]
	if c1.Deck != "Go Fundamentals" {
		t.Errorf("expected deck 'Go Fundamentals', got '%s'", c1.Deck)
	}
	if c1.Type != TypeQA {
		t.Errorf("expected TypeQA, got %v", c1.Type)
	}
	if c1.Prompt != "What is a Goroutine? #concurrency #go" {
		t.Errorf("unexpected prompt: %s", c1.Prompt)
	}
	if c1.Answer != "A lightweight thread of execution managed by the Go runtime." {
		t.Errorf("unexpected answer: %s", c1.Answer)
	}
	if len(c1.Tags) != 2 {
		t.Errorf("expected 2 tags, got %d", len(c1.Tags))
	}

	// Second card (multiline code block)
	c2 := cards[1]
	if c2.Type != TypeQA {
		t.Errorf("expected TypeQA, got %v", c2.Type)
	}
	if !contains(c2.Answer, "val, ok := <-ch") {
		t.Errorf("expected answer to contain code block, got %s", c2.Answer)
	}

	// Third card (cloze)
	c3 := cards[2]
	if c3.Type != TypeCloze {
		t.Errorf("expected TypeCloze, got %v", c3.Type)
	}
	if c3.Prompt != "Channels in Go can be either [...] or [...]. #channels" {
		t.Errorf("unexpected cloze front: %s", c3.Prompt)
	}
	if c3.Answer != "Channels in Go can be either **[buffered]** or **[unbuffered]**. #channels" {
		t.Errorf("unexpected cloze back: %s", c3.Answer)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || (len(s) > 0 && len(substr) > 0 && searchSubstring(s, substr)))
}

func searchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
