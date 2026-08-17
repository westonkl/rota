package anki

import (
	"html"
	"regexp"
	"strings"
)

var (
	// HTML conversion regexes
	reBr         = regexp.MustCompile(`(?i)<br\s*/?>`)
	reDivStart   = regexp.MustCompile(`(?i)<div[^>]*>`)
	reDivEnd     = regexp.MustCompile(`(?i)</div>`)
	rePStart     = regexp.MustCompile(`(?i)<p[^>]*>`)
	rePEnd       = regexp.MustCompile(`(?i)</p>`)
	reBold       = regexp.MustCompile(`(?is)<(?:b|strong)[^>]*>(.*?)</(?:b|strong)>`)
	reItalic     = regexp.MustCompile(`(?is)<(?:i|em)[^>]*>(.*?)</(?:i|em)>`)
	reStrike     = regexp.MustCompile(`(?is)<(?:s|strike|del)[^>]*>(.*?)</(?:s|strike|del)>`)
	rePreCode    = regexp.MustCompile(`(?is)<pre[^>]*><code[^>]*>(.*?)</code></pre>`)
	rePre        = regexp.MustCompile(`(?is)<pre[^>]*>(.*?)</pre>`)
	reCode       = regexp.MustCompile(`(?is)<code[^>]*>(.*?)</code>`)
	reImg        = regexp.MustCompile(`(?i)<img[^>]+src=["']?([^"'>]+)["']?[^>]*>`)
	reSound      = regexp.MustCompile(`(?i)\[sound:([^\]]+)\]`)
	reAnkiCloze  = regexp.MustCompile(`\{\{c\d+::([^}]+?)(?:::([^}]+?))?\}\}`)
	reAnyTag     = regexp.MustCompile(`<[^>]+>`)
	reMultiNl    = regexp.MustCompile(`\n{3,}`)
	reMultiSpace = regexp.MustCompile(`[ \t]{2,}`)
)

// HTMLToMarkdown converts Anki HTML formatted fields into clean Markdown.
func HTMLToMarkdown(raw string, mediaPrefix string) string {
	if strings.TrimSpace(raw) == "" {
		return ""
	}

	text := raw

	// 1. Handle Code blocks before other tags
	text = rePreCode.ReplaceAllStringFunc(text, func(m string) string {
		match := rePreCode.FindStringSubmatch(m)
		if len(match) > 1 {
			code := html.UnescapeString(match[1])
			return "\n```\n" + strings.TrimSpace(code) + "\n```\n"
		}
		return m
	})

	text = rePre.ReplaceAllStringFunc(text, func(m string) string {
		match := rePre.FindStringSubmatch(m)
		if len(match) > 1 {
			code := html.UnescapeString(match[1])
			return "\n```\n" + strings.TrimSpace(code) + "\n```\n"
		}
		return m
	})

	text = reCode.ReplaceAllStringFunc(text, func(m string) string {
		match := reCode.FindStringSubmatch(m)
		if len(match) > 1 {
			code := html.UnescapeString(match[1])
			return "`" + strings.TrimSpace(code) + "`"
		}
		return m
	})

	// 2. Images: <img src="foo.png"> -> ![](./media/foo.png)
	text = reImg.ReplaceAllStringFunc(text, func(m string) string {
		match := reImg.FindStringSubmatch(m)
		if len(match) > 1 {
			src := strings.TrimSpace(match[1])
			if mediaPrefix != "" && !strings.HasPrefix(src, "http") {
				src = mediaPrefix + src
			}
			return "![](" + src + ")"
		}
		return ""
	})

	// 3. Audio sounds: [sound:foo.mp3] -> (Audio: foo.mp3)
	text = reSound.ReplaceAllString(text, "(Audio: $1)")

	// 4. Line breaks and block elements
	text = reBr.ReplaceAllString(text, "\n")
	text = reDivStart.ReplaceAllString(text, "\n")
	text = reDivEnd.ReplaceAllString(text, "")
	text = rePStart.ReplaceAllString(text, "\n\n")
	text = rePEnd.ReplaceAllString(text, "")

	// 5. Basic inline formatting
	text = reBold.ReplaceAllString(text, "**$1**")
	text = reItalic.ReplaceAllString(text, "*$1*")
	text = reStrike.ReplaceAllString(text, "~~$1~~")

	// 6. Strip residual HTML tags (spans, styles, font tags, tables wrappers if any)
	text = reAnyTag.ReplaceAllString(text, "")

	// 7. Unescape HTML entities (&nbsp;, &lt;, &gt;, &amp;, &quot;, &#39;)
	text = html.UnescapeString(text)
	text = strings.ReplaceAll(text, "\u00a0", " ") // Non-breaking space

	// 8. Clean excess whitespace & newlines
	text = reMultiSpace.ReplaceAllString(text, " ")
	text = reMultiNl.ReplaceAllString(text, "\n\n")

	return strings.TrimSpace(text)
}

// ConvertAnkiClozeToRota converts Anki cloze syntax {{c1::word::hint}} to Rota cloze syntax [word|hint].
func ConvertAnkiClozeToRota(raw string) string {
	return reAnkiCloze.ReplaceAllStringFunc(raw, func(m string) string {
		sub := reAnkiCloze.FindStringSubmatch(m)
		if len(sub) > 1 {
			content := sub[1]
			hint := ""
			if len(sub) > 2 && sub[2] != "" {
				hint = sub[2]
			}
			if hint != "" {
				return "[" + content + "|" + hint + "]"
			}
			return "[" + content + "]"
		}
		return m
	})
}
