package main

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/ranwei/mneme/pkg/hook"
	"github.com/ranwei/mneme/pkg/match"
	"github.com/ranwei/mneme/pkg/state"
)

func runPreWrite(stdin io.Reader) {
	defer recoverAndLog("pre-write")
	ev := parseOrExit(stdin, "pre-write")
	root, id, resolved := resolveProjectFromEvent(ev)
	if !resolved {
		os.Exit(0)
	}
	state.IncrementSafe(root, "hook_fired.pre-write")
	publishHookFired("pre-write", id, nil)

	rules, _ := state.ReadCerebrum(root)
	entries, _ := state.ReadBuglog(root)

	if len(rules) == 0 && len(entries) == 0 {
		exitHook("pre-write", root)
	}

	added := ev.ExtractAddedLines()
	if len(added) == 0 {
		exitHook("pre-write", root)
	}

	type warnItem struct {
		source string
		msg    string
		was    string
		line   int
	}
	var warns []warnItem

	// Pass 1: cerebrum regex matching
	for _, rule := range rules {
		re, err := regexp.Compile(rule.Pattern)
		if err != nil {
			appendGlobalLog(fmt.Sprintf("pre-write: invalid regex %q: %v", rule.Pattern, err))
			continue
		}
		for _, lineNo := range sortedLineNos(added) {
			if re.MatchString(added[lineNo]) {
				warns = append(warns, warnItem{"cerebrum", rule.Message, "", lineNo})
				break
			}
		}
	}

	// Pass 2: buglog token-overlap matching
	if len(entries) > 0 {
		addedSlice := make([]string, 0, len(added))
		for _, lineNo := range sortedLineNos(added) {
			addedSlice = append(addedSlice, added[lineNo])
		}
		newTokens := match.Tokenize(strings.Join(addedSlice, "\n"))

		for _, entry := range entries {
			badTokens := match.Tokenize(entry.BadCode)
			if match.TokenOverlap(newTokens, badTokens) < 3 {
				continue
			}
			firstLine := 0
			for _, lineNo := range sortedLineNos(added) {
				if match.TokenOverlap(match.Tokenize(added[lineNo]), badTokens) >= 1 {
					firstLine = lineNo
					break
				}
			}
			firstLineOfBad := entry.BadCode
			if i := strings.Index(firstLineOfBad, "\n"); i >= 0 {
				firstLineOfBad = firstLineOfBad[:i]
			}
			warns = append(warns, warnItem{"buglog", entry.Description, firstLineOfBad, firstLine})
		}
	}

	if len(warns) == 0 {
		exitHook("pre-write", root)
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "⚡ mneme: ⚠️ %d rule(s)/match(es):\n", len(warns))
	for _, w := range warns {
		if w.was != "" {
			fmt.Fprintf(&sb, "  • [%s] %s (line %d)\n    was: %s\n", w.source, w.msg, w.line, w.was)
		} else {
			fmt.Fprintf(&sb, "  • [%s] %s (line %d)\n", w.source, w.msg, w.line)
		}
	}
	hook.WriteStderr(sb.String())
	os.Exit(1)
}

// sortedLineNos returns the integer keys of m in ascending order.
func sortedLineNos(m map[int]string) []int {
	keys := make([]int, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	return keys
}
