import { request } from './client'
import type { BugLogResponse } from './types'

export const getBugLog = (projectId: string) =>
  request<BugLogResponse>('GET', `/api/buglog?project=${encodeURIComponent(projectId)}`)
export const deleteEntry = (projectId: string, entryId: string) =>
  request<{ status: string }>('POST', '/buglog/delete', { project_id: projectId, entry_id: entryId })
