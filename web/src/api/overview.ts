import { request } from './client'
import type { Overview } from './types'

export const getOverview = () => request<Overview>('GET', '/api/overview')
