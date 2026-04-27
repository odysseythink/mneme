# M0 Hook Protocol Validation — Runbook

Step-by-step guide for executing the M0 spike against a real Claude Code session. Total user time: 30-60 minutes of focused attention. Do not multitask during R3 (latency grading).

**Before starting:**
- Close any running Claude Code sessions on this machine (avoids interference).
- Clear notification noise (you'll be subjectively grading interactivity).
- Have this file open in one window and a fresh terminal in another.

---

## Phase 0 — Setup (1 minute)

```bash
cd <go-claude-context project root>
bash scripts/m0-probe/setup-sandbox.sh
```

**Expected output:** `✓ Sandbox at /tmp/claude-context-m0-sandbox/` and 5 other ✓ lines, ending with `NEXT STEPS:`.

If anything is missing, stop and investigate.

---

## Phase 1 — R1 + R4 + R5 joint capture (10-15 minutes)

This phase captures stdin payloads (R1), tests `_managed_by` field tolerance (R4), and tests `@import` path forms (R5) — all in a single Claude Code session.

```bash
# In a NEW terminal:
cd /tmp/claude-context-m0-sandbox/
claude code
```

Starting Claude Code triggers `SessionStart`. After Claude is ready, **type these prompts one at a time**, waiting for completion before the next:

| # | Prompt | Triggers | Expected dump location |
|---|---|---|---|
| 1 | `read README.md` | PreToolUse:Read | `/tmp/claude-context-m0/pre-read/` |
| 2 | `read auth.go` | PreToolUse:Read (2nd sample) | same |
| 3 | `create a file hello.txt with content 'hi'` | PreToolUse:Write + PostToolUse:Write | `pre-write/` + `post-write/` |
| 4 | `edit hello.txt to say "hello world"` | PreToolUse:Edit + PostToolUse:Edit | same |
| 5 | `read README.md again` | PreToolUse:Read (3rd sample, same file) | `pre-read/` |
| 6 | `do you see any of these passphrases? passphrase-tilde-abc123, passphrase-abs-def456, passphrase-rel-ghi789` | (R5 test, no hook needed) | — |
| 7 | `/exit` | Stop | `stop/` |

**For prompt #6 (R5):** record which passphrases Claude says it sees. The mapping:
- `passphrase-tilde-abc123` ← imported via `@~/claude-context-m0-tilde-test.md`
- `passphrase-abs-def456` ← imported via `@/abs/path/...m0-test-abs.md`
- `passphrase-rel-ghi789` ← imported via `@./.claude/m0-test-rel.md`

Whichever Claude confirms = which `@import` path forms work.

**For R4:** if Claude Code started without complaining about the `_managed_by` field, AND step 1 produced a file under `pre-read/`, R4 passes.

After `/exit`, list the dumps:

```bash
# Back in your original terminal:
ls -la /tmp/claude-context-m0/
ls -la /tmp/claude-context-m0/pre-read/
ls -la /tmp/claude-context-m0/pre-write/
ls -la /tmp/claude-context-m0/post-write/
ls -la /tmp/claude-context-m0/session-start/
ls -la /tmp/claude-context-m0/stop/
```

**Paste the output of all 6 `ls` commands back to the conversation, plus the R5 passphrase findings, plus any R4 anomalies (Claude Code error messages, hook fire failures).**

---

## Phase 2 — R2 (exit code semantics) (10 minutes)

For each exit code in {0, 1, 2, 127}, swap all hooks to `exit-n.sh <code> "marker-<code>"` and run a brief Claude Code session.

```bash
# In your original terminal (sandbox terminal can stay closed):
bash scripts/m0-probe/swap-hook.sh exit-n.sh 0 "marker-zero"
```

```bash
# In a NEW Claude Code session:
cd /tmp/claude-context-m0-sandbox/
claude code
> read README.md
> /exit
```

**Observe and record:**
1. Did Claude proceed with the Read normally? (yes / no / blocked)
2. Did `marker-zero` appear in your terminal stderr? (yes / no)
3. Did Claude react to `marker-zero` in its next response? (yes / no — look at transcript)
4. Any Claude Code error messages?

**Repeat for codes 1, 2, 127** — each time:
```bash
bash scripts/m0-probe/swap-hook.sh exit-n.sh <CODE> "marker-<CODE>"
# new claude code session, same prompts, same observations
```

**Pay special attention to exit 2** — the architecture spec assumes it might block + feed stderr to Claude. We need definitive evidence.

**Paste a 4-row report (one per exit code) back to the conversation.**

---

## Phase 3 — R3 (latency tolerance) (10 minutes)

For each latency in {50, 100, 200, 500, 1000, 3000} ms, swap to `sleep-n.sh <ms>` and start a session. Read a file. Subjectively grade the interaction.

```bash
bash scripts/m0-probe/swap-hook.sh sleep-n.sh 50
# new Claude Code session, prompt: 'read README.md', grade
bash scripts/m0-probe/swap-hook.sh sleep-n.sh 100
# repeat
bash scripts/m0-probe/swap-hook.sh sleep-n.sh 200
# repeat
bash scripts/m0-probe/swap-hook.sh sleep-n.sh 500
# repeat
bash scripts/m0-probe/swap-hook.sh sleep-n.sh 1000
# repeat
bash scripts/m0-probe/swap-hook.sh sleep-n.sh 3000
# repeat — watch for Claude Code timing out
```

**Grading scale (1-5):**
- 5 = no perceptible delay
- 4 = slight delay, fine
- 3 = noticeable but tolerable
- 2 = annoying, would not want this every Read
- 1 = broken / unusable

**Record per latency:**
- Subjective grade
- Whether Claude Code appeared to time out (showed an error, killed the hook, or proceeded without waiting)

**Paste 6 rows (one per latency) back to the conversation.**

---

## Phase 4 — Cleanup (1 minute)

```bash
bash scripts/m0-probe/cleanup.sh
```

**Expected output:** all ✓ marks ending with `✓ All clean.`

If you see any ✗, **paste the cleanup output back** before considering M0 done. We may need to investigate manually.

---

## After Cleanup

The conversation will continue with assistant analysis of your captured data, the M0 report, fixture selection, and any required architecture spec patches. You don't need to do anything else.
