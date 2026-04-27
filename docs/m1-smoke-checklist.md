# M1 Manual Smoke Checklist

**Run this against a real Claude Code session. Not in CI.**

## Prerequisites

1. Build the binary: `go build -o ./bin/claude-context ./cmd`
2. Register with Claude Code (if not already):
   ```bash
   claude mcp add claude-context \
     -e EMBEDDING_API_KEY=$EMBEDDING_API_KEY \
     -- /path/to/bin/claude-context
   ```
3. Choose a throwaway git repository for testing.

## Steps

**1. Install**
```bash
cd <throwaway-git-repo>
/path/to/claude-context init --yes
```
Expected: 4 ✓ lines printed, no errors.

**2. Verify files exist**
```bash
ls ~/.claude/settings.json ~/.claude/claude-context-rules.md ~/.claude/CLAUDE.md
ls .claude-context/.local-id .claude-context/.gitignore
```
Expected: all 5 files exist.

**3. Start Claude Code with debug mode**
```bash
CLAUDE_CONTEXT_DEBUG=1 claude
```

**4. Issue test prompts**
```
> Read the README.md file
> Create a file called hello.txt with content "hello"
> Edit hello.txt to say "hello world"
```

**5. Verify hook activity in Claude transcript**

For each tool use, Claude transcript should show a line like:
```
Failed with non-blocking status code: ⚡ claude-context: M1 stub: pre-read fired (project=<uuid>)
```

Expected hooks per operation:
- Read README.md → `pre-read`
- Create hello.txt → `pre-write`, `post-write`
- Edit hello.txt → `pre-write`, `post-write`

**6. Exit Claude and check stats**
```bash
/exit
cd <throwaway-repo> && /path/to/claude-context stats
```
Expected: non-zero counters for at least `pre-read`, `pre-write`, `post-write`, `session-start`, `stop`.

**7. Uninstall**
```bash
/path/to/claude-context init --uninstall --yes
```
Expected: 3 ✓ lines + 1 ! line about project dir.

**8. Verify clean uninstall**
```bash
diff ~/.claude/settings.json ~/.claude/settings.json.bak.<timestamp>  # should differ only by removed hooks
grep "claude-context-managed" ~/.claude/CLAUDE.md  # should match nothing
```

## Pass criteria

All 8 steps complete without error, and step 5 shows hook fire messages in Claude transcript.
