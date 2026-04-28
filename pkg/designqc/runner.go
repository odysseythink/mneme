package designqc

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type RunOptions struct {
	ProjectRoot   string
	ProjectID     string
	HomeDir       string
	RouteOverride []string
	Quality       int
	MaxWidth      int
	Port          int
	BaseURL       string
	Framework     string
}

type Capturer interface {
	Capture(opts CaptureOptions) ([]byte, int, int, error)
	Close()
}

func Run(ctx context.Context, opts RunOptions) (*Report, error) {
	if _, err := DetectChrome(); err != nil {
		return nil, err
	}

	if opts.Framework == "" {
		fw, err := DetectFramework(opts.ProjectRoot)
		if err != nil {
			return nil, err
		}
		opts.Framework = fw.Name
		if opts.Port == 0 {
			opts.Port = fw.DefaultPort
		}
	}
	if opts.BaseURL == "" {
		port := opts.Port
		if port == 0 {
			port = 5173
		}
		opts.BaseURL = fmt.Sprintf("http://localhost:%d", port)
	}

	if err := WaitForDevServer(opts.BaseURL, 1*time.Second); err != nil {
		return nil, err
	}

	br := NewBrowser(ctx)
	defer br.Close()
	return RunWithCapturer(ctx, opts, br)
}

func RunWithCapturer(ctx context.Context, opts RunOptions, cap Capturer) (*Report, error) {
	routes, err := EnumerateRoutes(opts.ProjectRoot, opts.RouteOverride)
	if err != nil {
		return nil, err
	}
	if len(routes) == 0 {
		return nil, errors.New("no routes to capture")
	}

	runDir := filepath.Join(opts.HomeDir, ".mneme", "designqc", opts.ProjectID)
	capturesDir := filepath.Join(runDir, "captures")
	if err := os.RemoveAll(capturesDir); err != nil {
		return nil, fmt.Errorf("wipe captures: %w", err)
	}
	if err := os.MkdirAll(capturesDir, 0o700); err != nil {
		return nil, fmt.Errorf("mkdir captures: %w", err)
	}

	report := &Report{
		Version:    ReportVersion,
		CapturedAt: time.Now().UTC().Format(time.RFC3339),
		Framework:  opts.Framework,
		BaseURL:    opts.BaseURL,
		Captures:   []Capture{},
	}

	successCount := 0
	for _, r := range routes {
		url := strings.TrimRight(opts.BaseURL, "/") + r.Path
		fileName := fmt.Sprintf("route-%s.jpg", r.Slug)
		jpeg, w, h, err := cap.Capture(CaptureOptions{
			URL: url, Quality: opts.Quality, MaxWidth: opts.MaxWidth,
		})
		entry := Capture{
			Route:        r.Path,
			File:         fileName,
			CapturedAtMS: time.Now().UnixMilli(),
		}
		if err != nil {
			entry.Error = err.Error()
			fmt.Fprintf(os.Stderr, "designqc: warn: route %s: %v\n", r.Path, err)
			report.Captures = append(report.Captures, entry)
			continue
		}
		path := filepath.Join(capturesDir, fileName)
		if werr := os.WriteFile(path, jpeg, 0o600); werr != nil {
			entry.Error = werr.Error()
			report.Captures = append(report.Captures, entry)
			continue
		}
		entry.Width = w
		entry.Height = h
		report.Captures = append(report.Captures, entry)
		successCount++
	}

	if werr := WriteReport(runDir, report); werr != nil {
		return nil, fmt.Errorf("write report: %w", werr)
	}
	if successCount == 0 {
		return report, errors.New("all routes failed to capture")
	}
	return report, nil
}
