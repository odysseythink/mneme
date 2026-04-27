package main

import (
	"fmt"
	"os"
)

const version = "0.1.0-m4b"

func printTopUsage() {
	fmt.Fprintln(os.Stderr, `claude-context — Claude Code context management

Usage:
  claude-context                   start MCP server (for claude mcp add)
  claude-context hook <event>      handle a Claude Code hook event
  claude-context init [flags]      install hooks and scaffolding
  claude-context stats             show ledger counters for current project
  claude-context scan              scan project files and update anatomy map
  claude-context cerebrum <cmd>    manage project coding rules
  claude-context buglog <cmd>      manage per-project bug history
  claude-context version           print version

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
  --uninstall   remove all claude-context managed entries`)
}
