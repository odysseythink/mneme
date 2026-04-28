import { request } from './client'
import type { ActivityResponse } from './types'

export type ActivityArgs = { limit?: number; since?: number; types?: string[]; projectId?: string }

export function getActivity(args: ActivityArgs = {}): Promise<ActivityResponse> {
  const params = new URLSearchParams()
  if (args.limit) params.set('limit', String(args.limit))
  if (args.since) params.set('since', String(args.since))
  if (args.types?.length) params.set('types', args.types.join(','))
  if (args.projectId) params.set('project_id', args.projectId)
  const qs = params.toString()
  return request<ActivityResponse>('GET', '/api/activity' + (qs ? '?' + qs : ''))
}
