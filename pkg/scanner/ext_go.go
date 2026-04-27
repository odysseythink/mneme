package scanner

import "strings"

func extractGo(data []byte) string {
	lines := strings.Split(string(data), "\n")
	for i, line := range lines {
		t := strings.TrimSpace(line)
		if !strings.HasPrefix(t, "func ") &&
			!strings.HasPrefix(t, "type ") &&
			!strings.HasPrefix(t, "var ") &&
			!strings.HasPrefix(t, "const ") {
			continue
		}
		// Walk backwards to find the start of a contiguous // comment block.
		commentStart := -1
		for j := i - 1; j >= 0; j-- {
			prev := strings.TrimSpace(lines[j])
			if strings.HasPrefix(prev, "//") {
				commentStart = j
			} else {
				break
			}
		}
		if commentStart >= 0 {
			raw := strings.TrimSpace(lines[commentStart])
			raw = strings.TrimPrefix(raw, "//")
			raw = strings.TrimSpace(raw)
			return truncate(raw, 100)
		}
		return truncate(t, 100)
	}
	return extractFallback(data)
}
