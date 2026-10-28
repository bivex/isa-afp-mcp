package searchindex

import (
	"strings"
	"unicode"
)

var defaultStopWords = map[string]struct{}{
	"a": {}, "about": {}, "above": {}, "after": {}, "again": {}, "against": {}, "all": {},
	"am": {}, "an": {}, "and": {}, "any": {}, "are": {}, "as": {}, "at": {}, "be": {},
	"because": {}, "been": {}, "before": {}, "being": {}, "below": {}, "between": {},
	"both": {}, "but": {}, "by": {}, "could": {}, "did": {}, "do": {}, "does": {},
	"doing": {}, "down": {}, "during": {}, "each": {}, "few": {}, "for": {}, "from": {},
	"further": {}, "had": {}, "has": {}, "have": {}, "having": {}, "he": {}, "her": {},
	"here": {}, "hers": {}, "herself": {}, "him": {}, "himself": {}, "his": {}, "how": {},
	"i": {}, "if": {}, "in": {}, "into": {}, "is": {}, "it": {}, "its": {}, "itself": {},
	"me": {}, "more": {}, "most": {}, "my": {}, "myself": {}, "no": {}, "nor": {}, "not": {},
	"of": {}, "off": {}, "on": {}, "once": {}, "only": {}, "or": {}, "other": {}, "ought": {},
	"our": {}, "ours": {}, "ourselves": {}, "out": {}, "over": {}, "own": {}, "same": {},
	"she": {}, "should": {}, "so": {}, "some": {}, "such": {}, "than": {}, "that": {},
	"the": {}, "their": {}, "theirs": {}, "them": {}, "themselves": {}, "then": {},
	"there": {}, "these": {}, "they": {}, "this": {}, "those": {}, "through": {}, "to": {},
	"too": {}, "under": {}, "until": {}, "up": {}, "very": {}, "was": {}, "we": {},
	"were": {}, "what": {}, "when": {}, "where": {}, "which": {}, "while": {}, "who": {},
	"whom": {}, "why": {}, "with": {}, "would": {}, "you": {}, "your": {}, "yours": {},
	"yourself": {}, "yourselves": {},
}

// Tokenize splits text into normalized lower-case search tokens.
// It splits on non-alphanumeric chars, but also splits camelCase and snake_case while keeping
// compound tokens where appropriate.
func Tokenize(text string) []string {
	var tokens []string
	var current strings.Builder

	flush := func() {
		if current.Len() > 0 {
			tok := strings.ToLower(current.String())
			current.Reset()
			if _, isStop := defaultStopWords[tok]; !isStop && len(tok) > 1 {
				tokens = append(tokens, tok)
			}
		}
	}

	runes := []rune(text)
	n := len(runes)

	for i := range n {
		r := runes[i]
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			// CamelCase boundary check: e.g. "FormalSSA" -> "formal", "ssa"
			if unicode.IsUpper(r) && current.Len() > 0 {
				prevIsLower := unicode.IsLower(runes[i-1])
				nextIsLower := i+1 < n && unicode.IsLower(runes[i+1])
				if prevIsLower || nextIsLower {
					flush()
				}
			}
			current.WriteRune(r)
		} else {
			flush()
		}
	}
	flush()

	return tokens
}
