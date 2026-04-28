import { request } from './client'
import type { CronResponse } from './types'

export const getCron = () => request<CronResponse>('GET', '/api/cron')
export const runTask = (name: string) => request<void>('POST', '/cron/run', { name })
export const retryTask = (name: string) => request<void>('POST', '/cron/retry', { name })
