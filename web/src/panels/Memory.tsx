import { useFetch } from '../hooks/useFetch'
import { getMemory } from '../api/memory'

export function Memory(): JSX.Element {
  const { data, error } = useFetch(getMemory, { intervalMs: 60_000 })

  if (error) return <div className="text-red-700">failed to load: {error.message}</div>
  if (!data) return <div className="text-gray-500">loading…</div>

  return (
    <div className="flex flex-col gap-4">
      <h1 className="text-2xl font-semibold">memory</h1>
      <div className="text-xs text-gray-600">Memory is cross-project. The sidebar picker is ignored on this page.</div>

      {data.rows.length === 0 ? (
        data.raw ? (
          <pre className="rounded border bg-white p-3 text-sm whitespace-pre-wrap">{data.raw}</pre>
        ) : (
          <div className="text-gray-500">no memory rows yet</div>
        )
      ) : (
        <ul className="flex flex-col gap-2">
          {data.rows.map((row, i) => (
            <li key={i} className="rounded border bg-white p-3 flex gap-3">
              <div className="text-xs text-gray-500 w-40 shrink-0">{row.started_at}</div>
              {row.turn_count > 0 && (
                <span className="text-xs bg-gray-100 rounded px-2 py-0.5 self-start shrink-0">{row.turn_count} turns</span>
              )}
              <div className="text-sm whitespace-pre-wrap flex-1">{row.summary}</div>
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}
