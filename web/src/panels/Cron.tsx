import { useFetch } from '../hooks/useFetch'
import { useSSE } from '../hooks/useSSE'
import { getCron } from '../api/cron'
import { CronTaskRow } from '../components/CronTaskRow'

export function Cron(): JSX.Element {
  const { data, error, refetch } = useFetch(getCron, { intervalMs: 5_000 })
  useSSE(['cron.tick'], () => refetch())

  if (error) return <div className="text-red-700">failed to load: {error.message}</div>
  if (!data) return <div className="text-gray-500">loading…</div>

  return (
    <div>
      <h1 className="text-2xl font-semibold mb-4">cron</h1>
      <table className="w-full bg-white rounded border">
        <thead className="text-left text-sm text-gray-600 border-b">
          <tr>
            <th className="px-3 py-2">name</th>
            <th className="px-3 py-2">schedule</th>
            <th className="px-3 py-2">last run</th>
            <th className="px-3 py-2">status</th>
            <th className="px-3 py-2">actions</th>
          </tr>
        </thead>
        <tbody>
          {data.tasks.map(t => <CronTaskRow key={t.name} t={t} onAction={refetch} />)}
        </tbody>
      </table>
    </div>
  )
}
