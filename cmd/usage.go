package main

import (
	"fmt"
	"os"
)

var version = "0.1.0-m4b"

func printTopUsage() {
	fmt.Fprintln(os.Stderr, `mneme — Claude Code context management

Usage:
  mneme                   start MCP server (for claude mcp add)
  mneme hook <event>      handle a Claude Code hook event
  mneme init [flags]      install hooks and scaffolding
  mneme stats             show ledger counters for current project
  mneme status            print health snapshot (use --json for machine-readable)
  mneme scan              scan project files and update anatomy map
  mneme cerebrum <cmd>    manage project coding rules
  mneme buglog <cmd>      manage per-project bug history
  mneme restore           restore from backup (--list, --latest, or <timestamp>)
  mneme update            sync templates across all projects (--binary self-updates)
  mneme update --list     list initialized projects
  mneme version           print version

cerebrum commands:
  cerebrum add [--pattern P] [--message M] [--comment C]
  cerebrum list
  cerebrum remove <N> [--yes]

buglog commands:
  buglog add [--description D] [--code C] [--file F]
  buglog list
  buglog clear [--yes]

init flags:
  --yes         skip y/N confirmation
  --dry-run     show plan without writing
  --print       print final file contents to stdout
  --project     write to <project>/.claude/settings.json
  --local       write to <project>/.claude/settings.local.json
  --no-scan     skip anatomy scan on init
  --uninstall   remove all mneme managed entries`)
}
