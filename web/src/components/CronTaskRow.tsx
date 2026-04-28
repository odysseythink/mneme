import { useState } from 'react'
import type { CronTask } from '../api/types'
import { runTask, retryTask } from '../api/cron'

export function CronTaskRow({ t, onAction }: { t: CronTask; onAction: () => void }): JSX.Element {
  const [busy, setBusy] = useState(false)
  const [err, setErr] = useState<string | null>(null)
  const dead = !!t.state.dead_lettered_at

  const fire = async (fn: (n: string) => Promise<void>) => {
    setBusy(true); setErr(null)
    try { await fn(t.name); onAction() }
    catch (e) { setErr((e as Error).message) }
    finally { setBusy(false) }
  }

  return (
    <tr className="border-b">
      <td className="px-3 py-2 font-medium">{t.name}</td>
      <td className="px-3 py-2 text-sm text-gray-600">{t.schedule}</td>
      <td className="px-3 py-2 text-sm">{t.state.last_run || '—'}</td>
      <td className="px-3 py-2 text-sm">
        {dead ? <span className="text-red-700">dead-lettered</span> :
         t.state.consecutive_failures > 0 ? <span className="text-yellow-700">retrying</span> :
         <span className="text-green-700">healthy</span>}
      </td>
      <td className="px-3 py-2 text-sm">
        <button disabled={busy} onClick={() => fire(runTask)} className="px-2 py-1 rounded border mr-2 disabled:opacity-50">Run</button>
        {dead && <button disabled={busy} onClick={() => fire(retryTask)} className="px-2 py-1 rounded border">Retry</button>}
        {err && <span className="ml-2 text-red-700 text-xs">{err}</span>}
      </td>
    </tr>
  )
}
