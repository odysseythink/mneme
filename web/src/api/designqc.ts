import { request } from './client'
import type { DesignQCResponse } from './types'

export const getDesignQC = (projectId: string) =>
  request<DesignQCResponse>('GET', `/api/designqc?project=${encodeURIComponent(projectId)}`)
