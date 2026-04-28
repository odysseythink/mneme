import { request } from './client'
import type { SuggestionsResponse } from './types'

export const getSuggestions = (projectId: string) =>
  request<SuggestionsResponse>('GET', `/api/suggestions?project=${encodeURIComponent(projectId)}`)
export const dismissSuggestion = (projectId: string, suggestionId: string) =>
  request<{ status: string }>('POST', '/suggestions/dismiss', { project_id: projectId, suggestion_id: suggestionId })
