package main

import (
	"fmt"
	"os"
)

const version = "0.1.0-m3"

func printTopUsage() {
	fmt.Fprintln(os.Stderr, `claude-context — Claude Code context management

Usage:
  claude-context                   start MCP server (for claude mcp add)
  claude-context hook <event>      handle a Claude Code hook event
  claude-context init [flags]      install hooks and scaffolding
  claude-context scan [--force]    build anatomy map for current project
  claude-context stats             show ledger counters for current project
  claude-context version           print version

init flags:
  --yes         skip y/N confirmation
  --dry-run     show plan without writing
  --print       print final file contents to stdout
  --project     write to <project>/.claude/settings.json
  --local       write to <project>/.claude/settings.local.json
  --no-scan     skip initial anatomy scan
  --uninstall   remove all claude-context managed entries`)
}
