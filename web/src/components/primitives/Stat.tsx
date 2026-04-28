import type { ReactNode } from 'react'

export type StatDelta = { value: string; direction: 'up' | 'down' }

export function Stat(props: {
  label: string
  value: ReactNode
  delta?: StatDelta
  breakdown?: ReactNode
  sparkline?: ReactNode
}): JSX.Element {
  const { label, value, delta, breakdown, sparkline } = props
  return (
    <div
      style={{
        background: 'var(--bg-surface)',
        border: '1px solid var(--border-default)',
        borderRadius: 'var(--radius-3)',
        padding: 'var(--space-3)',
        minWidth: 120,
      }}
    >
      <div
        style={{
          fontSize: 10,
          textTransform: 'uppercase',
          letterSpacing: '0.06em',
          color: 'var(--text-muted)',
          fontWeight: 600,
          marginBottom: 6,
        }}
      >
        {label}
      </div>
      <div
        className="mono"
        style={{
          fontSize: 21,
          fontWeight: 500,
          color: 'var(--text-strong)',
        }}
      >
        {value}
        {delta && (
          <span
            className="mono"
            style={{
              fontSize: 11,
              color: delta.direction === 'up' ? 'var(--ok)' : 'var(--err)',
              marginLeft: 6,
            }}
          >
            {delta.direction === 'up' ? '↑' : '↓'}
            {delta.value}
          </span>
        )}
      </div>
      {breakdown && (
        <div
          className="mono"
          style={{
            fontSize: 10,
            color: 'var(--text-muted)',
            marginTop: 6,
          }}
        >
          {breakdown}
        </div>
      )}
      {sparkline && <div style={{ marginTop: 6, height: 18 }}>{sparkline}</div>}
    </div>
  )
}
