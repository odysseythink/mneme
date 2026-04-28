import { useFetch } from '../hooks/useFetch'
import { getMemory } from '../api/memory'
import { PageHead } from '../components/PageHead'
import { Empty, Skeleton, Pill } from '../components/primitives'

export function Memory(): JSX.Element {
  const { data, error } = useFetch(getMemory, { intervalMs: 60_000 })

  if (error) {
    return (
      <>
        <PageHead title="Memory" />
        <Empty title={`failed to load: ${error.message}`} />
      </>
    )
  }
  if (!data) {
    return (
      <>
        <PageHead title="Memory" />
        <Skeleton rows={4} />
      </>
    )
  }

  return (
    <>
      <PageHead title="Memory" meta={`${data.rows.length} sessions · cross-project`} />

      {data.rows.length === 0 ? (
        data.raw ? (
          <pre
            className="mono"
            style={{
              background: 'var(--bg-surface)',
              border: '1px solid var(--border-default)',
              borderRadius: 'var(--radius-3)',
              padding: 'var(--space-3)',
              fontSize: 12,
              whiteSpace: 'pre-wrap',
              color: 'var(--text-body)',
              margin: 0,
            }}
          >
            {data.raw}
          </pre>
        ) : (
          <Empty icon="▭" title="No memory rows yet" hint="Memory accumulates from session summaries." />
        )
      ) : (
        <ul style={{ listStyle: 'none', padding: 0, margin: 0, display: 'flex', flexDirection: 'column', gap: 8 }}>
          {data.rows.map((row, i) => (
            <li
              key={i}
              style={{
                background: 'var(--bg-surface)',
                border: '1px solid var(--border-default)',
                borderRadius: 'var(--radius-3)',
                padding: 'var(--space-3)',
                display: 'flex',
                gap: 12,
              }}
            >
              <div className="mono" style={{ fontSize: 11, color: 'var(--text-muted)', width: 160, flexShrink: 0 }}>
                {row.started_at}
              </div>
              {row.turn_count > 0 && (
                <div style={{ alignSelf: 'flex-start', flexShrink: 0 }}>
                  <Pill variant="neutral">{row.turn_count} turns</Pill>
                </div>
              )}
              <div style={{ fontSize: 12, whiteSpace: 'pre-wrap', flex: 1, color: 'var(--text-body)' }}>
                {row.summary}
              </div>
            </li>
          ))}
        </ul>
      )}
    </>
  )
}
