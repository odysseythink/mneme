package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ranwei/claude-context/pkg/hook"
	"github.com/ranwei/claude-context/pkg/state"
)

func dispatchHook(args []string) {
	if len(args) < 1 {
		os.Exit(2)
	}
	switch args[0] {
	case "pre-read":
		runPreRead(os.Stdin)
	case "pre-write":
		runPreWrite(os.Stdin)
	case "post-write":
		runPostWrite(os.Stdin)
	case "post-tool-use":
		runPostToolUse(os.Stdin)
	case "session-start":
		runSessionStart(os.Stdin)
	case "stop":
		runStop(os.Stdin)
	default:
		os.Exit(0) // unknown event: silent forward-compat
	}
}

func runPreRead(stdin io.Reader) {
	defer recoverAndLog("pre-read")
	ev := parseOrExit(stdin, "pre-read")
	root, _, resolved := resolveProjectFromEvent(ev)
	if !resolved {
		os.Exit(0)
	}
	state.IncrementSafe(root, "hook_fired.pre-read")

	absPath, ok := ev.FilePathFromToolInput()
	if !ok {
		exitHook("pre-read", root)
		return
	}
	// Resolve symlinks to match the canonical git root path
	absPath, _ = filepath.EvalSymlinks(absPath)
	relPath, err := filepath.Rel(root, absPath)
	if err != nil || strings.HasPrefix(relPath, "..") {
		exitHook("pre-read", root)
		return
	}

	anatomy, _ := state.ReadAnatomy(root)
	alreadyRead, _ := state.AppendSessionRead(root, relPath)

	if entry, ok := anatomy[relPath]; ok {
		state.IncrementSafe(root, "anatomy_hits")
		hook.WriteStderr(fmt.Sprintf("%s — %s (~%d tok)", relPath, entry.Description, entry.EstTokens))
		os.Exit(1)
	}
	if alreadyRead {
		state.IncrementSafe(root, "repeat_reads")
		hook.WriteStderr(fmt.Sprintf("%s already read this session", relPath))
		os.Exit(1)
	}

	exitHook("pre-read", root)
}

func runPreWrite(stdin io.Reader) {
	defer recoverAndLog("pre-write")
	ev := parseOrExit(stdin, "pre-write")
	root, _, resolved := resolveProjectFromEvent(ev)
	if !resolved {
		os.Exit(0)
	}
	state.IncrementSafe(root, "hook_fired.pre-write")
	exitHook("pre-write", root)
}

func runPostWrite(stdin io.Reader) {
	defer recoverAndLog("post-write")
	ev := parseOrExit(stdin, "post-write")
	root, _, resolved := resolveProjectFromEvent(ev)
	if !resolved {
		os.Exit(0)
	}
	state.IncrementSafe(root, "hook_fired.post-write")
	exitHook("post-write", root)
}


// parseOrExit parses from r; on failure logs and exits 0 (never blocks Claude).
func parseOrExit(r io.Reader, hookName string) *hook.Event {
	ev, err := hook.ParseEvent(r)
	if err != nil {
		appendGlobalLog(fmt.Sprintf("%s stdin parse error: %v", hookName, err))
		os.Exit(0)
	}
	return ev
}

// resolveProjectFromEvent locates the project root from the event's file path or cwd.
// Returns (projectRoot, projectID, ok). ok=false means outside any initialized project.
func resolveProjectFromEvent(ev *hook.Event) (string, string, bool) {
	var startDir string
	switch ev.HookEventName {
	case "PreToolUse", "PostToolUse":
		if fp, ok := ev.FilePathFromToolInput(); ok {
			startDir = filepath.Dir(fp)
		} else {
			startDir = ev.Cwd
		}
	default:
		startDir = ev.Cwd
	}
	if startDir == "" {
		return "", "", false
	}

	root, ok := state.FindGitRoot(startDir)
	if !ok {
		root = startDir
	}

	// Require .claude-context/ to exist (i.e., init was run)
	if _, err := os.Stat(filepath.Join(root, ".claude-context")); err != nil {
		return "", "", false
	}

	id, err := state.ReadOrCreateLocalID(root)
	if err != nil {
		return "", "", false
	}
	return root, id, true
}

// exitHook exits 0 silently, or exits 1 with debug message when CLAUDE_CONTEXT_DEBUG>=1.
// Level 2 also prints extra detail to stdout (visible in user terminal, not Claude transcript).
func exitHook(eventName, projectRoot string) {
	level := hook.DebugLevel()
	if level >= 2 {
		id, _ := state.ReadOrCreateLocalID(projectRoot)
		fmt.Printf("[claude-context debug] %s fired project=%s\n", eventName, id)
	}
	if level >= 1 {
		id, _ := state.ReadOrCreateLocalID(projectRoot)
		hook.WriteStderr(fmt.Sprintf("M1 stub: %s fired (project=%s)", eventName, id))
		os.Exit(1) // exit 1 makes Claude see the stderr message (M0 R2)
	}
	os.Exit(0)
}

func recoverAndLog(hookName string) {
	if r := recover(); r != nil {
		home, _ := os.UserHomeDir()
		path := filepath.Join(home, ".claude-context", "hook-errors.log")
		os.MkdirAll(filepath.Dir(path), 0755)
		entry := fmt.Sprintf("%s panic in %s: %v\n", time.Now().UTC().Format(time.RFC3339), hookName, r)
		if f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); err == nil {
			f.WriteString(entry)
			f.Close()
		}
		os.Exit(0)
	}
}

func appendGlobalLog(msg string) {
	home, _ := os.UserHomeDir()
	path := filepath.Join(home, ".claude-context", "hook-errors.log")
	os.MkdirAll(filepath.Dir(path), 0755)
	entry := fmt.Sprintf("%s %s\n", time.Now().UTC().Format(time.RFC3339), msg)
	if f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); err == nil {
		f.WriteString(entry)
		f.Close()
	}
}
