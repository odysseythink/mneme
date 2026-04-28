export type ProjectSummary = {
  id: string
  origin: string
  anatomy_files: number
  cerebrum_pending: number
  memory_bytes: number
  last_activity_ts: number
}

export type Overview = {
  daemon: { pid: number; uptime_s: number; version: string; started_at: number }
  totals: {
    projects: number
    anatomy_files: number
    cerebrum_pending: number
    open_suggestions: number
  }
  projects: ProjectSummary[]
}

export type Event = {
  ts: number
  type: string
  project_id?: string
  data?: Record<string, unknown>
}

export type ActivityResponse = {
  events: Event[]
  next_cursor: number
}

export type TaskState = {
  last_run: string
  last_success: string
  last_error: string
  consecutive_failures: number
  dead_lettered_at: string
}

export type CronTask = {
  name: string
  schedule: string
  enabled: boolean
  state: TaskState
}

export type CronResponse = { tasks: CronTask[] }
export type ProjectsResponse = { projects: ProjectSummary[] }

export type CerebrumRule = {
  comment: string
  pattern: string
  message: string
}

export type CerebrumTrigger = {
  phrase: string
  user_msg: string
  prior_asst: string
  turn: number
}

export type CerebrumCandidate = {
  id: string
  trigger: CerebrumTrigger
  draft_rule: CerebrumRule
  confidence: number
  queued_at: string
  hit_count: number
}

export type CerebrumResponse = {
  rules: CerebrumRule[]
  pending: CerebrumCandidate[]
}

export type MemoryRow = {
  started_at: string
  turn_count: number
  summary: string
}

export type MemoryResponse = {
  rows: MemoryRow[]
  raw: string
}

export type AnatomyFile = {
  name: string
  description: string
  est_tokens: number
  language: string
}

export type AnatomyDir = {
  path: string
  files: AnatomyFile[]
}

export type AnatomyResponse = {
  directories: AnatomyDir[]
  generated_at: string
}

export type BugLogEntry = {
  id: string
  created_at: string
  source: string
  file: string
  description: string
  bad_code: string
}

export type BugLogResponse = {
  entries: BugLogEntry[]
}

export type SuggestionEntry = {
  id: string
  type: string
  target: string
  title: string
  detail: string
  generated_at: string
}

export type SuggestionsResponse = {
  suggestions: SuggestionEntry[]
}

export type LedgerTotals = {
  hook_fired: Record<string, number>
  hook_errors: number
  stdin_parse_failures: number
  outside_project_skipped: number
  write_skipped: number
  anatomy_hits: number
  repeat_reads: number
  scan_count: number
  edit_patterns?: Record<string, number>
  memory_rows_written?: number
}

export type LedgerSnapshot = {
  ts: string
  session_id: string
  totals: LedgerTotals
}

export type TokenResponse = {
  totals: LedgerTotals
  first_recorded: string
  last_updated: string
  history: LedgerSnapshot[]
}

export type DesignQCCapture = {
  route: string
  file: string
  width: number
  height: number
  captured_at_ms: number
  error?: string
}

export type DesignQCReport = {
  version: number
  captured_at: string
  framework: string
  base_url: string
  captures: DesignQCCapture[]
}

export type DesignQCResponse = {
  available: boolean
  reason?: string
  report?: DesignQCReport
  captures?: unknown[]
}
