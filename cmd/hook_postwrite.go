package main

import (
	"encoding/json"
	"io"
	"os"
	"strings"

	"github.com/ranwei/claude-context/pkg/classifier"
	"github.com/ranwei/claude-context/pkg/hook"
	"github.com/ranwei/claude-context/pkg/state"
)

func runPostToolUse(stdin io.Reader) {
	defer recoverAndLog("post-tool-use")
	ev := parseOrExit(stdin, "post-tool-use")
	root, _, resolved := resolveProjectFromEvent(ev)
	if !resolved {
		os.Exit(0)
	}
	state.IncrementSafe(root, "hook_fired.post-tool-use")

	edit := extractEditInput(ev)
	if edit.FilePath == "" {
		os.Exit(0)
	}

	cat := classifier.ClassifyEdit(edit.ToolName, edit.FilePath, edit.OldStr, edit.NewStr)
	state.IncrementSafe(root, "edit_pattern."+cat)

	if err := state.AppendTurnEdit(root, state.TurnEdit{
		File:      edit.FilePath,
		Category:  cat,
		LineDelta: edit.LinesAdded - edit.LinesRemoved,
	}); err != nil {
		hook.WriteStderr("post-tool-use: append turn edit: " + err.Error())
	}

	exitHook("post-tool-use", root)
}

type editInput struct {
	ToolName     string
	FilePath     string
	OldStr       string
	NewStr       string
	LinesAdded   int
	LinesRemoved int
}

func extractEditInput(ev *hook.Event) editInput {
	var inp editInput
	inp.ToolName = ev.ToolName
	if len(ev.ToolInput) == 0 {
		return inp
	}

	switch ev.ToolName {
	case "Write":
		var ti struct {
			FilePath string `json:"file_path"`
			Content  string `json:"content"`
		}
		if err := json.Unmarshal(ev.ToolInput, &ti); err == nil {
			inp.FilePath = ti.FilePath
			inp.NewStr = ti.Content
			inp.LinesAdded = countNonEmpty(ti.Content)
		}
	case "Edit":
		var ti struct {
			FilePath  string `json:"file_path"`
			OldString string `json:"old_string"`
			NewString string `json:"new_string"`
		}
		if err := json.Unmarshal(ev.ToolInput, &ti); err == nil {
			inp.FilePath = ti.FilePath
			inp.OldStr = ti.OldString
			inp.NewStr = ti.NewString
			inp.LinesAdded = countNonEmpty(ti.NewString)
			inp.LinesRemoved = countNonEmpty(ti.OldString)
		}
	case "MultiEdit":
		var ti struct {
			FilePath string `json:"file_path"`
			Edits    []struct {
				OldString string `json:"old_string"`
				NewString string `json:"new_string"`
			} `json:"edits"`
		}
		if err := json.Unmarshal(ev.ToolInput, &ti); err == nil {
			inp.FilePath = ti.FilePath
			for _, e := range ti.Edits {
				inp.OldStr += e.OldString
				inp.NewStr += e.NewString
				inp.LinesAdded += countNonEmpty(e.NewString)
				inp.LinesRemoved += countNonEmpty(e.OldString)
			}
		}
	case "NotebookEdit":
		var ti struct {
			NotebookPath string `json:"notebook_path"`
			OldSource    string `json:"old_source"`
			NewSource    string `json:"new_source"`
		}
		if err := json.Unmarshal(ev.ToolInput, &ti); err == nil {
			inp.FilePath = ti.NotebookPath
			inp.OldStr = ti.OldSource
			inp.NewStr = ti.NewSource
			inp.LinesAdded = countNonEmpty(ti.NewSource)
			inp.LinesRemoved = countNonEmpty(ti.OldSource)
		}
	}
	return inp
}

func countNonEmpty(s string) int {
	if s == "" {
		return 0
	}
	lines := 0
	for _, l := range strings.Split(s, "\n") {
		if strings.TrimSpace(l) != "" {
			lines++
		}
	}
	return lines
}
