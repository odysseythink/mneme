# M0 — Hook Protocol Validation Spike Design

**Date:** 2026-04-27
**Status:** Draft
**Scope:** Empirically validate the hook-protocol assumptions baked into the architecture spec (`2026-04-27-mneme-hook-architecture-design.md`) before implementation. M0 is a spike — its outputs are knowledge (a written report), data (test fixtures), and corrections (inline patches to the architecture spec). It produces no product code.

---

## Problem

The architecture spec for embedding openwolf-style token-saving mechanisms into `mneme` makes seven assumptions about Claude Code's hook protocol (§13 risks R1-R7). Three of those (R1, R2, R3) are gating risks — if wrong, large parts of the M1 implementation must be redesigned. Two more (R4, R5) are virtually free to verify in the same probe session and would also force redesign if wrong.

If we proceed straight to M1 implementation without verifying these assumptions, the cost of being wrong compounds across all later milestones. The cost of verifying first is one human-in-the-loop probe session, ~30-60 minutes of the user's attention, and ~4-7 hours of total elapsed work.

---

## Goals

- Empirically answer R1-R5 with concrete data.
- Capture canonical stdin payloads from a real Claude Code session as permanent test fixtures (`tests/fixtures/hook-payloads/`).
- Inline-patch the architecture spec wherever reality contradicts the original assumptions.
- Leave a re-runnable probe harness in `scripts/hook-protocol-probe/` so future Claude Code upgrades can be re-validated cheaply.

---

## Non-Goals

- Validate R6 (concurrency model) — defer to first time concurrency actually contends.
- Validate R7 (scan memory peak) — unrelated to hook protocol; M2's domain.
- Produce any pkg-level Go code — M0 is bash + Markdown only.
- Build automated CI for hook-protocol verification — it requires real Claude Code, can't run headlessly in CI.
- Test on Windows — macOS/Linux only (tier-1 platforms).
- Validate hook behavior across multiple Claude Code versions — single version (whatever the user has installed at M0 time) is enough; future versions get re-run as their own one-off effort.

---

## Cross-cutting Decisions

Locked during the M0 brainstorm.

| Decision | Choice |
|---|---|
| Scope | R1-R5 (R6/R7 deferred) |
| Probe implementation | Bash scripts in `scripts/m0-probe/` (later renamed `scripts/hook-protocol-probe/`) |
| Sandbox location | Temporary `/tmp/mneme-m0-sandbox/` |
| Hook registration scope | `<sandbox>/.claude/settings.local.json` (project-local + git-ignored) |
| Execution model | Human in the loop (user runs runbook in a separate Claude Code session; this conversation analyses results) |
| Spec patches | Inline edits to architecture spec submitted in the same commit as the M0 report |

---

## Deliverables

### Phase 1 — M0 setup (assistant-produced, before user runs anything)

```
scripts/m0-probe/
├── echo.sh              probe: dump stdin to file, exit 0
├── exit-n.sh            probe: parameterized exit code (0/1/2/127)
├── sleep-n.sh           probe: parameterized sleep before exit (50/200/500/1000/3000 ms)
├── setup-sandbox.sh     creates /tmp/mneme-m0-sandbox/ with realistic content
├── swap-hook.sh         rewrites sandbox settings.local.json to register a different
│                        probe (e.g., swap-hook.sh exit-n.sh 2 — used by Phases 2 & 3)
├── cleanup.sh           removes sandbox + dump dir + verifies env clean
└── README.md            usage notes (later expanded for permanent home)

docs/m0-runbook.md       step-by-step instructions for the user
```

### Phase 2 — M0 report (assistant-produced, after user feeds back data)

```
docs/superpowers/specs/2026-04-27-m0-hook-protocol-validation-report.md
                          findings for R1-R5 with evidence
tests/fixtures/hook-payloads/
├── pre-read.json
├── pre-write.json
├── post-write.json
├── session-start.json
└── stop.json
                          canonical payloads selected from Phase 2 captures

(if findings contradict architecture spec)
docs/superpowers/specs/2026-04-27-mneme-hook-architecture-design.md
                          inline patches (same commit as the report)
```

### Phase 3 — release-form rename (after report is committed)

```
scripts/m0-probe/  →  scripts/hook-protocol-probe/
```

The `scripts/hook-protocol-probe/README.md` opens with a prominent **"This is NOT product code. Re-run after Claude Code upgrades."** notice and references the M0 report path.

---

## Probe Scripts

### `echo.sh`

```bash
#!/usr/bin/env bash
# Probe: dump stdin verbatim to a timestamped file under $OUT_DIR.
# Used to capture real hook payloads (R1).
set -euo pipefail
OUT_DIR="${OUT_DIR:-/tmp/mneme-m0/$1}"
mkdir -p "$OUT_DIR"
ts=$(date +%s)-$$
cat > "$OUT_DIR/${ts}-stdin.json"
exit 0
```

Invoked as: `echo.sh <event-name>`. The `<event-name>` argument segregates dumps by hook event so the user can verify each event fired as expected.

### `exit-n.sh`

```bash
#!/usr/bin/env bash
# Probe: read stdin (discard), then exit with the supplied code.
# May write a fixed marker to stderr so we can observe how Claude treats it.
# Used to pin down R2.
set -euo pipefail
N="${1:-0}"
MSG="${2:-mneme-m0 probe stderr line}"
cat > /dev/null
echo "$MSG" >&2
exit "$N"
```

Invoked via settings.local.json with command `exit-n.sh 2 "marker text"`. The user repeats with N ∈ {0, 1, 2, 127} and observes both terminal output and Claude's transcript.

### `sleep-n.sh`

```bash
#!/usr/bin/env bash
# Probe: sleep for N milliseconds before exiting 0.
# Used to find the user-perceived latency cliff (R3).
set -euo pipefail
N_MS="${1:-100}"
cat > /dev/null
# Cross-platform millisecond sleep: GNU sleep accepts fractional seconds; macOS does too.
sleep "$(awk "BEGIN { print $N_MS / 1000 }")"
exit 0
```

Invoked via settings.local.json with command `sleep-n.sh 500`. User repeats with N_MS ∈ {50, 100, 200, 500, 1000, 3000} and grades subjective friction.

### `setup-sandbox.sh`

```bash
#!/usr/bin/env bash
# Builds /tmp/mneme-m0-sandbox/ with:
#   - 4 files with realistic content (Go, Python, README, package.json)
#   - .claude/settings.local.json registering all 5 hook events to echo.sh
#   - dump dir at /tmp/mneme-m0/{pre-read,pre-write,post-write,session-start,stop}/
# Prints next-step instructions.
```

The sandbox files contain real markers (godoc, docstring, named exports) so payload variability matches what M1+ will see in production:

```
/tmp/mneme-m0-sandbox/
├── auth.go         (with package + godoc on a function)
├── utils.py        (with module docstring + def with docstring)
├── README.md       (with first-heading)
├── package.json    (known config file)
└── .claude/
    └── settings.local.json  (all 5 hook events → echo.sh)
```

### `cleanup.sh`

```bash
#!/usr/bin/env bash
# Removes:
#   - /tmp/mneme-m0-sandbox/
#   - /tmp/mneme-m0/
#   - $HOME/mneme-m0-* (R5 tilde-test files)
# Self-checks:
#   - confirms ~/.claude/settings.json was not modified (compares hash to pre-recorded baseline)
#   - confirms no stray mneme-m0 references in ~/.claude/
#   - confirms no leftover $HOME/mneme-m0-* files
#   - prints diff if any leftover detected
```

The hash baseline is captured by `setup-sandbox.sh` into `/tmp/mneme-m0-baseline-hash`. If `~/.claude/settings.json` was accidentally modified during M0, cleanup surfaces it loudly.

---

## Runbook (`docs/m0-runbook.md`)

User-facing checklist. Five phases, each with explicit "what to type" + "what to expect".

### Phase 0 — Prep

```
$ cd <go-mneme project root>
$ bash scripts/m0-probe/setup-sandbox.sh

Expected output:
  ✓ sandbox at /tmp/mneme-m0-sandbox/
  ✓ hooks registered in <sandbox>/.claude/settings.local.json (echo.sh, all 5 events)
  ✓ dump dir at /tmp/mneme-m0/
  Next: open a NEW terminal, cd into the sandbox, start Claude Code.
```

### Phase 1 — R1 + R4 + R5 joint capture

```
[NEW TERMINAL]
$ cd /tmp/mneme-m0-sandbox/
$ claude code        (this triggers SessionStart)

In Claude Code, type these prompts (one at a time, wait for completion):
  1. "read README.md"             (triggers Read → pre-read fires)
  2. "read auth.go"                (Read again → second pre-read sample)
  3. "create a file hello.txt with content 'hi'"   (Write → pre-write + post-write)
  4. "edit hello.txt to say 'hello world'"          (Edit → pre-write + post-write)
  5. "read README.md"             (third pre-read; tests repeated read)
  6. /exit                         (triggers Stop)

Then back in your original terminal:
$ ls -la /tmp/mneme-m0/
Expected: 5 subdirs (pre-read, pre-write, post-write, session-start, stop), each with N JSON files.

Paste the directory listing back to this conversation.
```

For R4 (within Phase 1): the setup-sandbox.sh's settings.local.json includes a `_managed_by: "mneme"` field on each hook entry. R4 succeeds iff Claude Code starts cleanly AND the dumps appear (i.e., hook fires despite the extra field).

For R5 (within Phase 1): the sandbox CLAUDE.md contains three `@import` lines (one per form) each pointing to a tiny file with a unique passphrase:
```
@~/.mneme-m0-tilde-test.md           (file lives in $HOME, M0-prefixed for safe cleanup)
@/tmp/mneme-m0-sandbox/.claude/m0-test-abs.md
@./.claude/m0-test-rel.md
```
Each target file says e.g. `passphrase-tilde-abc123`. After the session starts, the user asks Claude "do you see passphrase-tilde-abc123? passphrase-abs-def456? passphrase-rel-ghi789?". Whichever Claude confirms tells us which forms work.

**Cleanup obligation:** `cleanup.sh` must remove `~/.mneme-m0-tilde-test.md` (the only M0 artifact that lives outside `/tmp/`). The `mneme-m0-` prefix is mandatory for any file written outside `/tmp/` so cleanup can find them by glob.

### Phase 2 — R2 (exit codes)

```
$ bash scripts/m0-probe/swap-hook.sh exit-n.sh 0     (rewrites settings.local.json: command=exit-n.sh 0)
[NEW Claude Code session in sandbox]
  > "read README.md"
  > observe: did Claude proceed normally? did stderr appear in terminal?
  > /exit
$ bash scripts/m0-probe/swap-hook.sh exit-n.sh 1     (rewrite to exit 1)
[repeat]
$ bash scripts/m0-probe/swap-hook.sh exit-n.sh 2     (rewrite to exit 2)
[repeat — pay close attention to whether Claude is BLOCKED here]
$ bash scripts/m0-probe/swap-hook.sh exit-n.sh 127   (command-not-found analog)
[repeat]

Paste a short report per exit code:
  - exit 0: behavior = ?
  - exit 1: behavior = ?
  - exit 2: behavior = ?  (key: blocked? stderr fed to Claude?)
  - exit 127: behavior = ?
```

`swap-hook.sh` is a small helper that rewrites settings.local.json to swap the registered hook command (e.g., `swap-hook.sh exit-n.sh 2` or `swap-hook.sh sleep-n.sh 500`). Implementation uses `sed -i.bak` to substitute the `"command": "..."` value with an absolute path to the new probe + arguments.

### Phase 3 — R3 (latency)

```
$ bash scripts/m0-probe/swap-hook.sh sleep-n.sh 50
[Claude Code session: read README.md; subjectively grade 1-5]
$ bash scripts/m0-probe/swap-hook.sh sleep-n.sh 200    [grade]
$ bash scripts/m0-probe/swap-hook.sh sleep-n.sh 500    [grade]
$ bash scripts/m0-probe/swap-hook.sh sleep-n.sh 1000   [grade]
$ bash scripts/m0-probe/swap-hook.sh sleep-n.sh 3000   [grade — also: did Claude Code time out?]

Grading scale:
  5 = no perceptible delay
  4 = slight delay, fine
  3 = noticeable but tolerable
  2 = annoying, would not want this every Read
  1 = broken / unusable

Paste the 5 grades + any timeout observations.
```

### Phase 4 — Cleanup

```
$ bash scripts/m0-probe/cleanup.sh

Expected output:
  ✓ /tmp/mneme-m0-sandbox/ removed
  ✓ /tmp/mneme-m0/ removed
  ✓ ~/.claude/settings.json unchanged from baseline
  ✓ no leftover references found
```

If cleanup reports any anomaly, the user pastes the output back; we investigate before declaring M0 done.

---

## Report Format

`docs/superpowers/specs/2026-04-27-m0-hook-protocol-validation-report.md`:

```markdown
# M0 Hook Protocol Validation Report
**Date:** YYYY-MM-DD
**Claude Code version:** (from `claude --version`)
**OS:** (uname -a)
**Tester:** ranwei

## Summary
| Risk | Original assumption (arch spec ref) | Reality (evidence) | Action |
|---|---|---|---|
| R1 | §6 stdin schema | (real fields list) | "no patch" / "patch §6: rename X to Y" |
| R2 | exit 2 = block + feed stderr (assumed) | exit 0=..., 1=..., 2=..., 127=... | (decision on output contract) |
| R3 | preread 80ms p99 budget | grades: 50ms=5, 200ms=4, 500ms=3, 1s=2, 3s=1 (or whatever) | (revised budgets) |
| R4 | _managed_by tolerated | yes/no with evidence | (boundary scheme adjustment if needed) |
| R5 | all 3 import forms work | ✓~ ✓abs ✗rel (or whatever) | (rules.md path-form constraint) |

## R1: stdin schema findings
For each of the 5 hook events, list:
- Top-level fields observed (name : type)
- tool_input shape (per tool: Read / Write / Edit)
- Optional vs always-present fields (cross-sample comparison)
- Diff against architecture spec §6 assumption
- Evidence: link to captured fixtures

## R2: exit code semantics findings
For each exit code in {0, 1, 2, 127}:
- Was Claude blocked from proceeding?
- Where did stderr go (terminal vs Claude's transcript vs both vs nowhere)?
- Did Claude react in its next message to the stderr content?
- Verdict on which exit code(s) we should use for "informational stderr, never block"

## R3: latency tolerance findings
- Grade per sleep value (1-5)
- Identified cliff (e.g., "tolerable up to 200ms; objectionable from 500ms")
- Did Claude Code enforce its own timeout? At what duration?
- Verdict on revised latency budgets for §5/§6

## R4: settings.json schema tolerance
- Did Claude Code start with `_managed_by` field present?
- Did the hook still fire?
- Were any warnings/errors produced anywhere?
- Verdict on §8 boundary marker scheme

## R5: @import path forms
- For each form (~ / abs / rel): did Claude load the imported file?
- Evidence: passphrase recall result
- Verdict on init's rules-injection path generation

## Architecture spec patches required
Bullet list of every change made to `2026-04-27-mneme-hook-architecture-design.md`:
- §X.Y: changed "..." to "..." (reason: R<N> revealed ...)
- ...
(Empty list = no patches needed.)

## Fixture selection
For each of the 5 canonical fixtures in tests/fixtures/hook-payloads/:
- Source capture file
- Why this one was chosen as canonical (most-typical / has all optional fields / etc.)
```

---

## Acceptance

M0 is "done" when **all** of the following hold:

1. `docs/superpowers/specs/2026-04-27-m0-hook-protocol-validation-report.md` exists and gives definitive answers to R1-R5 (no "TBD", no "needs more research").
2. `tests/fixtures/hook-payloads/{pre-read,pre-write,post-write,session-start,stop}.json` all exist and parse as valid JSON.
3. `scripts/hook-protocol-probe/` exists (renamed from `scripts/m0-probe/`) with README and all 5 scripts (echo / exit-n / sleep-n / setup-sandbox / swap-hook / cleanup). README documents the manual re-verification procedure (linking to runbook). A one-shot `run-all.sh` wrapper is **out of M0 scope** — left as a future improvement once we have empirical data on what "all" means.
4. The architecture spec is internally consistent with the M0 report — any contradictions resolved via inline patches in the same commit as the report.
5. `cleanup.sh` exit code 0 with all self-checks passing (no leftover sandbox / no modified user settings.json).

---

## Risks of M0 Itself

| # | Risk | Mitigation |
|---|---|---|
| MA | User's Claude Code version doesn't support some hook events (older release without SessionStart/Stop) | Report records "event X unavailable on version Y" verbatim; arch spec annotated with degradation plan |
| MB | Non-deterministic Claude Code behavior (same prompt produces different tool calls) | Runbook requires ≥2 runs per phase; if results disagree, escalate to ≥5 runs and take majority |
| MC | User attention drift during R3 grading | Runbook reminds user to close noisy processes and grade after ≥2 trials per latency value |
| MD | Sandbox cleanup fails silently | cleanup.sh self-checks compare ~/.claude/settings.json hash to baseline captured at setup; mismatch surfaces loudly with a diff |
| ME | Probe scripts incompatible across macOS bash 3.2 vs Linux bash 5+ | Use `#!/usr/bin/env bash`, avoid GNU-only date/sed flags, restrict to POSIX-friendly constructs |
| MF | Captured payloads contain user-private paths (e.g., /Users/ranwei/...) | Fixture canonicalization step replaces user-specific paths with placeholders before commit (e.g., `<HOME>` for $HOME) |

---

## Effort Budget

- Assistant: prepare scripts + runbook + this spec ≈ 2-3h
- User: execute the runbook ≈ 30-60min wall-clock, undivided attention
- Assistant: analyze results, write report, select fixtures, patch arch spec ≈ 1-2h
- **Total: 4-7h, may span 1-2 calendar days**

---

## What Comes After M0

After M0 commits and user approves the report:
1. Brainstorm M1 (Foundation) detailed spec — now grounded in real protocol facts
2. Write M1 plan with `superpowers:writing-plans`
3. Execute M1
4. (Repeat for M2-M6)
