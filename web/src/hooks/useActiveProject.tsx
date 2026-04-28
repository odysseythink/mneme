import { createContext, useContext, useEffect, useMemo, type ReactNode } from 'react'
import { useSearchParams } from 'react-router-dom'
import { useFetch } from './useFetch'
import { getProjects } from '../api/projects'
import type { ProjectSummary } from '../api/types'

type ActiveProjectValue = {
  active: string | null
  projects: ProjectSummary[]
  setActive: (id: string) => void
  loading: boolean
}

const Ctx = createContext<ActiveProjectValue | null>(null)

export function ActiveProjectProvider({ children }: { children: ReactNode }): JSX.Element {
  const { data, loading } = useFetch(getProjects)
  const [params, setParams] = useSearchParams()
  const queryProject = params.get('project')

  useEffect(() => {
    if (loading || queryProject) return
    const first = data?.projects[0]?.id
    if (first) {
      const next = new URLSearchParams(params)
      next.set('project', first)
      setParams(next, { replace: true })
    }
  }, [loading, queryProject, data, params, setParams])

  const value = useMemo<ActiveProjectValue>(() => ({
    active: queryProject,
    projects: data?.projects ?? [],
    loading,
    setActive: (id: string) => {
      const next = new URLSearchParams(params)
      next.set('project', id)
      setParams(next)
    },
  }), [queryProject, data, loading, params, setParams])

  return <Ctx.Provider value={value}>{children}</Ctx.Provider>
}

export function useActiveProject(): ActiveProjectValue {
  const v = useContext(Ctx)
  if (!v) throw new Error('useActiveProject must be used inside ActiveProjectProvider')
  return v
}
