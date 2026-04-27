package tests

import (
	"os"
	"strings"
	"testing"

	"github.com/ranwei/claude-context/pkg/hook"
)

func TestParseEvent_PreRead(t *testing.T) {
	f, err := os.Open("fixtures/hook-payloads/pre-read.json")
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer f.Close()

	ev, err := hook.ParseEvent(f)
	if err != nil {
		t.Fatalf("ParseEvent: %v", err)
	}
	if ev.HookEventName != "PreToolUse" {
		t.Errorf("HookEventName = %q, want PreToolUse", ev.HookEventName)
	}
	if ev.ToolName != "Read" {
		t.Errorf("ToolName = %q, want Read", ev.ToolName)
	}
	if ev.SessionID == "" {
		t.Error("SessionID should not be empty")
	}
	fp, ok := ev.FilePathFromToolInput()
	if !ok {
		t.Error("FilePathFromToolInput: expected ok=true for pre-read")
	}
	if !strings.HasSuffix(fp, "auth.go") {
		t.Errorf("FilePathFromToolInput = %q, want suffix auth.go", fp)
	}
}

func TestParseEvent_SessionStart(t *testing.T) {
	f, _ := os.Open("fixtures/hook-payloads/session-start.json")
	defer f.Close()
	ev, err := hook.ParseEvent(f)
	if err != nil {
		t.Fatalf("ParseEvent: %v", err)
	}
	if ev.HookEventName != "SessionStart" {
		t.Errorf("HookEventName = %q, want SessionStart", ev.HookEventName)
	}
	if ev.Model == "" {
		t.Error("Model should not be empty for SessionStart")
	}
}

func TestParseEvent_Stop(t *testing.T) {
	f, _ := os.Open("fixtures/hook-payloads/stop.json")
	defer f.Close()
	ev, err := hook.ParseEvent(f)
	if err != nil {
		t.Fatalf("ParseEvent: %v", err)
	}
	if ev.IsRecursiveStop() {
		t.Error("stop_hook_active=false should not be recursive")
	}
}

func TestParseEvent_BadJSON(t *testing.T) {
	r := strings.NewReader("not json {{{")
	_, err := hook.ParseEvent(r)
	if err == nil {
		t.Error("expected error for bad JSON")
	}
}

func TestParseEvent_PreWrite(t *testing.T) {
	f, _ := os.Open("fixtures/hook-payloads/pre-write.json")
	defer f.Close()
	ev, err := hook.ParseEvent(f)
	if err != nil {
		t.Fatalf("ParseEvent: %v", err)
	}
	fp, ok := ev.FilePathFromToolInput()
	if !ok {
		t.Error("FilePathFromToolInput: expected ok=true")
	}
	if !strings.HasSuffix(fp, "hello.txt") {
		t.Errorf("FilePathFromToolInput = %q", fp)
	}
}

func TestParseEvent_PostWrite(t *testing.T) {
	f, _ := os.Open("fixtures/hook-payloads/post-write.json")
	defer f.Close()
	ev, err := hook.ParseEvent(f)
	if err != nil {
		t.Fatalf("ParseEvent: %v", err)
	}
	if ev.HookEventName != "PostToolUse" {
		t.Errorf("HookEventName = %q, want PostToolUse", ev.HookEventName)
	}
	if ev.DurationMs == nil || *ev.DurationMs != 1 {
		t.Errorf("DurationMs expected 1")
	}
}
