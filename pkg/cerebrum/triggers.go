package cerebrum

import "strings"

// TriggerPhrases lists hardcoded user-message keywords flagging a possible
// "user corrected Claude" moment. Match is case-insensitive substring.
var TriggerPhrases = []string{
	"corrected", "should not", "shouldn't", "instead", "wrong",
	"don't", "do not", "actually", "revert",
	"避免", "不要", "不应", "错误", "改回",
}

type TriggerHit struct {
	Phrase string
	Offset int
}

// ScanText returns one TriggerHit per first occurrence of each phrase.
func ScanText(s string) []TriggerHit {
	lower := strings.ToLower(s)
	var hits []TriggerHit
	seen := make(map[string]bool)
	for _, p := range TriggerPhrases {
		if seen[p] {
			continue
		}
		idx := strings.Index(lower, strings.ToLower(p))
		if idx < 0 {
			continue
		}
		hits = append(hits, TriggerHit{Phrase: p, Offset: idx})
		seen[p] = true
	}
	return hits
}
