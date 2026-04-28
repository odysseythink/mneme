import { HealthCard } from '../components/HealthCard'
import { ProjectCard } from '../components/ProjectCard'
import { useFetch } from '../hooks/useFetch'
import { useSSE } from '../hooks/useSSE'
import { getOverview } from '../api/overview'

export function Overview(): JSX.Element {
  const { data, error, refetch } = useFetch(getOverview, { intervalMs: 10_000 })
  useSSE(['scan.complete', 'cerebrum.candidate', 'suggestion.new'], () => refetch())

  if (error) return <div className="text-red-700">failed to load: {error.message}</div>
  if (!data) return <div className="text-gray-500">loading…</div>

  const t = data.totals
  return (
    <div className="flex flex-col gap-6">
      <h1 className="text-2xl font-semibold">overview</h1>
      <HealthCard />
      <div className="grid grid-cols-4 gap-3">
        <Stat label="projects" value={t.projects} />
        <Stat label="anatomy files" value={t.anatomy_files} />
        <Stat label="cerebrum pending" value={t.cerebrum_pending} />
        <Stat label="open suggestions" value={t.open_suggestions} />
      </div>
      <div>
        <h2 className="text-lg font-medium mb-2">projects</h2>
        <div className="grid grid-cols-2 gap-3">
          {data.projects.map(p => <ProjectCard key={p.id} p={p} />)}
        </div>
      </div>
    </div>
  )
}

function Stat({ label, value }: { label: string; value: number }): JSX.Element {
  return (
    <div className="rounded border p-3">
      <div className="text-xs text-gray-600">{label}</div>
      <div className="text-2xl font-semibold mt-1">{value}</div>
    </div>
  )
}
