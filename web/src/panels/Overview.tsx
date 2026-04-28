import { ProjectCard } from '../components/ProjectCard'
import { PageHead } from '../components/PageHead'
import { Stat, Empty, Skeleton } from '../components/primitives'
import { useFetch } from '../hooks/useFetch'
import { useSSE } from '../hooks/useSSE'
import { getOverview } from '../api/overview'

export function Overview(): JSX.Element {
  const { data, error, refetch } = useFetch(getOverview, { intervalMs: 10_000 })
  useSSE(['scan.complete', 'cerebrum.candidate', 'suggestion.new'], () => refetch())

  if (error) {
    return (
      <>
        <PageHead title="Overview" />
        <Empty title={`failed to load: ${error.message}`} />
      </>
    )
  }
  if (!data) {
    return (
      <>
        <PageHead title="Overview" />
        <Skeleton rows={4} />
      </>
    )
  }

  const t = data.totals
  return (
    <>
      <PageHead title="Overview" meta={`${data.projects.length} project${data.projects.length === 1 ? '' : 's'}`} />
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(4, 1fr)', gap: 10 }}>
        <Stat label="Projects" value={String(t.projects)} />
        <Stat label="Anatomy files" value={String(t.anatomy_files)} />
        <Stat label="Cerebrum pending" value={String(t.cerebrum_pending)} />
        <Stat label="Open suggestions" value={String(t.open_suggestions)} />
      </div>
      <div>
        <h2 style={{ fontSize: 13, fontWeight: 600, color: 'var(--text-strong)', margin: '12px 0 8px' }}>
          Projects
        </h2>
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(2, 1fr)', gap: 10 }}>
          {data.projects.map((p) => <ProjectCard key={p.id} p={p} />)}
        </div>
      </div>
    </>
  )
}
