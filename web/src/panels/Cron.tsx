import { useFetch } from '../hooks/useFetch'
import { useSSE } from '../hooks/useSSE'
import { getCron } from '../api/cron'
import { CronTaskRow } from '../components/CronTaskRow'
import { PageHead } from '../components/PageHead'
import { Empty, Skeleton } from '../components/primitives'

export function Cron(): JSX.Element {
  const { data, error, refetch } = useFetch(getCron, { intervalMs: 5_000 })
  useSSE(['cron.tick'], () => refetch())

  if (error) {
    return (
      <>
        <PageHead title="Cron" />
        <Empty title={`failed to load: ${error.message}`} />
      </>
    )
  }
  if (!data) {
    return (
      <>
        <PageHead title="Cron" />
        <Skeleton rows={4} />
      </>
    )
  }

  if (data.tasks.length === 0) {
    return (
      <>
        <PageHead title="Cron" />
        <Empty icon="⏱" title="No cron tasks scheduled" />
      </>
    )
  }

  const failingCount = data.tasks.filter(t => !!t.state.dead_lettered_at || t.state.consecutive_failures > 0).length

  return (
    <>
      <PageHead title="Cron" meta={`${data.tasks.length} task${data.tasks.length === 1 ? '' : 's'}${failingCount > 0 ? ` · ${failingCount} unhealthy` : ''}`} />
      <div style={{ background: 'var(--bg-surface)', border: '1px solid var(--border-default)', borderRadius: 'var(--radius-4)', padding: 'var(--space-4)' }}>
        <table style={{ width: '100%', borderCollapse: 'collapse' }}>
          <thead>
            <tr>
              {['Name', 'Schedule', 'Last run', 'Status', 'Actions'].map(h => (
                <th
                  key={h}
                  style={{
                    textAlign: 'left',
                    fontSize: 9,
                    textTransform: 'uppercase',
                    letterSpacing: '0.06em',
                    color: 'var(--text-muted)',
                    fontWeight: 600,
                    padding: '6px 10px',
                    borderBottom: '1px solid var(--border-default)',
                  }}
                >
                  {h}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {data.tasks.map(t => <CronTaskRow key={t.name} t={t} onAction={refetch} />)}
          </tbody>
        </table>
      </div>
    </>
  )
}
