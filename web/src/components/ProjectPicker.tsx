import { useActiveProject } from '../hooks/useActiveProject'

export function ProjectPicker(): JSX.Element {
  const { active, projects, setActive, loading } = useActiveProject()
  if (loading) return <div className="text-xs text-gray-500">loading…</div>
  if (projects.length === 0) {
    return <div className="text-xs text-gray-500">no projects registered</div>
  }
  return (
    <select
      value={active ?? ''}
      onChange={(e) => setActive(e.target.value)}
      className="w-full rounded border px-2 py-1 bg-white"
    >
      {projects.map(p => (
        <option key={p.id} value={p.id}>{p.origin || p.id}</option>
      ))}
    </select>
  )
}
