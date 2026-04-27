package state

import (
	"os"
	"path/filepath"
	"strings"
	"time"
)

type CerebrumRule struct {
	Comment string
	Pattern string
	Message string
}

const cerebrumHeader = "<!-- claude-context cerebrum v1 -->"

func ReadCerebrum(projectRoot string) ([]CerebrumRule, error) {
	path := filepath.Join(projectRoot, ".claude-context", "cerebrum.md")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return parseCerebrum(string(data)), nil
}

func AppendCerebrumRule(projectRoot string, rule CerebrumRule) error {
	dir := filepath.Join(projectRoot, ".claude-context")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	lockPath := filepath.Join(dir, "cerebrum.lock")
	release, err := AcquireLock(lockPath, 50*time.Millisecond)
	if err != nil {
		return err
	}
	defer release()

	path := filepath.Join(dir, "cerebrum.md")
	existing, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	var content string
	if len(existing) == 0 {
		content = cerebrumHeader + "\n\n" + formatRule(rule)
	} else {
		content = string(existing) + "\n" + formatRule(rule)
	}
	return AtomicWrite(path, []byte(content))
}

func WriteCerebrum(projectRoot string, rules []CerebrumRule) error {
	dir := filepath.Join(projectRoot, ".claude-context")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	lockPath := filepath.Join(dir, "cerebrum.lock")
	release, err := AcquireLock(lockPath, 50*time.Millisecond)
	if err != nil {
		return err
	}
	defer release()

	var sb strings.Builder
	sb.WriteString(cerebrumHeader + "\n")
	for _, r := range rules {
		sb.WriteString("\n")
		sb.WriteString(formatRule(r))
	}
	return AtomicWrite(filepath.Join(dir, "cerebrum.md"), []byte(sb.String()))
}

func formatRule(r CerebrumRule) string {
	var sb strings.Builder
	if r.Comment != "" {
		sb.WriteString("# " + r.Comment + "\n")
	}
	sb.WriteString("pattern: " + r.Pattern + "\n")
	sb.WriteString("warning: " + r.Message + "\n")
	return sb.String()
}

func parseCerebrum(content string) []CerebrumRule {
	var rules []CerebrumRule
	var cur CerebrumRule
	inRule := false

	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.HasPrefix(line, "<!--") || line == "" {
			if inRule && cur.Pattern != "" && cur.Message != "" {
				rules = append(rules, cur)
				cur = CerebrumRule{}
				inRule = false
			}
			continue
		}
		if strings.HasPrefix(line, "# ") {
			if inRule && cur.Pattern != "" && cur.Message != "" {
				rules = append(rules, cur)
				cur = CerebrumRule{}
			}
			cur.Comment = strings.TrimPrefix(line, "# ")
			inRule = true
			continue
		}
		if strings.HasPrefix(line, "pattern: ") {
			cur.Pattern = strings.TrimPrefix(line, "pattern: ")
			inRule = true
			continue
		}
		if strings.HasPrefix(line, "warning: ") {
			cur.Message = strings.TrimPrefix(line, "warning: ")
			inRule = true
			continue
		}
	}
	if inRule && cur.Pattern != "" && cur.Message != "" {
		rules = append(rules, cur)
	}
	return rules
}
