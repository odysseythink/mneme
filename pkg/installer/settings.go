package installer

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/ranwei/claude-context/pkg/state"
)

type hookEntry struct {
	Type        string `json:"type"`
	Command     string `json:"command"`
	ManagedBy   string `json:"_managed_by,omitempty"`
	Version     int    `json:"_version,omitempty"`
	InstalledAt string `json:"_installed_at,omitempty"`
}

type hookMatcher struct {
	Matcher string      `json:"matcher"`
	Hooks   []hookEntry `json:"hooks"`
}

type hooksMap map[string][]hookMatcher

type hookConfig struct {
	event   string
	matcher string
	subCmd  string
}

var managedHooks = []hookConfig{
	{"PreToolUse", "Read", "hook pre-read"},
	{"PreToolUse", "Write", "hook pre-write"},
	{"PostToolUse", "Write", "hook post-write"},
	{"SessionStart", "", "hook session-start"},
	{"Stop", "", "hook stop"},
}

const managedBy = "claude-context"

// MergeHooks updates settingsPath with our 5 hook entries. Idempotent.
// binaryPath is the absolute path to the claude-context binary.
func MergeHooks(settingsPath, binaryPath string) error {
	raw := loadRawSettings(settingsPath)
	hm := parseHooksMap(raw)
	now := time.Now().UTC().Format(time.RFC3339)

	for _, cfg := range managedHooks {
		entry := hookEntry{
			Type:        "command",
			Command:     binaryPath + " " + cfg.subCmd,
			ManagedBy:   managedBy,
			Version:     1,
			InstalledAt: now,
		}
		upsertMatcher(hm, cfg.event, cfg.matcher, entry)
	}

	return saveHooksMap(settingsPath, raw, hm)
}

// UninstallHooks removes all entries with _managed_by == "claude-context".
func UninstallHooks(settingsPath string) error {
	raw := loadRawSettings(settingsPath)
	hm := parseHooksMap(raw)

	for event, matchers := range hm {
		var newMatchers []hookMatcher
		for _, m := range matchers {
			var kept []hookEntry
			for _, h := range m.Hooks {
				if h.ManagedBy != managedBy {
					kept = append(kept, h)
				}
			}
			if len(kept) > 0 {
				m.Hooks = kept
				newMatchers = append(newMatchers, m)
			}
		}
		if len(newMatchers) > 0 {
			hm[event] = newMatchers
		} else {
			delete(hm, event)
		}
	}

	return saveHooksMap(settingsPath, raw, hm)
}

func upsertMatcher(hm hooksMap, event, matcher string, entry hookEntry) {
	matchers := hm[event]
	for i, m := range matchers {
		if m.Matcher == matcher {
			var kept []hookEntry
			for _, h := range m.Hooks {
				if h.ManagedBy != managedBy {
					kept = append(kept, h)
				}
			}
			matchers[i].Hooks = append(kept, entry)
			hm[event] = matchers
			return
		}
	}
	hm[event] = append(matchers, hookMatcher{
		Matcher: matcher,
		Hooks:   []hookEntry{entry},
	})
}

func loadRawSettings(path string) map[string]json.RawMessage {
	data, err := os.ReadFile(path)
	if err != nil {
		return make(map[string]json.RawMessage)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return make(map[string]json.RawMessage)
	}
	return raw
}

func parseHooksMap(raw map[string]json.RawMessage) hooksMap {
	v, ok := raw["hooks"]
	if !ok {
		return make(hooksMap)
	}
	var hm hooksMap
	if err := json.Unmarshal(v, &hm); err != nil {
		return make(hooksMap)
	}
	return hm
}

func saveHooksMap(settingsPath string, raw map[string]json.RawMessage, hm hooksMap) error {
	if len(hm) == 0 {
		delete(raw, "hooks")
	} else {
		data, _ := json.Marshal(hm)
		raw["hooks"] = json.RawMessage(data)
	}
	return writeRawSettings(settingsPath, raw)
}

func writeRawSettings(path string, raw map[string]json.RawMessage) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return err
	}
	return state.AtomicWrite(path, data)
}
