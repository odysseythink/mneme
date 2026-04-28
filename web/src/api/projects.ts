import { request } from './client'
import type { ProjectsResponse } from './types'

export const getProjects = () => request<ProjectsResponse>('GET', '/api/projects')
