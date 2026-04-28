import type { ReactNode } from 'react'

export type ToastTone = 'ok' | 'err' | 'warn' | 'info'

const COLOR: Record<ToastTone, string> = {
  ok: 'var(--ok)',
  err: 'var(--err)',
  warn: 'var(--warn)',
  info: 'var(--info)',
}

export function Toast(props: { tone?: ToastTone; children: ReactNode }): JSX.Element {
  const tone = props.tone ?? 'ok'
  return (
    <div
      style={{
        padding: '10px 14px',
        borderRadius: 'var(--radius-3)',
        background: 'var(--bg-raised)',
        border: '1px solid var(--border-default)',
        borderLeft: `3px solid ${COLOR[tone]}`,
        fontSize: 11,
        color: 'var(--text-body)',
        display: 'flex',
        alignItems: 'center',
        gap: 8,
      }}
    >
      {props.children}
    </div>
  )
}
