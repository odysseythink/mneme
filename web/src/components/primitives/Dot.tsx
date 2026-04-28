export type DotStatus = 'ok' | 'warn' | 'err' | 'offline'

const COLOR: Record<DotStatus, string> = {
  ok: 'var(--ok)',
  warn: 'var(--warn)',
  err: 'var(--err)',
  offline: 'var(--text-faint)',
}

export function Dot({ status }: { status: DotStatus }): JSX.Element {
  const color = COLOR[status]
  const isOk = status === 'ok'
  return (
    <span
      style={{
        display: 'inline-block',
        width: 6,
        height: 6,
        borderRadius: '50%',
        background: color,
        boxShadow: isOk ? `0 0 6px color-mix(in srgb, ${color} 50%, transparent)` : 'none',
        animation: isOk ? 'breathe 2s ease-in-out infinite' : 'none',
        verticalAlign: 'middle',
      }}
    />
  )
}
