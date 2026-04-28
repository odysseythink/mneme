import { useFetch } from '../hooks/useFetch'
import { useSSE } from '../hooks/useSSE'
import { getToken } from '../api/token'
import { useActiveProject } from '../hooks/useActiveProject'
import { PageHead } from '../components/PageHead'
import { Stat, Sparkline, Empty, Skeleton } from '../components/primitives'

export function Token(): JSX.Element {
  const { active } = useActiveProject()
  const fn = active ? () => getToken(active) : () => Promise.resolve(null as never)
  const { data, error, refetch } = useFetch(fn, { intervalMs: 10_000 })
  useSSE(['hook.fired', 'cron.tick'], () => refetch())

  if (!active) {
    return (
      <>
        <PageHead title="Token" />
        <Empty title="No project selected" hint="Pick one in the sidebar." />
      </>
    )
  }
  if (error) {
    return (
      <>
        <PageHead title="Token" />
        <Empty title={`failed to load: ${error.message}`} />
      </>
    )
  }
  if (!data) {
    return (
      <>
        <PageHead title="Token" />
        <Skeleton rows={3} />
      </>
    )
  }

  const totals = data.totals
  const hookFired = totals.hook_fired ?? {}
  const totalFires = Object.values(hookFired).reduce((a, b) => a + b, 0)
  const trend = data.history.map(h => Object.values(h.totals.hook_fired ?? {}).reduce((a, b) => a + b, 0))
  const sectionStyle = { background: 'var(--bg-surface)', border: '1px solid var(--border-default)', borderRadius: 'var(--radius-4)', padding: 'var(--space-4)' }
  const labelStyle = { fontSize: 9, textTransform: 'uppercase' as const, letterSpacing: '0.06em', color: 'var(--text-muted)', fontWeight: 600 as const, marginBottom: 8 }

  return (
    <>
      <PageHead title="Token" meta={`${data.history.length} snapshots`} />

      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(4, 1fr)', gap: 10 }}>
        <Stat label="Hook fires" value={String(totalFires)} sparkline={trend.length >= 2 ? <Sparkline data={trend} /> : undefined} />
        <Stat label="Anatomy hits" value={String(totals.anatomy_hits)} />
        <Stat label="Repeat reads" value={String(totals.repeat_reads)} />
        <Stat label="Scan count" value={String(totals.scan_count)} />
      </div>

      <section style={sectionStyle}>
        <div style={labelStyle}>Trend (last {trend.length} snapshots)</div>
        {trend.length < 2 ? (
          <div style={{ fontSize: 11, color: 'var(--text-muted)' }}>Not enough history yet.</div>
        ) : (
          <Sparkline data={trend} height={48} />
        )}
      </section>

      <section style={sectionStyle}>
        <div style={labelStyle}>Hook fires by type</div>
        {Object.keys(hookFired).length === 0 ? (
          <div style={{ fontSize: 11, color: 'var(--text-muted)' }}>No hook fires yet.</div>
        ) : (
          <ul style={{ listStyle: 'none', padding: 0, margin: 0 }}>
            {Object.entries(hookFired).map(([k, v], i, arr) => (
              <li
                key={k}
                style={{
                  display: 'flex',
                  justifyContent: 'space-between',
                  padding: '6px 0',
                  borderBottom: i === arr.length - 1 ? 'none' : '1px solid color-mix(in srgb, var(--border-default) 40%, transparent)',
                  fontSize: 11,
                }}
              >
                <span className="mono" style={{ color: 'var(--text-body)' }}>{k}</span>
                <span className="mono" style={{ color: 'var(--text-strong)' }}>{v}</span>
              </li>
            ))}
          </ul>
        )}
      </section>
    </>
  )
}
