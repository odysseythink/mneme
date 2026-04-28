import type { ProjectSummary } from '../api/types'

export function ProjectCard({ p }: { p: ProjectSummary }): JSX.Element {
  return (
    <div
      style={{
        background: 'var(--bg-surface)',
        border: '1px solid var(--border-default)',
        borderRadius: 'var(--radius-3)',
        padding: 'var(--space-3)',
      }}
    >
      <div style={{ fontWeight: 500, color: 'var(--text-strong)' }}>{p.origin || p.id}</div>
      <div style={{ fontSize: 13, color: 'var(--text-muted)', marginTop: 'var(--space-2)' }}>
        {p.anatomy_files} files · {p.cerebrum_pending} cerebrum pending · {Math.round(p.memory_bytes / 1024)} KB memory
      </div>
    </div>
  )
}
