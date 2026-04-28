import { useFetch } from './useFetch'
import { useSSE } from './useSSE'
import { getProjects } from '../api/projects'

export function useProjectList() {
  const r = useFetch(getProjects)
  useSSE(['scan.complete', 'cerebrum.candidate', 'suggestion.new'], () => r.refetch())
  return r
}
