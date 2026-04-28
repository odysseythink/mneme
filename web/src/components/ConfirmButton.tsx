import { useEffect, useRef, useState } from 'react'

type ConfirmButtonProps = {
  onConfirm: () => Promise<void> | void
  label: string
  confirmLabel?: string
  className?: string
}

export function ConfirmButton({ onConfirm, label, confirmLabel = 'Confirm?', className = '' }: ConfirmButtonProps): JSX.Element {
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
    <span className="inline-flex items-center gap-2">
      <button
        type="button"
        disabled={busy}
        onClick={armed ? fire : arm}
        className={'px-2 py-1 rounded border text-sm disabled:opacity-50 ' + (armed ? 'bg-red-100 border-red-400' : 'hover:bg-gray-100') + ' ' + className}
      >
        {armed ? confirmLabel : label}
      </button>
      {err && <span className="text-xs text-red-700">{err}</span>}
    </span>
  )
}
