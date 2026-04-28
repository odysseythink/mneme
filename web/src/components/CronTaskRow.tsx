import { useState } from 'react'
import type { CronTask } from '../api/types'
import { runTask, retryTask } from '../api/cron'
import { Button } from './primitives/Button'
import { Dot } from './primitives/Dot'

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

  const status: 'ok' | 'warn' | 'err' = dead ? 'err' : t.state.consecutive_failures > 0 ? 'warn' : 'ok'
  const label = dead ? 'dead-lettered' : t.state.consecutive_failures > 0 ? 'retrying' : 'healthy'
  const labelColor = status === 'err' ? 'var(--err)' : status === 'warn' ? 'var(--warn)' : 'var(--ok)'

  const td = (extra?: React.CSSProperties): React.CSSProperties => ({
    padding: '6px 10px',
    fontSize: 12,
    borderBottom: '1px solid color-mix(in srgb, var(--border-default) 40%, transparent)',
    ...extra,
  })

  return (
    <tr>
      <td style={td({ fontWeight: 500, color: 'var(--text-strong)' })}>{t.name}</td>
      <td style={td({ color: 'var(--text-muted)' })} className="mono">{t.schedule}</td>
      <td style={td({ color: 'var(--text-body)' })} className="mono">{t.state.last_run || '—'}</td>
      <td style={td({ display: 'flex', alignItems: 'center', gap: 6 })}>
        <Dot status={status} />
        <span style={{ color: labelColor }}>{label}</span>
      </td>
      <td style={td({ display: 'flex', gap: 6, alignItems: 'center' })}>
        <Button size="sm" disabled={busy} onClick={() => fire(runTask)}>Run</Button>
        {dead && <Button size="sm" variant="danger" disabled={busy} onClick={() => fire(retryTask)}>Retry</Button>}
        {err && <span style={{ marginLeft: 4, color: 'var(--err)', fontSize: 11 }}>{err}</span>}
      </td>
    </tr>
  )
}
