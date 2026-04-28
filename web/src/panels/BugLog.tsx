import { useState, Fragment } from 'react'
import { useFetch } from '../hooks/useFetch'
import { useSSE } from '../hooks/useSSE'
import { getBugLog, deleteEntry } from '../api/buglog'
import { useActiveProject } from '../hooks/useActiveProject'
import { ConfirmButton } from '../components/ConfirmButton'

export function BugLog(): JSX.Element {
  const { active } = useActiveProject()
  const fn = active ? () => getBugLog(active) : () => Promise.resolve({ entries: [] })
  const { data, error, refetch } = useFetch(fn, { intervalMs: 30_000 })
  useSSE(['buglog.deleted'], () => refetch())
  const [expanded, setExpanded] = useState<Set<string>>(new Set())

  if (!active) return <div className="text-gray-500">No project selected.</div>
  if (error) return <div className="text-red-700">failed to load: {error.message}</div>
  if (!data) return <div className="text-gray-500">loading…</div>

  const toggle = (id: string) => {
    setExpanded(prev => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id); else next.add(id)
      return next
    })
  }

  return (
    <div>
      <h1 className="text-2xl font-semibold mb-4">buglog</h1>
      {data.entries.length === 0 ? (
        <div className="text-gray-500">no bug entries</div>
      ) : (
        <table className="w-full bg-white rounded border">
          <thead className="text-left text-sm text-gray-600 border-b">
            <tr>
              <th className="px-3 py-2 w-40">created</th>
              <th className="px-3 py-2 w-20">source</th>
              <th className="px-3 py-2">file</th>
              <th className="px-3 py-2">description</th>
              <th className="px-3 py-2 w-32">actions</th>
            </tr>
          </thead>
          <tbody>
            {data.entries.map(e => (
              <Fragment key={e.id}>
                <tr className="border-b text-sm">
                  <td className="px-3 py-2 cursor-pointer" onClick={() => toggle(e.id)}>{e.created_at}</td>
                  <td className="px-3 py-2 cursor-pointer" onClick={() => toggle(e.id)}>{e.source}</td>
                  <td className="px-3 py-2 font-mono text-xs cursor-pointer" onClick={() => toggle(e.id)}>{e.file}</td>
                  <td className="px-3 py-2 cursor-pointer" onClick={() => toggle(e.id)}>{e.description}</td>
                  <td className="px-3 py-2">
                    <ConfirmButton
                      label="Delete"
                      onConfirm={async () => { await deleteEntry(active, e.id); refetch() }}
                    />
                  </td>
                </tr>
                {expanded.has(e.id) && e.bad_code && (
                  <tr className="border-b">
                    <td colSpan={5} className="px-3 py-2 bg-gray-50">
                      <pre className="text-xs overflow-auto whitespace-pre">{e.bad_code}</pre>
                    </td>
                  </tr>
                )}
              </Fragment>
            ))}
          </tbody>
        </table>
      )}
    </div>
  )
}
