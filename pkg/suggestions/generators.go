package suggestions

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/ranwei/mneme/pkg/state"
)

func newID(typ, target string) string {
	h := sha256.Sum256([]byte("v1|" + typ + "|" + target))
	return hex.EncodeToString(h[:8])
}

// GenStaleAnatomy emits one suggestion per file whose mtime is more than 7
// days newer than the anatomy.md file.
func GenStaleAnatomy(projectRoot string, now time.Time) []Suggestion {
	anatomyPath := filepath.Join(projectRoot, ".mneme", "anatomy.md")
	anatomyInfo, err := os.Stat(anatomyPath)
	if err != nil {
		return nil
	}
	entries, err := state.ReadAnatomy(projectRoot)
	if err != nil {
		return nil
	}
	var out []Suggestion
	cutoff := anatomyInfo.ModTime().Add(7 * 24 * time.Hour)
	for path := range entries {
		fi, err := os.Stat(filepath.Join(projectRoot, path))
		if err != nil {
			continue
		}
		if fi.ModTime().After(cutoff) {
			out = append(out, Suggestion{
				ID:          newID("stale_anatomy", path),
				Type:        "stale_anatomy",
				Target:      path,
				Title:       "Anatomy entry is stale",
				Detail:      fmt.Sprintf("%s changed after anatomy was generated; run: mneme scan", path),
				GeneratedAt: now.Format(time.RFC3339),
			})
		}
	}
	return out
}

// GenStaleRule emits one suggestion per cerebrum rule when cerebrum.md
// has not been touched in the last 90 days.
func GenStaleRule(projectRoot string, now time.Time) []Suggestion {
	rules, err := state.ReadCerebrum(projectRoot)
	if err != nil || len(rules) == 0 {
		return nil
	}
	info, err := os.Stat(filepath.Join(projectRoot, ".mneme", "cerebrum.md"))
	if err != nil {
		return nil
	}
	if now.Sub(info.ModTime()) < 90*24*time.Hour {
		return nil
	}
	var out []Suggestion
	for _, r := range rules {
		out = append(out, Suggestion{
			ID:          newID("stale_rule", r.Pattern),
			Type:        "stale_rule",
			Target:      r.Pattern,
			Title:       "Cerebrum rule has not matched in 90+ days",
			Detail:      fmt.Sprintf("rule %q may be obsolete; run: mneme cerebrum list", r.Pattern),
			GeneratedAt: now.Format(time.RFC3339),
		})
	}
	return out
}

// GenUnreadFile and GenCoRead are best-effort stubs (require session archives).
func GenUnreadFile(projectRoot string, now time.Time) []Suggestion {
	return nil
}

func GenCoRead(projectRoot string, now time.Time) []Suggestion {
	return nil
}
