package cerebrum

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/ranwei/mneme/pkg/state"
)

const (
	defaultRejectTTLDays = 90
	maxUserMsgLen        = 200
	maxPriorAsstLen      = 400
)

type transcriptLine struct {
	Type    string          `json:"type"`
	Message json.RawMessage `json:"message"`
}

type userMsg struct {
	Content string `json:"content"`
}

type asstMsg struct {
	Content []asstBlock `json:"content"`
}

type asstBlock struct {
	Type  string          `json:"type"`
	Text  string          `json:"text,omitempty"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`
}

type editInput struct {
	FilePath string `json:"file_path"`
}

// Learn reads a Claude Code transcript JSONL and persists candidate rules.
func Learn(transcriptPath, projectRoot, sessionID string, now time.Time) ([]Candidate, error) {
	f, err := os.Open(transcriptPath)
	if err != nil {
		return nil, fmt.Errorf("open transcript: %w", err)
	}
	defer f.Close()

	existing, _ := state.ReadCerebrum(projectRoot)
	existingPatterns := make(map[string]bool, len(existing))
	for _, r := range existing {
		existingPatterns[r.Pattern] = true
	}

	rejected, _ := LoadRejected(projectRoot, defaultRejectTTLDays, now)

	var (
		out       []Candidate
		priorTxt  string
		priorFile string
		turn      int
	)
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for sc.Scan() {
		turn++
		var line transcriptLine
		if err := json.Unmarshal(sc.Bytes(), &line); err != nil {
			continue
		}
		switch line.Type {
		case "assistant":
			var m asstMsg
			if err := json.Unmarshal(line.Message, &m); err != nil {
				continue
			}
			priorTxt, priorFile = summarizeAssistant(m)
		case "user":
			var u userMsg
			if err := json.Unmarshal(line.Message, &u); err != nil {
				continue
			}
			hits := ScanText(u.Content)
			for _, h := range hits {
				cand := buildCandidate(h, u.Content, priorTxt, priorFile, turn, sessionID, now)
				if existingPatterns[cand.DraftRule.Pattern] {
					continue
				}
				if rejected[cand.ID] {
					continue
				}
				if err := AppendPending(projectRoot, cand); err != nil {
					return nil, err
				}
				out = append(out, cand)
			}
		}
	}
	return out, sc.Err()
}

func summarizeAssistant(m asstMsg) (text, filePath string) {
	var sb strings.Builder
	for _, b := range m.Content {
		switch b.Type {
		case "text":
			sb.WriteString(b.Text)
			sb.WriteString(" ")
		case "tool_use":
			if b.Name == "Edit" || b.Name == "Write" || b.Name == "MultiEdit" {
				var ei editInput
				if err := json.Unmarshal(b.Input, &ei); err == nil && ei.FilePath != "" && filePath == "" {
					filePath = ei.FilePath
				}
			}
		}
	}
	return strings.TrimSpace(sb.String()), filePath
}

func buildCandidate(h TriggerHit, userMsg, priorTxt, priorFile string, turn int, sessionID string, now time.Time) Candidate {
	pattern := priorFile
	if pattern != "" {
		pattern = regexp.QuoteMeta(pattern)
	} else {
		pattern = firstToken(priorTxt)
	}
	msg := truncate(userMsg, 80)
	cand := Candidate{
		Trigger: Trigger{
			Phrase:    h.Phrase,
			UserMsg:   truncate(userMsg, maxUserMsgLen),
			PriorAsst: truncate(priorTxt, maxPriorAsstLen),
			Turn:      turn,
		},
		DraftRule: state.CerebrumRule{
			Comment: fmt.Sprintf("learned from session %s turn %d", sessionID, turn),
			Pattern: pattern,
			Message: msg,
		},
		Confidence: 1.0,
		QueuedAt:   now.UTC().Format(time.RFC3339),
		HitCount:   1,
	}
	cand.ID = NewCandidateID(h.Phrase, priorTxt)
	return cand
}

func firstToken(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "."
	}
	for i, r := range s {
		if r == ' ' || r == '\n' || r == '\t' {
			return regexp.QuoteMeta(s[:i])
		}
	}
	return regexp.QuoteMeta(s)
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}
