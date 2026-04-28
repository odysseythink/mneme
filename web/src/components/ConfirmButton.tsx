import { useEffect, useRef, useState } from 'react'
import { Button } from './primitives/Button'

type ConfirmButtonProps = {
  onConfirm: () => Promise<void> | void
  label: string
  confirmLabel?: string
}

export function ConfirmButton({ onConfirm, label, confirmLabel = 'Confirm?' }: ConfirmButtonProps): JSX.Element {
  const [armed, setArmed] = useState(false)
  const [busy, setBusy] = useState(false)
  const [err, setErr] = useState<string | null>(null)
  const timer = useRef<number | null>(null)

  useEffect(() => () => { if (timer.current) window.clearTimeout(timer.current) }, [])

  const arm = () => {
    setArmed(true)
    if (timer.current) window.clearTimeout(timer.current)
    timer.current = window.setTimeout(() => setArmed(false), 5000)
  }

  const fire = async () => {
    setBusy(true); setErr(null)
    try { await onConfirm(); setArmed(false) }
    catch (e) { setErr((e as Error).message) }
    finally { setBusy(false) }
  }

  return (
    <span style={{ display: 'inline-flex', alignItems: 'center', gap: 8 }}>
      <Button
        size="sm"
        variant={armed ? 'danger' : 'default'}
        disabled={busy}
        onClick={armed ? fire : arm}
      >
        {armed ? confirmLabel : label}
      </Button>
      {err && <span style={{ fontSize: 10, color: 'var(--err)' }}>{err}</span>}
    </span>
  )
}
