import { request } from './client'
import type { TokenResponse } from './types'

export const getToken = (projectId: string) =>
  request<TokenResponse>('GET', `/api/token?project=${encodeURIComponent(projectId)}`)
