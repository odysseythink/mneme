import { request } from './client'
import type { CerebrumResponse } from './types'

export const getCerebrum = (projectId: string) =>
  request<CerebrumResponse>('GET', `/api/cerebrum?project=${encodeURIComponent(projectId)}`)
export const approveCandidate = (projectId: string, candidateId: string) =>
  request<{ status: string }>('POST', '/cerebrum/approve', { project_id: projectId, candidate_id: candidateId })
export const rejectCandidate = (projectId: string, candidateId: string) =>
  request<{ status: string }>('POST', '/cerebrum/reject', { project_id: projectId, candidate_id: candidateId })
