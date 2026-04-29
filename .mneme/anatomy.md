<!-- mneme anatomy v1 -->
<!-- generated: 2026-04-28T12:20:22Z | files: 397 | project: d38b7436-61e0-4908-898e-3cc1fb2f309b -->

## ./

- `.gitignore` — Git ignore rules (config, ~11 tok)
- `AGENTS.md` — Mneme Go - Agent Instructions (markdown, ~328 tok)
- `CLAUDE.md` — Claude Code project instructions (config, ~934 tok)
- `Makefile` — Build rules (config, ~139 tok)
- `README.md` — Mneme - Go Edition (markdown, ~2803 tok)
- `go.mod` — Go module definition (config, ~457 tok)
- `go.sum` — Go dependency checksums (config, ~2921 tok)

## .claude/

- `settings.local.json` — { (unknown, ~64 tok)

## .claude/worktrees/agent-a8031b1aba8b55820/

- `.gitignore` — Git ignore rules (config, ~4 tok)
- `AGENTS.md` — Claude Context Go - Agent Instructions (markdown, ~334 tok)
- `CLAUDE.md` — Claude Code project instructions (config, ~965 tok)
- `README.md` — Claude Context - Go Edition (markdown, ~1081 tok)
- `go.mod` — Go module definition (config, ~346 tok)
- `go.sum` — Go dependency checksums (config, ~2336 tok)

## .claude/worktrees/agent-a8031b1aba8b55820/.claude/

- `settings.local.json` — { (unknown, ~64 tok)

## .claude/worktrees/agent-a8031b1aba8b55820/cmd/mcp/

- `main.go` — initLogger configures mlog to write to stderr and sets verbosity from LOG_LEVEL. (go, ~661 tok)

## .claude/worktrees/agent-a8031b1aba8b55820/docs/

- `DEPLOYMENT.md` — Deployment Guide (markdown, ~331 tok)
- `m0-runbook.md` — M0 Hook Protocol Validation — Runbook (markdown, ~1425 tok)

## .claude/worktrees/agent-a8031b1aba8b55820/docs/superpowers/plans/

- `2026-04-23-go-refactor.md` — Claude Context Go Refactoring Implementation Plan (markdown, ~12786 tok)
- `2026-04-24-multi-backend-vectordb.md` — Multi-Backend Vector Database Implementation Plan (markdown, ~6820 tok)
- `2026-04-27-m0-hook-protocol-validation.md` — M0 Hook Protocol Validation Implementation Plan (markdown, ~11999 tok)

## .claude/worktrees/agent-a8031b1aba8b55820/docs/superpowers/specs/

- `2026-04-23-go-refactor-design.md` — Claude Context Go Refactoring Design (markdown, ~2980 tok)
- `2026-04-24-multi-backend-vectordb-design.md` — Multi-Backend Vector Database Design (markdown, ~2064 tok)
- `2026-04-27-claude-context-hook-architecture-design.md` — Claude-Context Hook Architecture Design (markdown, ~8298 tok)
- `2026-04-27-m0-hook-protocol-validation-design.md` — M0 — Hook Protocol Validation Spike Design (markdown, ~4383 tok)
- `2026-04-27-m0-hook-protocol-validation-report.md` — M0 Hook Protocol Validation Report (markdown, ~3694 tok)
- `2026-04-27-m1-foundation-design.md` — M1 — Foundation Design (markdown, ~8188 tok)

## .claude/worktrees/agent-a8031b1aba8b55820/pkg/

- `types.go` — type Vector struct { (go, ~487 tok)

## .claude/worktrees/agent-a8031b1aba8b55820/pkg/config/

- `config.go` — type Config struct { (go, ~964 tok)

## .claude/worktrees/agent-a8031b1aba8b55820/pkg/context/

- `indexer.go` — type Indexer struct { (go, ~1487 tok)
- `searcher.go` — type Searcher struct { (go, ~534 tok)

## .claude/worktrees/agent-a8031b1aba8b55820/pkg/embedding/

- `client.go` — type CachedClient struct { (go, ~507 tok)
- `qwen.go` — type QwenProvider struct { (go, ~1277 tok)
- `siliconflow.go` — type SiliconFlowProvider struct { (go, ~890 tok)

## .claude/worktrees/agent-a8031b1aba8b55820/pkg/mcp/

- `resources.go` — func (s *Server) handleResourceRead(ctx context.Context, params json.RawMessage) interface{} { (go, ~504 tok)
- `server.go` — type Server struct { (go, ~2032 tok)
- `types.go` — type MCPRequest struct { (go, ~276 tok)

## .claude/worktrees/agent-a8031b1aba8b55820/pkg/splitter/

- `ast.go` — type ASTSplitter struct{} (go, ~3768 tok)

## .claude/worktrees/agent-a8031b1aba8b55820/pkg/vectordb/

- `chromem.go` — type ChromemStore struct { (go, ~2834 tok)
- `factory.go` — NewStoreFromConfig selects and constructs the vector store backend from config. (go, ~167 tok)
- `qdrant.go` — type QdrantStore struct { (go, ~2561 tok)
- `store.go` — type DuckDBStore struct { (go, ~1423 tok)

## .claude/worktrees/agent-a8031b1aba8b55820/scripts/hook-protocol-probe/

- `README.md` — Hook Protocol Probe Scripts (markdown, ~559 tok)
- `cleanup.sh` — set -euo pipefail (unknown, ~727 tok)
- `echo.sh` — set -euo pipefail (unknown, ~92 tok)
- `exit-n.sh` — set -euo pipefail (unknown, ~79 tok)
- `setup-sandbox.sh` — set -euo pipefail (unknown, ~1107 tok)
- `sleep-n.sh` — set -euo pipefail (unknown, ~87 tok)
- `swap-hook.sh` — set -euo pipefail (unknown, ~337 tok)

## .claude/worktrees/agent-a8031b1aba8b55820/tests/

- `config_backend_test.go` — func TestConfigBackendDefaults(t *testing.T) { (go, ~467 tok)
- `config_test.go` — func TestConfigFileOverridesDefaults(t *testing.T) { (go, ~561 tok)
- `embedding_test.go` — type MockEmbeddingProvider struct { (go, ~1123 tok)
- `indexer_test.go` — func TestIndexerIndex(t *testing.T) { (go, ~1047 tok)
- `integration_test.go` — func TestEndToEndIndexAndSearch(t *testing.T) { (go, ~320 tok)
- `mcp_test.go` — func TestMCPServerInit(t *testing.T) { (go, ~732 tok)
- `qwen_test.go` — func TestQwenProviderInit(t *testing.T) { (go, ~68 tok)
- `searcher_test.go` — type MockSearchStore struct{} (go, ~360 tok)
- `siliconflow_test.go` — func TestSiliconFlowProviderInit(t *testing.T) { (go, ~74 tok)
- `splitter_test.go` — func TestSplitGoCode(t *testing.T) { (go, ~1590 tok)
- `vectordb_chromem_test.go` — func TestChromemStoreInitialize(t *testing.T) { (go, ~1432 tok)
- `vectordb_factory_test.go` — func TestFactoryDefaultsToDuckDB(t *testing.T) { (go, ~447 tok)
- `vectordb_qdrant_test.go` — func qdrantURL(t *testing.T) string { (go, ~590 tok)
- `vectordb_test.go` — func TestStoreInitialize(t *testing.T) { (go, ~558 tok)

## .claude/worktrees/agent-a8031b1aba8b55820/tests/fixtures/hook-payloads/

- `post-write.json` — { (unknown, ~248 tok)
- `pre-read.json` — { (unknown, ~92 tok)
- `pre-write.json` — { (unknown, ~113 tok)
- `session-start.json` — { (unknown, ~55 tok)
- `stop.json` — { (unknown, ~165 tok)

## .mneme/

- `.gitignore` — Git ignore rules (config, ~5 tok)
- `.local-id` — d38b7436-61e0-4908-898e-3cc1fb2f309b (unknown, ~9 tok)
- `.local-id.lock` — (no description) (unknown, ~0 tok)
- `anatomy.md` — <!-- mneme anatomy v1 --> (markdown, ~4412 tok)
- `buglog.json` — { (unknown, ~9244 tok)
- `buglog.lock` — (no description) (unknown, ~0 tok)

## cmd/

- `cmd_buglog.go` — func dispatchBuglog(args []string) { (go, ~1386 tok)
- `cmd_cerebrum.go` — func dispatchCerebrum(args []string) { (go, ~1834 tok)
- `cmd_daemon.go` — func dispatchDaemon(args []string) { (go, ~1224 tok)
- `cmd_dashboard.go` — func dispatchDashboard(args []string) { (go, ~135 tok)
- `cmd_designqc.go` — type stringSlice []string (go, ~583 tok)
- `cmd_hook.go` — func dispatchHook(args []string) { (go, ~1125 tok)
- `cmd_init.go` — type initOpts struct { (go, ~1653 tok)
- `cmd_memory.go` — func dispatchMemory(args []string) { (go, ~235 tok)
- `cmd_report.go` — func dispatchReport(args []string) { (go, ~361 tok)
- `cmd_restore.go` — func dispatchRestore(args []string) { (go, ~603 tok)
- `cmd_scan.go` — func dispatchScan(args []string) { (go, ~711 tok)
- `cmd_stats.go` — type statsReport struct { (go, ~1042 tok)
- `cmd_status.go` — type daemonStatus struct { (go, ~826 tok)
- `cmd_suggestions.go` — func dispatchSuggestions(args []string) { (go, ~550 tok)
- `cmd_uninstall.go` — func dispatchUninstall(opts initOpts) { (go, ~701 tok)
- `cmd_update.go` — func dispatchUpdate(args []string) { (go, ~683 tok)
- `cmd_version.go` — func printVersion() { (go, ~21 tok)
- `hook_postwrite.go` — func runPostToolUse(stdin io.Reader) { (go, ~914 tok)
- `hook_prewrite.go` — func runPreWrite(stdin io.Reader) { (go, ~685 tok)
- `hook_publish.go` — publishHookFired is best-effort. If the daemon is not running the call (go, ~193 tok)
- `hook_sessionstart.go` — func runSessionStart(stdin io.Reader) { (go, ~881 tok)
- `hook_stop.go` — func runStop(stdin io.Reader) { (go, ~638 tok)
- `main.go` — Build-time variables. Override via: (go, ~323 tok)
- `mcp_server.go` — func initLogger(logLevel string) { (go, ~622 tok)
- `usage.go` — const version = "0.1.0-m4b" (go, ~380 tok)

## docs/

- `DEPLOYMENT.md` — Deployment Guide (markdown, ~322 tok)
- `m0-runbook.md` — M0 Hook Protocol Validation — Runbook (markdown, ~1398 tok)
- `m1-smoke-checklist.md` — M1 Manual Smoke Checklist (markdown, ~483 tok)

## docs/superpowers/plans/

- `2026-04-23-go-refactor.md` — Mneme Go Refactoring Implementation Plan (markdown, ~12645 tok)
- `2026-04-24-multi-backend-vectordb.md` — Multi-Backend Vector Database Implementation Plan (markdown, ~6768 tok)
- `2026-04-27-m0-hook-protocol-validation.md` — M0 Hook Protocol Validation Implementation Plan (markdown, ~11801 tok)
- `2026-04-27-m1-foundation.md` — M1 — Foundation Implementation Plan (markdown, ~20702 tok)
- `2026-04-27-m2-anatomy.md` — M2 Anatomy Map Implementation Plan (markdown, ~15769 tok)
- `2026-04-27-m3-memory.md` — M3: Memory + Ledger + Edit Summary Implementation Plan (markdown, ~13521 tok)
- `2026-04-28-m10a-dashboard-scaffold.md` — M10a — Dashboard Backend & Scaffold Implementation Plan (markdown, ~13334 tok)
- `2026-04-28-m10b-rest-sse-panels.md` — M10b — REST + SSE + 3 Panels Implementation Plan (markdown, ~24678 tok)
- `2026-04-28-m10c-remaining-panels.md` — M10c — Remaining Dashboard Panels Implementation Plan (markdown, ~22027 tok)
- `2026-04-28-m11-designqc-reframe.md` — M11 — Design QC + Reframe Implementation Plan (markdown, ~17048 tok)
- `2026-04-28-m4a-cerebrum.md` — M4a: Cerebrum Rules Engine Implementation Plan (markdown, ~7101 tok)
- `2026-04-28-m4b-buglog.md` — M4b: Buglog Implementation Plan (markdown, ~8449 tok)
- `2026-04-28-m4c-mcp-local-tools.md` — M4c: MCP Local-State Tools Implementation Plan (markdown, ~6272 tok)
- `2026-04-28-m5-diagnostics.md` — M5: Diagnostics Implementation Plan (markdown, ~6070 tok)
- `2026-04-28-m6-maintenance.md` — M6: Maintenance Implementation Plan (markdown, ~6719 tok)
- `2026-04-28-m7-cli-maintenance.md` — M7: CLI Maintenance Suite — Implementation Plan (markdown, ~20988 tok)
- `2026-04-28-m8-daemon.md` — M8 — Daemon + Cron Scheduler — Implementation Plan (markdown, ~27865 tok)
- `2026-04-28-m9-intelligence-loop.md` — M9 — Intelligence Loop Implementation Plan (markdown, ~24209 tok)

## docs/superpowers/specs/

- `2026-04-23-go-refactor-design.md` — Mneme Go Refactoring Design (markdown, ~2962 tok)
- `2026-04-24-multi-backend-vectordb-design.md` — Multi-Backend Vector Database Design (markdown, ~2053 tok)
- `2026-04-27-m0-hook-protocol-validation-design.md` — M0 — Hook Protocol Validation Spike Design (markdown, ~4318 tok)
- `2026-04-27-m0-hook-protocol-validation-report.md` — M0 Hook Protocol Validation Report (markdown, ~3667 tok)
- `2026-04-27-m1-foundation-design.md` — M1 — Foundation Design (markdown, ~8041 tok)
- `2026-04-27-m2-anatomy-design.md` — M2 — Anatomy Map Design Spec (markdown, ~4332 tok)
- `2026-04-27-m3-memory-design.md` — M3: Memory + Ledger + Edit Summary — Design Spec (markdown, ~2390 tok)
- `2026-04-27-mneme-hook-architecture-design.md` — Mneme Hook Architecture Design (markdown, ~8129 tok)
- `2026-04-28-m10a-dashboard-scaffold-design.md` — M10a — Dashboard Backend & Scaffold — Design Spec (markdown, ~5729 tok)
- `2026-04-28-m10b-rest-sse-panels-design.md` — M10b — REST + SSE + 3 Panels — Design Spec (markdown, ~5727 tok)
- `2026-04-28-m10c-remaining-panels-design.md` — M10c — Remaining Dashboard Panels — Design Spec (markdown, ~5086 tok)
- `2026-04-28-m11-designqc-reframe-design.md` — M11 — Design QC + Reframe — Design Spec (markdown, ~6359 tok)
- `2026-04-28-m4a-cerebrum-design.md` — M4a: Cerebrum Rules Engine — Design Spec (markdown, ~2478 tok)
- `2026-04-28-m4b-buglog-design.md` — M4b: Buglog — Design Spec (markdown, ~2351 tok)
- `2026-04-28-m4c-mcp-local-tools-design.md` — M4c: MCP Local-State Tools — Design Spec (markdown, ~2635 tok)
- `2026-04-28-m7-m11-roadmap-design.md` — M7–M11: openwolf Feature-Parity Roadmap — Design Spec (markdown, ~6305 tok)
- `2026-04-28-m8-daemon-design.md` — M8 — Daemon + Cron Scheduler — Design Spec (markdown, ~6833 tok)
- `2026-04-28-m9-intelligence-loop-design.md` — M9 — Intelligence Loop — Design Spec (markdown, ~5500 tok)

## pkg/

- `types.go` — type Vector struct { (go, ~487 tok)

## pkg/cerebrum/

- `learner.go` — const ( (go, ~966 tok)
- `pending.go` — const pendingLockTimeout = 5 * time.Second (go, ~1177 tok)
- `triggers.go` — TriggerPhrases lists hardcoded user-message keywords flagging a possible (go, ~215 tok)

## pkg/classifier/

- `classifier.go` — ClassifyEdit returns one of 13 category strings for a file edit. (go, ~885 tok)

## pkg/config/

- `config.go` — type Config struct { (go, ~1709 tok)

## pkg/consolidator/

- `memory.go` — const staleDays = 7 (go, ~734 tok)

## pkg/context/

- `indexer.go` — type Indexer struct { (go, ~1480 tok)
- `searcher.go` — type Searcher struct { (go, ~529 tok)

## pkg/daemon/

- `cerebrum_handler.go` — const cerebrumQueueCapacity = 100 (go, ~891 tok)
- `daemon.go` — RunOptions configures a daemon process. (go, ~1018 tok)
- `heartbeat.go` — WriteHeartbeat atomically writes ~/.mneme/daemon/heartbeat.json. (go, ~372 tok)
- `install.go` — Loader runs an external command (launchctl, systemctl). Tests inject a spy. (go, ~1117 tok)
- `log.go` — Logger is the daemon log interface used by scheduler/server/handlers. (go, ~872 tok)
- `middleware.go` — Transport identifies which listener served a request. (go, ~885 tok)
- `routes.go` — RouteDeps wires handler dependencies. (go, ~2264 tok)
- `scheduler.go` — CancelFn cancels a pending retry. Returns true if cancellation prevented the (go, ~1579 tok)
- `server.go` — ServerConfig wires up the listeners and middleware. (go, ~594 tok)
- `signals_unix.go` — runReloadHandler watches SIGHUP and SIGUSR1 and dispatches to callbacks. (go, ~146 tok)
- `signals_windows.go` — runReloadHandler is a no-op on Windows. (go, ~44 tok)
- `state.go` — Manifest is the user-editable cron manifest at ~/.mneme/daemon/cron-manifest.json. (go, ~722 tok)
- `tasks.go` — TaskFunc is the signature of a registered cron task. (go, ~1949 tok)

## pkg/daemonclient/

- `client.go` — ErrUnavailable signals the daemon is not reachable. (go, ~856 tok)

## pkg/dashboard/

- `anatomy.go` — type AnatomyFile struct { (go, ~403 tok)
- `api.go` — APIDeps is the cross-cutting set of dependencies the API handlers need. (go, ~1264 tok)
- `buglog.go` — func BugLogHandler() http.HandlerFunc { (go, ~390 tok)
- `cerebrum.go` — resolveProject reads ?project= from r, looks up the project root, (go, ~1169 tok)
- `cli.go` — type CLIDeps struct { (go, ~695 tok)
- `designqc.go` — DesignQCHandler reads ~/.mneme/designqc/<project-id>/report.json. When the (go, ~825 tok)
- `dev_token.go` — DevTokenHandler sets the mneme_token cookie when enabled. (go, ~368 tok)
- `embed.go` — go:embed dist/* (go, ~208 tok)
- `exec.go` — func execCommand(bin string, args ...string) error { (go, ~35 tok)
- `memory.go` — type MemoryRow struct { (go, ~469 tok)
- `server.go` — Deps is reserved for future composition; intentionally empty in M10a. (go, ~348 tok)
- `sse.go` — const ssePingInterval = 25 * time.Second (go, ~714 tok)
- `suggestions.go` — func SuggestionsHandler() http.HandlerFunc { (go, ~407 tok)
- `token.go` — const tokenHistoryCap = 12 (go, ~218 tok)

## pkg/dashboard/dist/

- `index.html` — <!doctype html> (unknown, ~115 tok)

## pkg/dashboard/dist/assets/

- `index-6ejTlmJS.css` — (no description) (unknown, ~3555 tok)
- `index-6z0wtNh8.js` — var U0=Object.defineProperty;var w0=(n,r,f)=>r in n?U0(n,r,{enumerable:!0,configurable:!0,writable:! (js, ~77829 tok)

## pkg/designqc/

- `capture.go` — type CaptureOptions struct { (go, ~846 tok)
- `detect.go` — type Framework struct { (go, ~227 tok)
- `report.go` — const ReportVersion = 1 (go, ~341 tok)
- `routes.go` — type Route struct { (go, ~447 tok)
- `runner.go` — type RunOptions struct { (go, ~745 tok)

## pkg/embedding/

- `client.go` — type CachedClient struct { (go, ~505 tok)
- `qwen.go` — type QwenProvider struct { (go, ~1277 tok)
- `siliconflow.go` — type SiliconFlowProvider struct { (go, ~890 tok)

## pkg/envcheck/

- `envcheck.go` — type Result struct { (go, ~502 tok)

## pkg/events/

- `bus.go` — Logger is the minimal sink for write errors and rotation events. (go, ~861 tok)
- `event.go` — Event is the canonical envelope for every dashboard event. (go, ~203 tok)
- `jsonl.go` — type jsonlWriter struct { (go, ~866 tok)
- `ring.go` — const ringCapacity = 500 (go, ~243 tok)

## pkg/hook/

- `feedback.go` — const prefix = "⚡ mneme: " (go, ~138 tok)
- `protocol.go` — type Event struct { (go, ~849 tok)

## pkg/installer/

- `backup.go` — BackupOp describes a destructive operation about to occur. (go, ~855 tok)
- `identity.go` — go:embed templates/identity.md.tmpl (go, ~1235 tok)
- `mnememd.go` — go:embed templates/mneme.md.tmpl (go, ~428 tok)
- `project.go` — const gitignoreContent = "_session.json\n*.bak.*\n" (go, ~169 tok)
- `reframe.go` — go:embed templates/reframe.md.tmpl (go, ~114 tok)
- `rules.go` — go:embed templates/rules.md (go, ~476 tok)
- `settings.go` — type hookEntry struct { (go, ~948 tok)
- `uninstall.go` — UninstallOpts configures the uninstall operation. (go, ~287 tok)

## pkg/installer/templates/

- `identity.md.tmpl` — <!-- managed by mneme — auto-generated; do not edit manually --> (unknown, ~83 tok)
- `mneme.md.tmpl` — <!-- managed by mneme — to update: mneme update --> (unknown, ~371 tok)
- `reframe.md.tmpl` — <!-- mneme reframe v1 --> (unknown, ~1464 tok)
- `rules.md` — <!-- managed by mneme — do not edit manually --> (markdown, ~409 tok)

## pkg/match/

- `tokens.go` — var nonAlnum = regexp.MustCompile(`[^a-zA-Z0-9]+`) (go, ~269 tok)

## pkg/mcp/

- `local_tools.go` — type localArgs struct { (go, ~1306 tok)
- `resources.go` — func (s *Server) handleResourceRead(ctx context.Context, params json.RawMessage) interface{} { (go, ~504 tok)
- `server.go` — type Server struct { (go, ~2625 tok)
- `types.go` — type MCPRequest struct { (go, ~276 tok)

## pkg/scanner/

- `ext_go.go` — func extractGo(data []byte) string { (go, ~208 tok)
- `ext_known.go` — var knownFiles = map[string]string{ (go, ~260 tok)
- `extractor.go` — FileEntry holds the result of scanning a single file. (go, ~1473 tok)
- `walker.go` — Walk returns project-relative file paths using git ls-files. (go, ~263 tok)

## pkg/splitter/

- `ast.go` — type ASTSplitter struct{} (go, ~3766 tok)

## pkg/state/

- `anatomy.go` — AnatomyEntry is the state package's representation of a scanned file. (go, ~1066 tok)
- `atomic.go` — AtomicWrite writes data to path via a temp file + fsync + rename. (go, ~114 tok)
- `buglog.go` — BuglogEntry records a previously fixed bug for re-introduction detection. (go, ~656 tok)
- `cerebrum.go` — type CerebrumRule struct { (go, ~753 tok)
- `daemon_paths.go` — DaemonDir returns ~/.mneme/daemon for the given home directory. (go, ~363 tok)
- `ledger.go` — type LedgerTotals struct { (go, ~1220 tok)
- `ledger_history.go` — const ledgerHistoryMaxBytes = 1 << 20 // 1 MiB (go, ~629 tok)
- `lock.go` — AcquireLock acquires an exclusive file lock on lockPath with a timeout. (go, ~201 tok)
- `memory.go` — type FileStat struct { (go, ~850 tok)
- `origin.go` — WriteOrigin records the absolute path of a project's working tree to (go, ~417 tok)
- `paths.go` — FindGitRoot runs git rev-parse to find the repository root from startDir. (go, ~419 tok)
- `session.go` — type TurnEdit struct { (go, ~1243 tok)
- `template_version.go` — const TemplateVersion = 1 // bump when bundled templates change shape (go, ~370 tok)

## pkg/suggestions/

- `dismissed.go` — const dismissedLockTimeout = 5 * time.Second (go, ~462 tok)
- `engine.go` — const refreshLockTimeout = 5 * time.Second (go, ~579 tok)
- `generators.go` — func newID(typ, target string) string { (go, ~591 tok)

## pkg/updater/

- `release.go` — const DefaultGitHubAPIBase = "https://api.github.com" (go, ~926 tok)
- `updater.go` — type SyncOptions struct { (go, ~597 tok)

## pkg/vectordb/

- `chromem.go` — type ChromemStore struct { (go, ~2831 tok)
- `factory.go` — NewStoreFromConfig selects and constructs the vector store backend from config. (go, ~163 tok)
- `qdrant.go` — type QdrantStore struct { (go, ~2559 tok)
- `store.go` — type DuckDBStore struct { (go, ~1420 tok)

## pkg/waste/

- `detector.go` — const ( (go, ~974 tok)
- `report.go` — type ReportInput struct { (go, ~878 tok)

## scripts/hook-protocol-probe/

- `README.md` — Hook Protocol Probe Scripts (markdown, ~550 tok)
- `cleanup.sh` — set -euo pipefail (unknown, ~698 tok)
- `echo.sh` — set -euo pipefail (unknown, ~89 tok)
- `exit-n.sh` — set -euo pipefail (unknown, ~77 tok)
- `setup-sandbox.sh` — set -euo pipefail (unknown, ~1076 tok)
- `sleep-n.sh` — set -euo pipefail (unknown, ~87 tok)
- `swap-hook.sh` — set -euo pipefail (unknown, ~335 tok)

## tests/

- `anatomy_timestamp_test.go` — func TestReadAnatomyGeneratedTime(t *testing.T) { (go, ~243 tok)
- `backup_test.go` — func TestBackupAndListAndRestore(t *testing.T) { (go, ~397 tok)
- `bench_test.go` — func BenchmarkParseEvent(b *testing.B) { (go, ~427 tok)
- `bug_search_test.go` — func seedBuglog(t *testing.T, dir string) { (go, ~506 tok)
- `buglog_test.go` — func TestReadBuglogEmpty(t *testing.T) { (go, ~636 tok)
- `cerebrum_handler_test.go` — func TestCerebrumLearnHandlerEnqueues(t *testing.T) { (go, ~522 tok)
- `cerebrum_m9_test.go` — func TestScanTextEnglish(t *testing.T) { (go, ~1350 tok)
- `cerebrum_test.go` — func TestReadCerebrumEmpty(t *testing.T) { (go, ~1130 tok)
- `classifier_test.go` — func TestClassifyEdit(t *testing.T) { (go, ~464 tok)
- `cmd_dashboard_test.go` — func TestDashboardPreflightAllOK(t *testing.T) { (go, ~521 tok)
- `cmd_restore_test.go` — func TestRestoreList(t *testing.T) { (go, ~470 tok)
- `cmd_status_test.go` — func buildMnemeBinary(t *testing.T) string { (go, ~542 tok)
- `cmd_update_test.go` — func TestUpdateCLISyncs(t *testing.T) { (go, ~660 tok)
- `config_backend_test.go` — func TestConfigBackendDefaults(t *testing.T) { (go, ~458 tok)
- `config_test.go` — func TestConfigFileOverridesDefaults(t *testing.T) { (go, ~1424 tok)
- `consolidator_test.go` — func setupConsolidatorHome(t *testing.T) string { (go, ~744 tok)
- `daemon_client_test.go` — func TestClient_TryDial_NoSocketReturnsErrUnavailable(t *testing.T) { (go, ~402 tok)
- `daemon_heartbeat_test.go` — func TestHeartbeat_WritesOnStart(t *testing.T) { (go, ~497 tok)
- `daemon_install_test.go` — func TestRenderLaunchdPlist(t *testing.T) { (go, ~539 tok)
- `daemon_log_test.go` — func TestLogger_WritesLine(t *testing.T) { (go, ~658 tok)
- `daemon_middleware_test.go` — func TestRecoverMW_CatchesPanic(t *testing.T) { (go, ~653 tok)
- `daemon_routes_test.go` — func TestRoutes_Health(t *testing.T) { (go, ~1303 tok)
- `daemon_scheduler_test.go` — func TestScheduler_RunOnceSuccess(t *testing.T) { (go, ~923 tok)
- `daemon_state_test.go` — func TestDaemonPaths(t *testing.T) { (go, ~1009 tok)
- `daemon_tasks_test.go` — type stubLogger struct { (go, ~654 tok)
- `daemonclient_publish_test.go` — func TestPublishEvent_RoundTrip(t *testing.T) { (go, ~365 tok)
- `dashboard_anatomy_test.go` — func TestAPI_Anatomy_GroupsByDir(t *testing.T) { (go, ~411 tok)
- `dashboard_api_test.go` — func TestAPI_Overview_EmptyHome(t *testing.T) { (go, ~1483 tok)
- `dashboard_buglog_test.go` — func TestAPI_BugLog_GET(t *testing.T) { (go, ~721 tok)
- `dashboard_cerebrum_test.go` — func seedProject(t *testing.T, home, projectID string) string { (go, ~1195 tok)
- `dashboard_designqc_live_test.go` — func TestAPI_DesignQC_NoReport(t *testing.T) { (go, ~1571 tok)
- `dashboard_designqc_test.go` — func TestAPI_DesignQC_Stub(t *testing.T) { (go, ~259 tok)
- `dashboard_memory_test.go` — func TestAPI_Memory_ParsesRows(t *testing.T) { (go, ~511 tok)
- `dashboard_sse_test.go` — func TestSSE_DeliversEvents(t *testing.T) { (go, ~693 tok)
- `dashboard_suggestions_test.go` — func writeSuggestionsFile(t *testing.T, projectID string, entries []suggestions.Suggestion) { (go, ~660 tok)
- `dashboard_test.go` — func TestFSContainsIndex(t *testing.T) { (go, ~715 tok)
- `dashboard_token_test.go` — func TestAPI_Token_TotalsAndHistory(t *testing.T) { (go, ~331 tok)
- `designqc_capture_test.go` — const fixtureHTML = `<!doctype html><html><head><title>fix</title></head> (go, ~436 tok)
- `designqc_detect_test.go` — func TestDetectFramework_DefaultPort(t *testing.T) { (go, ~388 tok)
- `designqc_report_test.go` — func TestReport_RoundTrip(t *testing.T) { (go, ~352 tok)
- `designqc_routes_test.go` — func TestEnumerateRoutes_FromRouter(t *testing.T) { (go, ~463 tok)
- `designqc_runner_test.go` — type fakeCapturer struct { (go, ~729 tok)
- `embedding_test.go` — type MockEmbeddingProvider struct { (go, ~1121 tok)
- `envcheck_test.go` — func TestDetectPackageManagerPnpm(t *testing.T) { (go, ~414 tok)
- `events_bus_test.go` — func TestBus_PublishSubscribe(t *testing.T) { (go, ~742 tok)
- `events_jsonl_test.go` — func TestBus_PersistsToJSONL(t *testing.T) { (go, ~862 tok)
- `hook_test.go` — func TestParseEvent_PreRead(t *testing.T) { (go, ~764 tok)
- `identity_test.go` — func TestDetectProjectMetadataGo(t *testing.T) { (go, ~818 tok)
- `indexer_test.go` — func TestIndexerIndex(t *testing.T) { (go, ~1040 tok)
- `installer_reframe_test.go` — func TestInstallReframe_CreatesFile(t *testing.T) { (go, ~314 tok)
- `installer_test.go` — func TestScaffoldProject(t *testing.T) { (go, ~1631 tok)
- `integration_test.go` — func TestEndToEndIndexAndSearch(t *testing.T) { (go, ~311 tok)
- `ledger_history_test.go` — func TestAppendLedgerHistory(t *testing.T) { (go, ~705 tok)
- `match_test.go` — func TestTokenizeStripsStopWords(t *testing.T) { (go, ~399 tok)
- `mcp_local_tools_test.go` — func newTestServer() *mcp.Server { (go, ~2003 tok)
- `mcp_test.go` — func TestMCPServerInit(t *testing.T) { (go, ~728 tok)
- `origin_test.go` — func TestWriteAndReadOrigin(t *testing.T) { (go, ~411 tok)
- `qwen_test.go` — func TestQwenProviderInit(t *testing.T) { (go, ~66 tok)
- `release_test.go` — func TestFetchReleaseAssetMatchesPlatform(t *testing.T) { (go, ~759 tok)
- `scan_check_test.go` — func TestScanCheckCleanProject(t *testing.T) { (go, ~746 tok)
- `scanner_incremental_test.go` — func TestScanProjectIncrementalSkipsUnchanged(t *testing.T) { (go, ~671 tok)
- `scanner_test.go` — func makeGitRepo(t *testing.T) string { (go, ~2447 tok)
- `scheduler_publish_test.go` — func TestScheduler_PublishesCronTick(t *testing.T) { (go, ~537 tok)
- `searcher_test.go` — type MockSearchStore struct{} (go, ~353 tok)
- `siliconflow_test.go` — func TestSiliconFlowProviderInit(t *testing.T) { (go, ~72 tok)
- `splitter_test.go` — func TestSplitGoCode(t *testing.T) { (go, ~1588 tok)
- `state_test.go` — func TestAtomicWrite(t *testing.T) { (go, ~4086 tok)
- `template_version_test.go` — func TestReadTemplateVersionMissing(t *testing.T) { (go, ~261 tok)
- `updater_test.go` — func setupProject(t *testing.T, home, projectName string, pinnedVersion int) string { (go, ~640 tok)
- `vectordb_chromem_test.go` — func TestChromemStoreInitialize(t *testing.T) { (go, ~1427 tok)
- `vectordb_factory_test.go` — func TestFactoryDefaultsToDuckDB(t *testing.T) { (go, ~443 tok)
- `vectordb_qdrant_test.go` — func qdrantURL(t *testing.T) string { (go, ~586 tok)
- `vectordb_test.go` — func TestStoreInitialize(t *testing.T) { (go, ~554 tok)
- `waste_report_test.go` — func TestGenerateAndRenderReport(t *testing.T) { (go, ~834 tok)
- `waste_test.go` — setupWasteProject returns a temp project root (with .mneme/) and a temp home dir. (go, ~2269 tok)

## tests/fixtures/hook-payloads/

- `post-tool-use-write.json` — { (unknown, ~113 tok)
- `post-write.json` — { (unknown, ~242 tok)
- `pre-read.json` — { (unknown, ~87 tok)
- `pre-write.json` — { (unknown, ~108 tok)
- `session-start.json` — { (unknown, ~53 tok)
- `stop.json` — { (unknown, ~158 tok)

## tests/golden/

- `anatomy_sample.md` — <!-- mneme anatomy v1 --> (markdown, ~62 tok)
- `mcp_search_response.json` — {"note": "captured during M1; regenerate with: go test ./tests/ -run TestMCPSearchGolden -update"} (unknown, ~24 tok)

## tests/golden/m9/transcripts/

- `english.jsonl` — {"type":"user","message":{"role":"user","content":"please edit pkg/foo/bar.go to fix the bug"}} (unknown, ~103 tok)
- `no_match.jsonl` — {"type":"user","message":{"role":"user","content":"please add a feature"}} (unknown, ~60 tok)

## tests/integration/

- `daemon_smoke_test.go` — func TestDaemonSmoke(t *testing.T) { (go, ~594 tok)
- `dashboard_panels_test.go` — func TestDashboardPanels_EndToEnd(t *testing.T) { (go, ~860 tok)
- `dashboard_smoke_test.go` — TestDashboardEndToEnd exercises the M10a auth pipeline using an (go, ~844 tok)
- `designqc_e2e_test.go` — const fixtureSPA = `<!doctype html><html><head><title>fix</title></head> (go, ~705 tok)
- `hook_chain_test.go` — func setupInitializedProject(t *testing.T) (projectDir string, homeDir string) { (go, ~6464 tok)
- `init_lifecycle_test.go` — var binaryPath string (go, ~1035 tok)
- `mcp_regression_test.go` — func TestMCPServerStartsWithNoArgs(t *testing.T) { (go, ~427 tok)
- `memory_consolidate_test.go` — func setupMemoryProject(t *testing.T) (projDir, homeDir string) { (go, ~423 tok)
- `scan_incremental_test.go` — func setupScanProject(t *testing.T) string { (go, ~656 tok)
- `stats_flags_test.go` — setupStatsProject creates a temp project with .mneme/ (no git needed). (go, ~801 tok)

## web/

- `.gitignore` — Git ignore rules (config, ~6 tok)
- `README.md` — mneme dashboard frontend (markdown, ~389 tok)
- `dist.placeholder.html` — <!doctype html> (unknown, ~369 tok)
- `index.html` — <!doctype html> (unknown, ~91 tok)
- `package.json` — Node.js package descriptor (config, ~189 tok)
- `pnpm-lock.yaml` — lockfileVersion: '9.0' (unknown, ~21960 tok)
- `tsconfig.json` — TypeScript compiler config (config, ~137 tok)
- `tsconfig.tsbuildinfo` — {"root":["./src/app.tsx","./src/bootstrap.tsx","./src/main.tsx","./src/vite-env.d.ts","./src/__tests (unknown, ~160 tok)
- `vite.config.ts` — export default defineConfig({ (js, ~151 tok)
- `vitest.config.ts` — export default defineConfig({ (js, ~76 tok)

## web/dist/

- `index.html` — <!doctype html> (unknown, ~115 tok)

## web/dist/assets/

- `index-6ejTlmJS.css` — (no description) (unknown, ~3555 tok)
- `index-6z0wtNh8.js` — var U0=Object.defineProperty;var w0=(n,r,f)=>r in n?U0(n,r,{enumerable:!0,configurable:!0,writable:! (js, ~77829 tok)

## web/src/

- `App.tsx` — export function App(): JSX.Element { (js, ~381 tok)
- `Bootstrap.tsx` — export function Bootstrap(): null { (js, ~41 tok)
- `main.tsx` — import { StrictMode } from 'react' (js, ~57 tok)
- `styles.css` — @import "tailwindcss"; (unknown, ~25 tok)
- `vite-env.d.ts` — import type { JSX as ReactJSX } from 'react' (js, ~156 tok)

## web/src/__tests__/

- `App.test.tsx` — import { describe, expect, test, vi, beforeEach } from 'vitest' (js, ~379 tok)
- `ProjectPicker.test.tsx` — function ShowQuery(): JSX.Element { (js, ~393 tok)
- `setup.ts` — class StubEventSource { (js, ~94 tok)

## web/src/api/

- `activity.ts` — export type ActivityArgs = { limit?: number; since?: number; types?: string[]; projectId?: string } (js, ~167 tok)
- `anatomy.ts` — export const getAnatomy = (projectId: string) => (js, ~55 tok)
- `buglog.ts` — export const getBugLog = (projectId: string) => (js, ~97 tok)
- `cerebrum.ts` — export const getCerebrum = (projectId: string) => (js, ~150 tok)
- `client.ts` — export class APIError extends Error { (js, ~165 tok)
- `cron.ts` — export const getCron = () => request<CronResponse>('GET', '/api/cron') (js, ~81 tok)
- `designqc.ts` — export const getDesignQC = (projectId: string) => (js, ~56 tok)
- `memory.ts` — export const getMemory = () => request<MemoryResponse>('GET', '/api/memory') (js, ~39 tok)
- `overview.ts` — export const getOverview = () => request<Overview>('GET', '/api/overview') (js, ~37 tok)
- `projects.ts` — export const getProjects = () => request<ProjectsResponse>('GET', '/api/projects') (js, ~41 tok)
- `suggestions.ts` — export const getSuggestions = (projectId: string) => (js, ~108 tok)
- `token.ts` — export const getToken = (projectId: string) => (js, ~53 tok)
- `types.ts` — export type ProjectSummary = { (js, ~798 tok)

## web/src/components/

- `AppShell.tsx` — export function AppShell(): JSX.Element { (js, ~483 tok)
- `ConfirmButton.tsx` — export function ConfirmButton({ onConfirm, label, confirmLabel = 'Confirm?', className = '' }: Confi (js, ~354 tok)
- `CronTaskRow.tsx` — export function CronTaskRow({ t, onAction }: { t: CronTask; onAction: () => void }): JSX.Element { (js, ~377 tok)
- `EventRow.tsx` — export function EventRow({ e }: { e: Event }): JSX.Element { (js, ~134 tok)
- `HealthCard.tsx` — export function HealthCard(): JSX.Element { (js, ~200 tok)
- `ProjectCard.tsx` — export function ProjectCard({ p }: { p: ProjectSummary }): JSX.Element { (js, ~110 tok)
- `ProjectPicker.tsx` — export function ProjectPicker(): JSX.Element { (js, ~165 tok)
- `Sparkline.tsx` — export function Sparkline({ data, width = 120, height = 30, color = '#2563eb' }: SparklineProps): JS (js, ~181 tok)

## web/src/hooks/

- `useActiveProject.tsx` — export function ActiveProjectProvider({ children }: { children: ReactNode }): JSX.Element { (js, ~395 tok)
- `useFetch.ts` — export type FetchState<T> = { (js, ~236 tok)
- `useProjectList.ts` — export function useProjectList() { (js, ~72 tok)
- `useSSE.tsx` — export function SSEProvider({ children }: { children: ReactNode }): JSX.Element { (js, ~552 tok)

## web/src/panels/

- `Activity.tsx` — export function Activity(): JSX.Element { (js, ~283 tok)
- `Anatomy.tsx` — export function Anatomy(): JSX.Element { (js, ~756 tok)
- `BugLog.tsx` — export function BugLog(): JSX.Element { (js, ~758 tok)
- `Cerebrum.tsx` — export function Cerebrum(): JSX.Element { (js, ~847 tok)
- `Cron.tsx` — export function Cron(): JSX.Element { (js, ~288 tok)
- `DesignQC.tsx` — export function DesignQC(): JSX.Element { (js, ~719 tok)
- `Memory.tsx` — export function Memory(): JSX.Element { (js, ~358 tok)
- `Overview.tsx` — export function Overview(): JSX.Element { (js, ~398 tok)
- `Suggestions.tsx` — export function Suggestions(): JSX.Element { (js, ~464 tok)
- `Token.tsx` — export function Token(): JSX.Element { (js, ~670 tok)
