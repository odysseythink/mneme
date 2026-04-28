import { useState } from 'react'
import { useFetch } from '../hooks/useFetch'
import { getDesignQC } from '../api/designqc'
import { useActiveProject } from '../hooks/useActiveProject'
import { PageHead } from '../components/PageHead'
import { Empty, Skeleton, Kbd } from '../components/primitives'

export function DesignQC(): JSX.Element {
  const { active } = useActiveProject()
  const fn = active
    ? () => getDesignQC(active)
    : () => Promise.resolve({ available: false, reason: 'select a project' } as const)
  const { data } = useFetch(fn, { intervalMs: 30_000 })
  const [expanded, setExpanded] = useState<string | null>(null)

  if (!active) {
    return (
      <>
        <PageHead title="DesignQC" />
        <Empty title="No project selected" hint="Pick one in the sidebar." />
      </>
    )
  }
  if (!data) {
    return (
      <>
        <PageHead title="DesignQC" />
        <Skeleton rows={4} />
      </>
    )
  }

  if (!data.available || !data.report) {
    return (
      <>
        <PageHead title="DesignQC" />
        <Empty
          icon="◭"
          title="No captures yet"
          hint={
            <>
              Run <Kbd>mneme designqc</Kbd> in this project to generate captures.
              {data.reason && <div style={{ marginTop: 6, fontSize: 10, color: 'var(--text-faint)' }}>reason: {data.reason}</div>}
            </>
          }
        />
      </>
    )
  }

  const r = data.report
  return (
    <>
      <PageHead
        title="DesignQC"
        meta={`${r.captures.length} routes · ${r.framework} · captured ${r.captured_at}`}
      />
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(3, 1fr)', gap: 12 }}>
        {r.captures.map(c => (
          <button
            key={c.route}
            onClick={() => !c.error && setExpanded(c.file)}
            disabled={!!c.error}
            style={{
              background: 'var(--bg-surface)',
              border: '1px solid var(--border-default)',
              borderRadius: 'var(--radius-3)',
              padding: 8,
              textAlign: 'left',
              cursor: c.error ? 'default' : 'pointer',
              opacity: c.error ? 0.6 : 1,
              display: 'flex',
              flexDirection: 'column',
              gap: 6,
            }}
          >
            {c.error ? (
              <div
                style={{
                  width: '100%',
                  height: 128,
                  borderRadius: 'var(--radius-2)',
                  background: 'color-mix(in srgb, var(--err) 10%, transparent)',
                  color: 'var(--err)',
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  fontSize: 11,
                  padding: '0 8px',
                  textAlign: 'center',
                }}
              >
                {c.error}
              </div>
            ) : (
              <img
                src={`/api/designqc/captures/${active}/${c.file}`}
                style={{ width: '100%', height: 128, objectFit: 'cover', objectPosition: 'top', borderRadius: 'var(--radius-2)', display: 'block' }}
                alt={c.route}
              />
            )}
            <div className="mono" style={{ fontSize: 12, color: 'var(--text-body)' }}>{c.route}</div>
            {!c.error && (
              <div className="mono" style={{ fontSize: 10, color: 'var(--text-muted)' }}>{c.width}×{c.height}</div>
            )}
          </button>
        ))}
      </div>

      {expanded && (
        <div
          onClick={() => setExpanded(null)}
          style={{
            position: 'fixed',
            inset: 0,
            background: 'rgba(0,0,0,0.85)',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            zIndex: 100,
            cursor: 'pointer',
          }}
        >
          <img
            src={`/api/designqc/captures/${active}/${expanded}`}
            style={{ maxWidth: '95vw', maxHeight: '95vh', objectFit: 'contain' }}
            alt={expanded}
          />
        </div>
      )}
    </>
  )
}
