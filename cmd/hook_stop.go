package main

import (
	"io"
	"os"
	"time"

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

	exitHook("stop", root)
}
