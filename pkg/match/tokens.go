package match

import (
	"regexp"
	"strings"
)

var nonAlnum = regexp.MustCompile(`[^a-zA-Z0-9]+`)

var stopWords = map[string]bool{
	"err": true, "nil": true, "if": true, "else": true,
	"return": true, "func": true, "var": true, "type": true,
	"the": true, "and": true, "for": true, "true": true,
	"false": true, "range": true, "len": true, "make": true,
}

// Tokenize splits s on non-alphanumeric boundaries, lowercases all tokens,
// and removes stop words and single-character tokens.
func Tokenize(s string) []string {
	parts := nonAlnum.Split(strings.ToLower(s), -1)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if len(p) <= 1 {
			continue
		}
		if stopWords[p] {
			continue
		}
		out = append(out, p)
	}
	return out
}

// TokenOverlap returns the count of tokens in a that also appear in b.
// Both slices should be pre-tokenized via Tokenize.
func TokenOverlap(a, b []string) int {
	set := make(map[string]bool, len(b))
	for _, t := range b {
		set[t] = true
	}
	count := 0
	for _, t := range a {
		if set[t] {
			count++
		}
	}
	return count
}
