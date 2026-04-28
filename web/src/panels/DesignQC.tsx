import { useState } from 'react'
import { useFetch } from '../hooks/useFetch'
import { getDesignQC } from '../api/designqc'
import { useActiveProject } from '../hooks/useActiveProject'

export function DesignQC(): JSX.Element {
  const { active } = useActiveProject()
  const fn = active
    ? () => getDesignQC(active)
    : () => Promise.resolve({ available: false, reason: 'select a project' } as const)
  const { data } = useFetch(fn, { intervalMs: 30_000 })
  const [expanded, setExpanded] = useState<string | null>(null)

  if (!active) return <div className="text-gray-500">No project selected.</div>
  if (!data) return <div className="text-gray-500">loading…</div>

  if (!data.available || !data.report) {
    return (
      <div>
        <h1 className="text-2xl font-semibold mb-4">design qc</h1>
        <div className="rounded border bg-white p-4 max-w-2xl">
          <p className="font-medium mb-2">No captures yet.</p>
          <p className="text-sm text-gray-700">
            Run <code className="bg-gray-100 px-1 rounded">mneme designqc</code> in this project to generate captures.
          </p>
          {data.reason && <p className="mt-2 text-xs text-gray-500">reason: {data.reason}</p>}
        </div>
      </div>
    )
  }

  const r = data.report
  return (
    <div>
      <h1 className="text-2xl font-semibold mb-2">design qc</h1>
      <div className="text-sm text-gray-600 mb-4">
        {r.framework} · {r.base_url} · captured {r.captured_at}
      </div>
      <div className="grid grid-cols-3 gap-4">
        {r.captures.map(c => (
          <button
            key={c.route}
            onClick={() => !c.error && setExpanded(c.file)}
            className={'rounded border bg-white p-2 text-left ' + (c.error ? 'opacity-60' : 'hover:shadow')}
          >
            {c.error ? (
              <div className="w-full h-32 rounded bg-red-50 text-red-700 flex items-center justify-center text-xs px-2">
                {c.error}
              </div>
            ) : (
              <img
                src={`/api/designqc/captures/${active}/${c.file}`}
                className="w-full h-32 object-cover object-top rounded"
                alt={c.route}
              />
            )}
            <div className="mt-2 text-sm font-mono">{c.route}</div>
            {!c.error && (
              <div className="text-xs text-gray-500">{c.width}×{c.height}</div>
            )}
          </button>
        ))}
      </div>
      {expanded && (
        <div
          onClick={() => setExpanded(null)}
          className="fixed inset-0 bg-black/70 flex items-center justify-center z-50 cursor-pointer"
        >
          <img
            src={`/api/designqc/captures/${active}/${expanded}`}
            className="max-w-[95vw] max-h-[95vh] object-contain"
            alt={expanded}
          />
        </div>
      )}
    </div>
  )
}
