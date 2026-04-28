import { useFetch } from '../hooks/useFetch'
import { getOverview } from '../api/overview'

export function HealthCard(): JSX.Element {
  const { data, error } = useFetch(getOverview, { intervalMs: 10_000 })
  if (error) return <div className="rounded border p-3 text-red-700">daemon unreachable</div>
  if (!data) return <div className="rounded border p-3 text-gray-500">loading…</div>
  const d = data.daemon
  return (
    <div className="rounded border p-3">
      <div className="font-medium">daemon</div>
      <div className="text-sm text-gray-600">
        pid {d.pid} · v{d.version} · up {fmt(d.uptime_s)}
      </div>
    </div>
  )
}

function fmt(s: number): string {
  const h = Math.floor(s / 3600)
  const m = Math.floor((s % 3600) / 60)
  return h > 0 ? `${h}h ${m}m` : `${m}m`
}
