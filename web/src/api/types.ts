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
