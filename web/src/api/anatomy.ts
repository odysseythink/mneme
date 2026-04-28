import { request } from './client'
import type { AnatomyResponse } from './types'

export const getAnatomy = (projectId: string) =>
  request<AnatomyResponse>('GET', `/api/anatomy?project=${encodeURIComponent(projectId)}`)
