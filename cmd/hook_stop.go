package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/ranwei/mneme/pkg/cerebrum"
	"github.com/ranwei/mneme/pkg/config"
	"github.com/ranwei/mneme/pkg/hook"
	"github.com/ranwei/mneme/pkg/state"
)

func runStop(stdin io.Reader) {
	defer recoverAndLog("stop")
	ev := parseOrExit(stdin, "stop")
	if ev.IsRecursiveStop() {
		os.Exit(0)
	}
	root, _, resolved := resolveProjectFromEvent(ev)
	if !resolved {
		os.Exit(0)
	}
	state.IncrementSafe(root, "hook_fired.stop")

	if err := state.AggregateTurn(root); err != nil {
		hook.WriteStderr("stop: aggregate turn: " + err.Error())
	}

	if l, err := state.ReadLedger(root); err == nil {
		snap := state.LedgerSnapshot{
			TS:        time.Now().UTC().Format(time.RFC3339),
			SessionID: ev.SessionID,
			Totals:    l.Totals,
		}
		if err := state.AppendLedgerHistory(root, snap); err != nil {
			hook.WriteStderr("stop: ledger history: " + err.Error())
		}
	}

	cfg := config.FromEnv()
	if cfg.CerebrumLearningEnabled && ev.TranscriptPath != "" {
		dispatchCerebrumLearn(root, ev.SessionID, ev.TranscriptPath)
	}

	exitHook("stop", root)
}

func dispatchCerebrumLearn(root, sessionID, transcriptPath string) {
	id, err := state.ReadOrCreateLocalID(root)
	if err != nil {
		return
	}
	if postCerebrumLearn(id, sessionID, transcriptPath) {
		return
	}
	defer func() { _ = recover() }()
	cerebrum.Learn(transcriptPath, root, sessionID, time.Now().UTC())
}

func postCerebrumLearn(projectID, sessionID, transcriptPath string) bool {
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	socket := filepath.Join(home, ".mneme", "daemon", "socket")
	if _, err := os.Stat(socket); err != nil {
		return false
	}

	tr := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			d := net.Dialer{Timeout: 200 * time.Millisecond}
			return d.DialContext(ctx, "unix", socket)
		},
	}
	client := &http.Client{Transport: tr, Timeout: 200 * time.Millisecond}

	body, _ := json.Marshal(map[string]string{
		"project_id":      projectID,
		"transcript_path": transcriptPath,
		"session_id":      sessionID,
	})
	req, err := http.NewRequest("POST", "http://unix/cerebrum/learn", bytes.NewReader(body))
	if err != nil {
		return false
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusAccepted || resp.StatusCode == http.StatusOK
}
