import { useState, Fragment } from 'react'
import { useFetch } from '../hooks/useFetch'
import { useSSE } from '../hooks/useSSE'
import { getBugLog, deleteEntry } from '../api/buglog'
import { useActiveProject } from '../hooks/useActiveProject'
import { ConfirmButton } from '../components/ConfirmButton'
import { PageHead } from '../components/PageHead'
import { Empty, Skeleton } from '../components/primitives'

export function BugLog(): JSX.Element {
  const { active } = useActiveProject()
  const fn = active ? () => getBugLog(active) : () => Promise.resolve({ entries: [] })
  const { data, error, refetch } = useFetch(fn, { intervalMs: 30_000 })
  useSSE(['buglog.deleted'], () => refetch())
  const [expanded, setExpanded] = useState<Set<string>>(new Set())

  if (!active) {
    return (
      <>
        <PageHead title="BugLog" />
        <Empty title="No project selected" hint="Pick a project from the sidebar." />
      </>
    )
  }
  if (error) {
    return (
      <>
        <PageHead title="BugLog" />
        <Empty title={`failed to load: ${error.message}`} />
      </>
    )
  }
  if (!data) {
    return (
      <>
        <PageHead title="BugLog" />
        <Skeleton rows={6} />
      </>
    )
  }

  const toggle = (id: string) => {
    setExpanded(prev => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id); else next.add(id)
      return next
    })
  }

  if (data.entries.length === 0) {
    return (
      <>
        <PageHead title="BugLog" />
        <Empty icon="∅" title="No bugs logged yet" hint="Run `mneme buglog add` to record one." />
      </>
    )
  }

  return (
    <>
      <PageHead title="BugLog" meta={`${data.entries.length} ${data.entries.length === 1 ? 'entry' : 'entries'}`} />
      <div style={{ background: 'var(--bg-surface)', border: '1px solid var(--border-default)', borderRadius: 'var(--radius-4)', padding: 'var(--space-4)' }}>
        <table style={{ width: '100%', borderCollapse: 'collapse' }}>
          <thead>
            <tr>
              {['Created', 'Source', 'File', 'Description', 'Actions'].map((h, i) => (
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
                    width: i === 0 ? 160 : i === 1 ? 80 : i === 4 ? 120 : undefined,
                  }}
                >
                  {h}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {data.entries.map(e => (
              <Fragment key={e.id}>
                <tr style={{ borderBottom: '1px solid color-mix(in srgb, var(--border-default) 40%, transparent)', cursor: e.bad_code ? 'pointer' : 'default' }}>
                  <td style={{ padding: '6px 10px', fontSize: 11, color: 'var(--text-muted)' }} className="mono" onClick={() => toggle(e.id)}>{e.created_at}</td>
                  <td style={{ padding: '6px 10px', fontSize: 11 }} onClick={() => toggle(e.id)}>{e.source}</td>
                  <td style={{ padding: '6px 10px', fontSize: 11 }} className="mono" onClick={() => toggle(e.id)}>{e.file}</td>
                  <td style={{ padding: '6px 10px', fontSize: 11 }} onClick={() => toggle(e.id)}>{e.description}</td>
                  <td style={{ padding: '6px 10px' }}>
                    <ConfirmButton
                      label="Delete"
                      onConfirm={async () => { await deleteEntry(active, e.id); refetch() }}
                    />
                  </td>
                </tr>
                {expanded.has(e.id) && e.bad_code && (
                  <tr style={{ borderBottom: '1px solid color-mix(in srgb, var(--border-default) 40%, transparent)' }}>
                    <td colSpan={5} style={{ padding: '6px 10px', background: 'var(--bg-raised)' }}>
                      <pre className="mono" style={{ fontSize: 10, overflow: 'auto', whiteSpace: 'pre', margin: 0, color: 'var(--text-body)' }}>{e.bad_code}</pre>
                    </td>
                  </tr>
                )}
              </Fragment>
            ))}
          </tbody>
        </table>
      </div>
    </>
  )
}
