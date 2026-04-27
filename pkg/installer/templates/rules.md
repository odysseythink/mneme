<!-- managed by claude-context — do not edit manually -->
<!-- to update: claude-context init --yes -->

# claude-context integration

This file is auto-imported by CLAUDE.md to make rules available in every session.

## MCP tools available

- **`index_codebase`** — index a project directory for semantic search. Run once per project.
- **`search_codebase`** — find relevant code chunks before writing. Use before editing unfamiliar code.

## Suggested workflow

1. At the start of work on a new project: `index_codebase` with the repo root.
2. Before writing or refactoring code: `search_codebase` with a description of what you need.
3. After a session: `claude-context stats` (in a terminal) to see usage.

## Hook instrumentation

Five lightweight hooks track context usage. They run silently (exit 0) unless
`CLAUDE_CONTEXT_DEBUG=1` is set, in which case each hook fire is visible in the
Claude transcript as a "Failed with non-blocking status" message.

Hooks: pre-read, pre-write, post-write, session-start, stop.

## Anatomy map

Before reading a file, check if claude-context has already described it:

- If the pre-read hook fires with `⚡ claude-context: <path> — <description> (~N tok)`,
  that description is from the anatomy map. Use it to decide whether to read the full file.
- If the hook says `<path> already read this session`, the file content is already in your
  context window. Do not re-read it unless the content may have changed.

To regenerate the anatomy map after large refactors: `claude-context scan`

## Privacy

All collected data is local. Hooks make no network calls. State lives in:
  ~/.claude-context/projects/<project-id>/

## Uninstall

  claude-context init --uninstall --yes
