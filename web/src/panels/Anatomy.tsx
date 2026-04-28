import { useState } from 'react'
import { useFetch } from '../hooks/useFetch'
import { useSSE } from '../hooks/useSSE'
import { getAnatomy } from '../api/anatomy'
import { useActiveProject } from '../hooks/useActiveProject'

export function Anatomy(): JSX.Element {
  const { active } = useActiveProject()
  const fn = active ? () => getAnatomy(active) : () => Promise.resolve({ directories: [], generated_at: '' })
  const { data, error, refetch } = useFetch(fn, { intervalMs: 60_000 })
  useSSE(['scan.complete'], () => refetch())
  const [filter, setFilter] = useState('')
  const [collapsed, setCollapsed] = useState<Set<string>>(new Set())

  if (!active) return <div className="text-gray-500">No project selected.</div>
  if (error) return <div className="text-red-700">failed to load: {error.message}</div>
  if (!data) return <div className="text-gray-500">loading…</div>

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

  return (
    <div className="flex flex-col gap-4">
      <h1 className="text-2xl font-semibold">anatomy</h1>
      <div className="text-xs text-gray-500">generated {data.generated_at || '(never)'}</div>
      <input
        value={filter}
        onChange={(e) => setFilter(e.target.value)}
        placeholder="filter by filename…"
        className="w-full rounded border px-3 py-2 text-sm"
      />
      {dirs.length === 0 ? (
        <div className="text-gray-500">no files {needle && 'match'}</div>
      ) : (
        <div className="flex flex-col gap-3">
          {dirs.map(d => {
            const isCollapsed = collapsed.has(d.path)
            return (
              <div key={d.path} className="rounded border bg-white">
                <button
                  className="w-full text-left px-3 py-2 font-medium border-b bg-gray-50 hover:bg-gray-100"
                  onClick={() => toggle(d.path)}
                >
                  {isCollapsed ? '▶' : '▼'} {d.path}/ ({d.files.length})
                </button>
                {!isCollapsed && (
                  <ul>
                    {d.files.map(f => (
                      <li key={f.name} className="px-3 py-2 border-b last:border-b-0 text-sm flex gap-3">
                        <span className="font-mono">{f.name}</span>
                        <span className="text-xs bg-gray-100 rounded px-1 py-0.5 self-start shrink-0">{f.est_tokens}t</span>
                        <span className="text-gray-700 flex-1">{f.description}</span>
                      </li>
                    ))}
                  </ul>
                )}
              </div>
            )
          })}
        </div>
      )}
    </div>
  )
}
