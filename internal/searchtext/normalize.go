package searchtext

import (
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

func Normalize(text string) string {
	text = strings.ToLower(strings.TrimSpace(text))
	if text == "" {
		return ""
	}
	decomposed := norm.NFD.String(text)
	var b strings.Builder
	b.Grow(len(decomposed))
	lastWasSpace := false
	for _, r := range decomposed {
		switch {
		case unicode.Is(unicode.Mn, r):
			continue
		case unicode.IsSpace(r):
			if lastWasSpace {
				continue
			}
			b.WriteRune(' ')
			lastWasSpace = true
		default:
			b.WriteRune(r)
			lastWasSpace = false
		}
	}
	return strings.TrimSpace(b.String())
}

func Tokenize(text string) []string {
	normalized := Normalize(text)
	if normalized == "" {
		return nil
	}
	return strings.Fields(normalized)
}
