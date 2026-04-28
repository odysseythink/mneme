package daemon

import (
	"encoding/json"
	"os"

	"github.com/ranwei/mneme/pkg/state"
)

// Manifest is the user-editable cron manifest at ~/.mneme/daemon/cron-manifest.json.
type Manifest struct {
	Version int            `json:"version"`
	Tasks   []ManifestTask `json:"tasks"`
}

type ManifestTask struct {
	Name     string `json:"name"`
	Schedule string `json:"schedule"`
	Enabled  bool   `json:"enabled"`
}

// CronState is per-task runtime state at ~/.mneme/daemon/cron-state.json.
type CronState struct {
	Version int                  `json:"version"`
	Tasks   map[string]TaskState `json:"tasks"`
}

type TaskState struct {
	LastRun             string `json:"last_run"`
	LastSuccess         string `json:"last_success"`
	LastError           string `json:"last_error"`
	ConsecutiveFailures int    `json:"consecutive_failures"`
	DeadLetteredAt      string `json:"dead_lettered_at"`
}

func defaultManifest() Manifest {
	return Manifest{
		Version: 1,
		Tasks: []ManifestTask{
			{Name: "anatomy-rescan", Schedule: "@every 6h", Enabled: true},
			{Name: "consolidate-memory", Schedule: "0 3 * * *", Enabled: true},
			{Name: "prune-backups", Schedule: "30 3 * * 0", Enabled: true},
			{Name: "weekly-waste-report", Schedule: "0 9 * * 1", Enabled: true},
			{Name: "suggestions-refresh", Schedule: "0 4 * * *", Enabled: true},
		},
	}
}

func LoadManifest(home string) (Manifest, error) {
	data, err := os.ReadFile(state.DaemonManifestPath(home))
	if err != nil {
		return Manifest{}, err
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return Manifest{}, err
	}
	return m, nil
}

func LoadOrSeedManifest(home string) (Manifest, error) {
	m, err := LoadManifest(home)
	if err == nil {
		return m, nil
	}
	if !os.IsNotExist(err) {
		return Manifest{}, err
	}
	seed := defaultManifest()
	if err := SaveManifest(home, seed); err != nil {
		return Manifest{}, err
	}
	return seed, nil
}

func SaveManifest(home string, m Manifest) error {
	if err := os.MkdirAll(state.DaemonDir(home), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return state.AtomicWrite(state.DaemonManifestPath(home), data)
}

func LoadCronState(home string) (CronState, error) {
	data, err := os.ReadFile(state.DaemonStatePath(home))
	if err != nil {
		if os.IsNotExist(err) {
			return CronState{Version: 1, Tasks: map[string]TaskState{}}, nil
		}
		return CronState{}, err
	}
	var s CronState
	if err := json.Unmarshal(data, &s); err != nil {
		return CronState{}, err
	}
	if s.Tasks == nil {
		s.Tasks = map[string]TaskState{}
	}
	return s, nil
}

func SaveCronState(home string, s CronState) error {
	if err := os.MkdirAll(state.DaemonDir(home), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return state.AtomicWrite(state.DaemonStatePath(home), data)
}
