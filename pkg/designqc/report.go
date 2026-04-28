package designqc

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ranwei/mneme/pkg/state"
)

const ReportVersion = 1

type Report struct {
	Version    int       `json:"version"`
	CapturedAt string    `json:"captured_at"`
	Framework  string    `json:"framework"`
	BaseURL    string    `json:"base_url"`
	Captures   []Capture `json:"captures"`
}

type Capture struct {
	Route        string `json:"route"`
	File         string `json:"file"`
	Width        int    `json:"width"`
	Height       int    `json:"height"`
	CapturedAtMS int64  `json:"captured_at_ms"`
	Error        string `json:"error,omitempty"`
}

const reportFile = "report.json"

func WriteReport(runDir string, r *Report) error {
	if err := os.MkdirAll(runDir, 0o700); err != nil {
		return err
	}
	if r.Captures == nil {
		r.Captures = []Capture{}
	}
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	return state.AtomicWrite(filepath.Join(runDir, reportFile), data)
}

func ReadReport(runDir string) (*Report, error) {
	data, err := os.ReadFile(filepath.Join(runDir, reportFile))
	if err != nil {
		return nil, err
	}
	var r Report
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, err
	}
	if r.Version != ReportVersion {
		return nil, fmt.Errorf("unsupported report version %d (want %d)", r.Version, ReportVersion)
	}
	return &r, nil
}
