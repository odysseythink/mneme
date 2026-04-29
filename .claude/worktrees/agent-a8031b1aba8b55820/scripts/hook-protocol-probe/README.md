# Hook Protocol Probe Scripts

> **THIS IS NOT PRODUCT CODE.** These bash scripts validate Claude Code's hook protocol assumptions baked into `claude-context`. They were originally created during the M0 spike (see `docs/superpowers/specs/2026-04-27-m0-hook-protocol-validation-design.md`) and are kept here for **re-verification when Claude Code's hook protocol changes**.

To re-validate against a new Claude Code version: follow the runbook at `docs/m0-runbook.md` and diff your findings against the M0 report at `docs/superpowers/specs/2026-04-27-m0-hook-protocol-validation-report.md`. Patch the architecture spec at `docs/superpowers/specs/2026-04-27-claude-context-hook-architecture-design.md` if the protocol has shifted.

## Scripts

| Script | Purpose | Used by |
|---|---|---|
| `echo.sh <event>` | Dump stdin to `/tmp/claude-context-m0/<event>/<ts>-stdin.json`, exit 0 | R1 (schema capture) |
| `exit-n.sh <code> [msg]` | Discard stdin, write msg to stderr, exit with code | R2 (exit code semantics) |
| `sleep-n.sh <ms>` | Discard stdin, sleep N ms, exit 0 | R3 (latency tolerance) |
| `swap-hook.sh <probe> [args]` | Rewrite sandbox `settings.local.json` to register a different probe | Phase 2 + 3 |
| `setup-sandbox.sh` | Build `/tmp/claude-context-m0-sandbox/` + register hooks + capture baseline | Phase 0 (setup) |
| `cleanup.sh` | Remove sandbox + dump dir + R5 leftovers + verify env clean | Phase 4 (cleanup) |

## Usage

Run the runbook at `docs/m0-runbook.md`. It walks through 4 phases:

1. Setup (`setup-sandbox.sh`)
2. Capture (R1 + R4 + R5: real Claude Code session in sandbox)
3. Probe variations (R2 exit codes, R3 latency: `swap-hook.sh` + new sessions)
4. Cleanup (`cleanup.sh`)

## Requirements

- macOS or Linux with bash 3.2+
- POSIX-compatible `sed`, `awk`, `shasum`, `grep`
- A real installed Claude Code CLI (not headless)

## Troubleshooting

- **No payloads in dump dir after Phase 1:** likely Claude Code doesn't honour hooks at the `settings.local.json` level. Try moving the config to `<sandbox>/.claude/settings.json` (drop `.local`).
- **`cleanup.sh` reports `~/.claude/settings.json` modified:** investigate manually before discarding the baseline. M0 should never touch user-level settings.
