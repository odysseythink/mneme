import { useFetch } from '../hooks/useFetch'
import { useSSE } from '../hooks/useSSE'
import { getToken } from '../api/token'
import { useActiveProject } from '../hooks/useActiveProject'
import { Sparkline } from '../components/primitives/Sparkline'

export function Token(): JSX.Element {
  const { active } = useActiveProject()
  const fn = active ? () => getToken(active) : () => Promise.resolve(null as never)
  const { data, error, refetch } = useFetch(fn, { intervalMs: 10_000 })
  useSSE(['hook.fired', 'cron.tick'], () => refetch())

  if (!active) return <div className="text-gray-500">No project selected.</div>
  if (error) return <div className="text-red-700">failed to load: {error.message}</div>
  if (!data) return <div className="text-gray-500">loading…</div>

  const totals = data.totals
  const hookFired = totals.hook_fired ?? {}
  const totalFires = Object.values(hookFired).reduce((a, b) => a + b, 0)
  const trend = data.history.map(h => Object.values(h.totals.hook_fired ?? {}).reduce((a, b) => a + b, 0))

  return (
    <div className="flex flex-col gap-6">
      <h1 className="text-2xl font-semibold">token</h1>

      <section className="rounded border bg-white p-3">
        <div className="text-sm text-gray-600 mb-2">totals</div>
        <div className="grid grid-cols-4 gap-3">
          <Stat label="hook fires" value={totalFires} />
          <Stat label="anatomy hits" value={totals.anatomy_hits} />
          <Stat label="repeat reads" value={totals.repeat_reads} />
          <Stat label="scan count" value={totals.scan_count} />
        </div>
      </section>

      <section className="rounded border bg-white p-3">
        <div className="text-sm text-gray-600 mb-2">trend (last {trend.length} snapshots)</div>
        {trend.length < 2 ? (
          <div className="text-gray-500 text-sm">Not enough history yet.</div>
        ) : (
          <Sparkline data={trend} height={40} />
        )}
      </section>

      <section className="rounded border bg-white p-3">
        <div className="text-sm text-gray-600 mb-2">hook fires by type</div>
        <ul className="text-sm">
          {Object.entries(hookFired).map(([k, v]) => (
            <li key={k} className="flex justify-between border-b last:border-b-0 py-1">
              <span className="font-mono text-xs">{k}</span>
              <span>{v}</span>
            </li>
          ))}
        </ul>
      </section>
    </div>
  )
}

function Stat({ label, value }: { label: string; value: number }): JSX.Element {
  return (
    <div>
      <div className="text-xs text-gray-600">{label}</div>
      <div className="text-2xl font-semibold mt-1">{value}</div>
    </div>
  )
}
