package main

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/ranwei/claude-context/pkg/hook"
	"github.com/ranwei/claude-context/pkg/state"
)

func runPreWrite(stdin io.Reader) {
	defer recoverAndLog("pre-write")
	ev := parseOrExit(stdin, "pre-write")
	root, _, resolved := resolveProjectFromEvent(ev)
	if !resolved {
		os.Exit(0)
	}
	state.IncrementSafe(root, "hook_fired.pre-write")

	rules, err := state.ReadCerebrum(root)
	if err != nil || len(rules) == 0 {
		exitHook("pre-write", root)
	}

	added := ev.ExtractAddedLines()
	if len(added) == 0 {
		exitHook("pre-write", root)
	}

	type matchResult struct {
		msg  string
		line int
	}
	var matches []matchResult

	for _, rule := range rules {
		re, err := regexp.Compile(rule.Pattern)
		if err != nil {
			continue
		}
		lineNums := make([]int, 0, len(added))
		for ln := range added {
			lineNums = append(lineNums, ln)
		}
		sort.Ints(lineNums)
		for _, ln := range lineNums {
			if re.MatchString(added[ln]) {
				matches = append(matches, matchResult{rule.Message, ln})
				break
			}
		}
	}

	if len(matches) == 0 {
		exitHook("pre-write", root)
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "⚠️ %d cerebrum rule(s) matched:\n", len(matches))
	for _, m := range matches {
		fmt.Fprintf(&sb, "  • %s (line %d)\n", m.msg, m.line)
	}
	hook.WriteStderr(sb.String())
	os.Exit(1)
}
