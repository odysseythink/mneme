import { useActiveProject } from '../hooks/useActiveProject'

export function ProjectPicker(): JSX.Element {
  const { active, projects, setActive, loading } = useActiveProject()

  if (loading) {
    return (
      <div style={{ fontSize: 10, color: 'var(--text-muted)' }}>loading…</div>
    )
  }
  if (projects.length === 0) {
    return (
      <div style={{ fontSize: 10, color: 'var(--text-muted)' }}>no projects registered</div>
    )
  }
  return (
    <select
      value={active ?? ''}
      onChange={(e) => setActive(e.target.value)}
      style={{
        width: '100%',
        padding: '5px 8px',
        borderRadius: 'var(--radius-2)',
        border: '1px solid var(--border-default)',
        background: 'var(--bg-base)',
        color: 'var(--text-body)',
        fontSize: 11,
        fontFamily: 'var(--font-mono)',
        cursor: 'pointer',
      }}
    >
      {projects.map(p => (
        <option key={p.id} value={p.id}>{p.origin || p.id}</option>
      ))}
    </select>
  )
}
