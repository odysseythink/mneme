package tests

import (
	"testing"

	"github.com/ranwei/claude-context/pkg/match"
)

func TestTokenizeStripsStopWords(t *testing.T) {
	got := match.Tokenize("var err = nil")
	if len(got) != 0 {
		t.Errorf("expected empty slice, got %v", got)
	}
}

func TestTokenizeStripsShortTokens(t *testing.T) {
	got := match.Tokenize("x = y + z")
	if len(got) != 0 {
		t.Errorf("expected empty slice, got %v", got)
	}
}

func TestTokenizeSplitsOnBoundaries(t *testing.T) {
	got := match.Tokenize("session.StopCount++")
	want := []string{"session", "stopcount"}
	if len(got) != 2 || got[0] != "session" || got[1] != "stopcount" {
		t.Errorf("expected %v, got %v", want, got)
	}
}

func TestTokenOverlapMeetsThreshold(t *testing.T) {
	a := match.Tokenize("session.StopCount++\nsession.Value = session.Init()")
	b := match.Tokenize("session.StopCount++")
	if match.TokenOverlap(a, b) < 3 {
		t.Errorf("expected overlap >= 3, got %d", match.TokenOverlap(a, b))
	}
}

func TestTokenOverlapBelowThreshold(t *testing.T) {
	a := match.Tokenize("completely different code block")
	b := match.Tokenize("session.StopCount++")
	if match.TokenOverlap(a, b) >= 3 {
		t.Errorf("expected overlap < 3, got %d", match.TokenOverlap(a, b))
	}
}

func TestAutoUpsertGuardTokenCount(t *testing.T) {
	short := match.Tokenize("x := 1")
	if len(short) >= 5 {
		t.Errorf("short string should tokenize to < 5 tokens, got %d: %v", len(short), short)
	}
	long := match.Tokenize("session.StopCount++\nbadValue := session.Init()")
	if len(long) < 5 {
		t.Errorf("substantial string should tokenize to >= 5 tokens, got %d: %v", len(long), long)
	}
}
