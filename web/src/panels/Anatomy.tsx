import { useState } from 'react'
import { useFetch } from '../hooks/useFetch'
import { useSSE } from '../hooks/useSSE'
import { getAnatomy } from '../api/anatomy'
import { useActiveProject } from '../hooks/useActiveProject'
import { PageHead } from '../components/PageHead'
import { Empty, Skeleton, Pill, Input, Kbd } from '../components/primitives'

export function Anatomy(): JSX.Element {
  const { active } = useActiveProject()
  const fn = active ? () => getAnatomy(active) : () => Promise.resolve({ directories: [], generated_at: '' })
  const { data, error, refetch } = useFetch(fn, { intervalMs: 60_000 })
  useSSE(['scan.complete'], () => refetch())
  const [filter, setFilter] = useState('')
  const [collapsed, setCollapsed] = useState<Set<string>>(new Set())

  if (!active) {
    return (
      <>
        <PageHead title="Anatomy" />
        <Empty title="No project selected" hint="Pick one in the sidebar." />
      </>
    )
  }
  if (error) {
    return (
      <>
        <PageHead title="Anatomy" />
        <Empty title={`failed to load: ${error.message}`} />
      </>
    )
  }
  if (!data) {
    return (
      <>
        <PageHead title="Anatomy" />
        <Skeleton rows={6} />
      </>
    )
  }

  const needle = filter.trim().toLowerCase()
  const dirs = needle === '' ? data.directories : data.directories
    .map(d => ({ ...d, files: d.files.filter(f => f.name.toLowerCase().includes(needle)) }))
    .filter(d => d.files.length > 0)

  const toggle = (path: string) => {
    setCollapsed(prev => {
      const next = new Set(prev)
      if (next.has(path)) next.delete(path); else next.add(path)
      return next
    })
  }

  const totalFiles = data.directories.reduce((acc, d) => acc + d.files.length, 0)

  return (
    <>
      <PageHead
        title="Anatomy"
        meta={`${dirs.length} dir${dirs.length === 1 ? '' : 's'} · ${totalFiles} files · generated ${data.generated_at || '(never)'}`}
      />
      <Input
        value={filter}
        onChange={(e) => setFilter(e.target.value)}
        placeholder="Filter by filename…"
        style={{ width: '100%', maxWidth: 360 }}
      />
      {dirs.length === 0 ? (
        <Empty
          icon="∅"
          title={needle ? `No files match "${filter}"` : 'No anatomy map'}
          hint={needle ? undefined : <>Run <Kbd>mneme scan</Kbd> to generate one.</>}
        />
      ) : (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
          {dirs.map(d => {
            const isCollapsed = collapsed.has(d.path)
            return (
              <div
                key={d.path}
                style={{
                  background: 'var(--bg-surface)',
                  border: '1px solid var(--border-default)',
                  borderRadius: 'var(--radius-3)',
                  overflow: 'hidden',
                }}
              >
                <button
                  onClick={() => toggle(d.path)}
                  style={{
                    width: '100%',
                    textAlign: 'left',
                    padding: '8px 12px',
                    fontWeight: 500,
                    fontSize: 12,
                    background: 'var(--bg-raised)',
                    color: 'var(--text-strong)',
                    border: 'none',
                    borderBottom: isCollapsed ? 'none' : '1px solid var(--border-default)',
                    cursor: 'pointer',
                    display: 'flex',
                    alignItems: 'center',
                    gap: 8,
                  }}
                >
                  <span style={{ color: 'var(--text-muted)', fontSize: 10 }}>{isCollapsed ? '▶' : '▼'}</span>
                  <span className="mono">{d.path}/</span>
                  <span style={{ color: 'var(--text-muted)', fontSize: 10, marginLeft: 'auto' }}>
                    {d.files.length} file{d.files.length === 1 ? '' : 's'}
                  </span>
                </button>
                {!isCollapsed && (
                  <ul style={{ listStyle: 'none', padding: 0, margin: 0 }}>
                    {d.files.map((f, i) => (
                      <li
                        key={f.name}
                        style={{
                          padding: '6px 12px',
                          fontSize: 11,
                          display: 'flex',
                          gap: 12,
                          alignItems: 'center',
                          borderBottom: i === d.files.length - 1 ? 'none' : '1px solid color-mix(in srgb, var(--border-default) 40%, transparent)',
                        }}
                      >
                        <span className="mono" style={{ color: 'var(--text-body)' }}>{f.name}</span>
                        <Pill variant="neutral">{f.est_tokens}t</Pill>
                        <span style={{ color: 'var(--text-muted)', flex: 1, fontSize: 11 }}>{f.description}</span>
                      </li>
                    ))}
                  </ul>
                )}
              </div>
            )
          })}
        </div>
      )}
    </>
  )
}
