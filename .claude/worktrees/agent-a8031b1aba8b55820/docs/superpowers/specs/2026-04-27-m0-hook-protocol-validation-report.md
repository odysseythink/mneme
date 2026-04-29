# M0 Hook Protocol Validation Report

**Date:** 2026-04-27
**Claude Code version:** 2.1.119 (Sonnet 4.6 · Claude Max)
**OS:** macOS (Darwin)
**Tester:** ranwei
**Status:** Complete — all 5 risks resolved with definitive answers; 5 architecture-spec patches required (see §"Architecture spec patches required").

---

## Summary

| Risk | Original assumption (arch spec ref) | Reality (evidence) | Action |
|---|---|---|---|
| **R1** | §6 stdin schema: only `session_id`, `transcript_path`, `tool_name`, `tool_input`, `tool_response` | Real schema has 6+ additional fields per event (`cwd`, `hook_event_name`, `permission_mode`, `tool_use_id`, `duration_ms`, plus event-specific: `source`+`model` for SessionStart, `stop_hook_active`+`last_assistant_message` for Stop). `tool_name` is NOT present in SessionStart or Stop. | **PATCH §6** — expand schema documentation per-event |
| **R2** | "exit ≥1 only used when hook crashed; exit 0 + stderr is the informational channel" | exit 0 stderr is **silent to Claude** (terminal only). exit 1 + stderr is "non-blocking error" wrapped as `Failed with non-blocking status code: <stderr>` and IS visible to Claude. exit 2 makes Claude perceive blocking. exit 127 = same as exit 1. | **PATCH §6 + §9** — informational hooks must use **exit 1**, not exit 0 |
| **R3** | preread 80ms / prewrite-postwrite 200ms / stop 500ms total budgets | Subjective grades not collected (objective inference only). All 6 latencies (50ms→3000ms) completed without Claude Code timing out the hook. No enforced upper bound observed under 3000ms. | **KEEP** budgets as designed; defer subjective re-validation to M2-M4 with real workloads |
| **R4** | `_managed_by` extra field tolerated by settings.json schema | Claude Code 2.1.119 accepted `_managed_by: "claude-context"` and `_version: 1` extra fields on every hook entry; all hooks fired normally | **NO PATCH** — boundary scheme stands as designed |
| **R5** | All 3 `@import` path forms (`@~/`, `@/abs`, `@./rel`) work | All three forms loaded their target files; Claude saw all 3 passphrases | **NO PATCH** — init can use any form; tilde form preferred for portability |

**Bonus finding (not a numbered risk)**: Stop hook fires after **every assistant turn**, not just session exit. This invalidates arch spec §5 Flow F's design and requires patching M3's session-end handling.

---

## R1: stdin schema findings

10 stdin payloads captured across 5 hook events. All payloads share 4 always-present top-level fields, plus event-specific additions.

### Common fields (all events)

| Field | Type | Notes |
|---|---|---|
| `session_id` | string (UUID) | persists across one Claude Code session lifecycle |
| `transcript_path` | string (absolute path) | `<HOME>/.claude/projects/<project-slug>/<session-id>.jsonl` |
| `cwd` | string | Claude Code's process working directory; useful for project root resolution |
| `hook_event_name` | string | one of "PreToolUse", "PostToolUse", "SessionStart", "Stop" |

### Event-specific fields

| Field | PreToolUse | PostToolUse | SessionStart | Stop |
|---|---|---|---|---|
| `permission_mode` | ✓ ("bypassPermissions" in samples) | ✓ | — | ✓ |
| `tool_name` | ✓ | ✓ | — | — |
| `tool_input` | ✓ | ✓ | — | — |
| `tool_use_id` | ✓ | ✓ | — | — |
| `tool_response` | — | ✓ | — | — |
| `duration_ms` | — | ✓ (int, ms) | — | — |
| `source` | — | — | ✓ ("startup" in sample) | — |
| `model` | — | — | ✓ (e.g., "claude-opus-4-7[1m]") | — |
| `stop_hook_active` | — | — | — | ✓ (bool — recursion guard) |
| `last_assistant_message` | — | — | — | ✓ (string — assistant's just-emitted text) |

### `tool_input` shape per tool

- `Read`: `{file_path: string}` — no `limit`/`offset` in our samples (those are optional)
- `Write`: `{file_path: string, content: string}`
- `Edit`: `{file_path: string, old_string: string, new_string: string, replace_all: bool}`

### `tool_response` shape per tool (PostToolUse only)

- `Write`: `{type: "create"|"update", filePath, content, structuredPatch, originalFile, userModified}`
- `Edit`: `{filePath, oldString, newString, originalFile, structuredPatch[{oldStart, oldLines, newStart, newLines, lines[]}], userModified, replaceAll}`

### Diff vs arch spec §6

Arch spec §6 listed only `session_id`, `transcript_path`, `tool_name`, `tool_input`, `tool_response`. **Real schema has more — and one assumption is wrong**:

- ❌ `tool_name` is **NOT** present in `SessionStart` or `Stop` events
- ➕ Add `cwd` (always)
- ➕ Add `hook_event_name` (always; unifies dispatch)
- ➕ Add `permission_mode` (PreToolUse / PostToolUse / Stop)
- ➕ Add `tool_use_id` (PreToolUse / PostToolUse) — **valuable**: lets us correlate Pre with Post for the same tool call
- ➕ Add `duration_ms` (PostToolUse) — free latency telemetry
- ➕ Add `source`, `model` (SessionStart)
- ➕ Add `stop_hook_active`, `last_assistant_message` (Stop)

Evidence: `tests/fixtures/hook-payloads/{pre-read,pre-write,post-write,session-start,stop}.json` (one canonical sample per event).

---

## R2: exit code semantics findings

Tested with `exit-n.sh <code> <marker>` registered on all 5 hook events; observed Claude's transcript reaction per code.

| exit code | Visible to user terminal? | Visible to Claude? | Claude's framing | Tool call proceeded? |
|---|---|---|---|---|
| **0** | yes (presumed; not explicitly checked) | **NO** | silent — hook output dropped before reaching model | ✓ |
| **1** | yes | **YES** as `Failed with non-blocking status code: <stderr>` | "non-blocking error" — Claude sees the message, model is told the operation didn't fail | ✓ |
| **2** | yes | **YES** as more elaborate hook-error block | **"blocking" perception** — Claude verbally responded as if blocked even though file was actually read (`Read 1 file (ctrl+o to expand)` did appear) | ✓ (but Claude believed otherwise) |
| **127** | yes | **YES** as `Failed with non-blocking status code: <stderr>` | identical framing to exit 1 | ✓ |

### Verdict on output contract

🚨 **Arch spec §6 was wrong**: it assumed informational hooks would use `exit 0 + stderr`. Reality: **exit 0 + stderr is silent to Claude.** Anatomy hits, cerebrum warnings, buglog hints would all be silently dropped.

**Decision for M1+ implementation**:
- Use **`exit 1`** for informational hooks. Claude sees the stderr text wrapped as `Failed with non-blocking status code: <our message>`.
- Wrapping noise is unfortunate but content is delivered.
- **Avoid `exit 2`** — Claude perceives it as blocking and verbally treats the action as denied.
- **Avoid `exit 0`** for informational stderr — it's silent to Claude.
- Reserve **`exit 0`** for the case where the hook explicitly does NOT want to send any feedback to Claude (e.g., session-start state init that has nothing to communicate).

### Stop hook framing (additional observation)

Stop hook errors (any non-zero exit) include the **full hook command path** in the message format: `Stop hook error: [<full command>]: <stderr>`. This is verbose noise in Claude's view.

### Follow-up to investigate (post-M0)

Modern Claude Code may support **JSON stdout** from hooks for structured feedback that bypasses the "Failed with non-blocking status code:" wrapping. A small future spike using the same probe scripts (modify echo.sh to emit JSON stdout) could verify this. If it works, we'd have a cleaner feedback channel than exit 1 + stderr. **Not blocking M1.**

---

## R3: latency tolerance findings

| sleep | Subjective grade | Hook completed? | Claude Code timeout observed? |
|---|---|---|---|
| 50ms | not collected | yes | no |
| 100ms | not collected | yes | no |
| 200ms | not collected | yes | no |
| 500ms | not collected | yes | no |
| 1000ms | not collected | yes | no |
| 3000ms | not collected | yes | **no** — Claude Code waited the full 3 seconds without enforcing a timeout |

### Conservative inference (no subjective data)

| Latency band | Assumed user impact |
|---|---|
| ≤ 100ms | imperceptible — safe for any-frequency hooks |
| 200-500ms | likely noticeable — acceptable for low-frequency events (Stop, post-write) |
| 1000ms+ | clearly noticeable — only for genuinely background work (fork-and-detach) |
| 3000ms+ | annoying — avoid synchronous use |

### Verdict on arch spec §5/§6 budgets

**KEEP** all budgets as designed:
- preread: 80ms p99 total (within imperceptible band)
- prewrite/postwrite: 200ms total (within "likely noticeable" band)
- stop: 500ms total (acceptable for low-frequency events)

**Defer subjective grading** to M2-M4 timeframe — by then we'll have real anatomy/cerebrum/buglog lookups instead of artificial sleep, and grading will reflect actual feature impact.

**Important Claude Code property confirmed**: no enforced hook timeout under 3000ms. Means our soft-timeout policy in §6 is purely self-imposed; Claude Code will wait as long as we take.

---

## R4: settings.json schema tolerance

**Result: ✅ PASS — `_managed_by` field is tolerated.**

Evidence:
- `setup-sandbox.sh` injected `"_managed_by": "claude-context", "_version": 1` on every hook entry (5 entries total) in `<sandbox>/.claude/settings.local.json`
- Claude Code 2.1.119 started without any error message
- All 5 hook events fired and produced dump files (verified via `ls /tmp/claude-context-m0/<event>/`)

**Verdict on arch spec §8 boundary scheme**: NO PATCH NEEDED. The `_managed_by` field-based boundary marker for `--uninstall` filtering works as designed.

---

## R5: @import path forms

**Result: ✅ ALL THREE FORMS WORK.**

CLAUDE.md contained:
```
@~/claude-context-m0-tilde-test.md
@/tmp/claude-context-m0-sandbox/.claude/m0-test-abs.md
@./.claude/m0-test-rel.md
```

Each target file held a unique passphrase. Asked Claude "do you see any of these passphrases?" — Claude confirmed seeing all three:
- `passphrase-tilde-abc123` ← `@~/path` form ✓
- `passphrase-abs-def456` ← `@/abs/path` form ✓
- `passphrase-rel-ghi789` ← `@./rel/path` form ✓

**Verdict on arch spec §8 init's `@import` line**: NO PATCH NEEDED. Init can use the planned `@~/.claude/claude-context-rules.md` form.

---

## Bonus finding: Stop hook fires per assistant turn

Not in the original R1-R7 list, but discovered during R1 sample collection. **Critical for M3.**

**Evidence**: one Claude Code session containing ~7 user prompts produced **6 dumps** in `/tmp/claude-context-m0/stop/` (one per assistant response, plus follow-ups).

**Implication for arch spec §5 Flow F**: the planned design treats Stop as session-final ("compute session totals → append session row to memory.md → delete `_session.json`"). With per-turn semantics, this would:
- Append 6+ session-summary rows per session
- Delete `_session.json` mid-session, breaking subsequent state lookups

**Mitigation paths (all valid; pick during M3 spec brainstorm)**:
1. Treat `Stop` as a "turn-end" event (write turn-summary to memory.md, leave `_session.json` intact); detect actual session end via process death or explicit `/exit` watcher
2. Use `stop_hook_active` field as a recursion guard; defer real session-finalization to a future `SessionEnd` event if Claude Code adds one
3. Maintain a "last-N-turns" rolling buffer in `_session.json` and let session expiry happen on idle timeout

**Verdict**: M3 spec brainstorm must explicitly address turn-vs-session semantics. **Architecture spec patch needed** — see "Architecture spec patches required" below.

---

## Architecture spec patches required

Five patches to `docs/superpowers/specs/2026-04-27-claude-context-hook-architecture-design.md`:

1. **§6 Hook Protocol → Input section** — replace the abbreviated stdin schema example with the complete real schema (all 4 common fields + per-event variations from R1 above).

2. **§6 Hook Protocol → Output contract table** — `exit 0 + stderr` is **NOT** the informational channel. Update to specify:
   - **exit 1** = informational channel (stderr fed to Claude wrapped as "non-blocking error")
   - **exit 0** = silent (use only when hook has no message for Claude)
   - **exit 2** = avoid (Claude perceives blocking)

3. **§6 Hook Protocol → "To verify in M1"** — remove this line (it's verified now: M0 report).

4. **§5 Flow F (Stop)** — add explicit note: "Stop fires per assistant turn, not per session. M3 must distinguish turn-end from session-end semantics." Reference the M0 report.

5. **§13 Risks table** — mark R1-R5 as VALIDATED with link to M0 report; R6 and R7 still pending (M3 and M2 respectively).

These patches will be applied in Task 17 (next step after this report).

---

## Fixture selection

5 canonical fixtures committed at `tests/fixtures/hook-payloads/`. Selection criteria + sanitization:

### `pre-read.json` (447 bytes raw)
- **Source**: `/tmp/claude-context-m0/pre-read/1777278755-755-27192-stdin.json` (`auth.go` Read)
- **Why chosen**: Picked Read of `auth.go` over README.md — auth.go is more representative of "production code being read" than a single-line README. Three pre-read samples all had identical schemas; size choice was secondary.
- **Sanitized**: `session_id` → `<SESSION_ID>`; `transcript_path` → `<TRANSCRIPT_PATH>`. Other paths kept verbatim (sandbox `/private/tmp/...` paths are reproducible, no PII).

### `pre-write.json` (512 bytes raw)
- **Source**: `/tmp/claude-context-m0/pre-write/1777278799-926-10180-stdin.json` (Edit of hello.txt)
- **Why chosen**: Edit selected over Write because Edit's `tool_input` contains more interesting fields (`old_string`, `new_string`, `replace_all`) — exercises more of the schema for parsing tests.
- **Sanitized**: same as above.

### `post-write.json` (879 bytes raw)
- **Source**: `/tmp/claude-context-m0/post-write/1777278799-934-13814-stdin.json` (PostToolUse:Edit)
- **Why chosen**: Largest sample; includes full `structuredPatch` array — important for M3's edit-summary classifier which will parse the diff structure.
- **Sanitized**: same.

### `session-start.json` (316 bytes raw)
- **Source**: only one sample available (sole SessionStart event in the session)
- **Why chosen**: forced choice. Documents the `source` and `model` fields per R1.
- **Sanitized**: same.

### `stop.json` (752 bytes raw)
- **Source**: `/tmp/claude-context-m0/stop/1777278825-1114-9097-stdin.json` (after passphrase-recall response)
- **Why chosen**: Largest sample; demonstrates the `last_assistant_message` field with realistic multiline content (passphrases + Markdown formatting). M3 will likely consume this field.
- **Sanitized**: same.

All fixtures verified as valid JSON with no `/Users/ranwei` or `ranwei` substrings (PII grep clean — see commit `f910de5`).
