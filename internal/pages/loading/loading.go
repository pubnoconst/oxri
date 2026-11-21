package loading

import _ "embed"

//go:embed style.css
var StyleCSS string

// DefaultPhrase is the non-technical confirmation message displayed during loading transitions.
const DefaultPhrase = "Confirming, please wait."

// CharToken represents a single rendered character in the magnifying-wave phrase.
type CharToken struct {
	Index   int
	Text    string
	IsSpace bool
}

// SplitPhrase splits a phrase into individual character tokens preserving word spaces.
func SplitPhrase(phrase string) []CharToken {
	if phrase == "" {
		phrase = DefaultPhrase
	}
	runes := []rune(phrase)
	out := make([]CharToken, 0, len(runes))
	for i, r := range runes {
		if r == ' ' {
			out = append(out, CharToken{Index: i, Text: "\u00A0", IsSpace: true})
		} else {
			out = append(out, CharToken{Index: i, Text: string(r), IsSpace: false})
		}
	}
	return out
}
