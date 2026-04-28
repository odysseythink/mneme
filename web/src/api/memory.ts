import { request } from './client'
import type { MemoryResponse } from './types'

export const getMemory = () => request<MemoryResponse>('GET', '/api/memory')
