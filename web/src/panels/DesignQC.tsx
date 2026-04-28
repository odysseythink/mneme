import { useFetch } from '../hooks/useFetch'
import { getDesignQC } from '../api/designqc'
import { useActiveProject } from '../hooks/useActiveProject'

export function DesignQC(): JSX.Element {
  const { active } = useActiveProject()
  const fn = active ? () => getDesignQC(active) : () => Promise.resolve({ available: false, reason: '', captures: [] })
  const { data } = useFetch(fn)

  if (!active) return <div className="text-gray-500">No project selected.</div>

  if (!data || !data.available) {
    return (
      <div>
        <h1 className="text-2xl font-semibold mb-4">design qc</h1>
        <div className="rounded border bg-white p-4 max-w-2xl">
          <p className="font-medium mb-2">Design QC is part of M11.</p>
          <p className="text-sm text-gray-700">
            Once <code className="bg-gray-100 px-1 rounded">mneme designqc</code> runs against this project,
            captures and the latest report will appear here.
          </p>
        </div>
      </div>
    )
  }

  return (
    <div>
      <h1 className="text-2xl font-semibold mb-4">design qc</h1>
      <div className="text-sm text-gray-500">{data.captures.length} captures (UI lands in M11)</div>
    </div>
  )
}
