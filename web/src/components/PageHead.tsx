import type { ReactNode } from 'react'

export function PageHead(props: { title: ReactNode; meta?: ReactNode; actions?: ReactNode }): JSX.Element {
  return (
    <div style={{ display: 'flex', alignItems: 'baseline', gap: 14, marginBottom: 4 }}>
      <h1
        style={{
          fontSize: 22,
          fontWeight: 600,
          color: 'var(--text-strong)',
          letterSpacing: '-0.01em',
          margin: 0,
        }}
      >
        {props.title}
      </h1>
      {props.meta && (
        <div className="mono" style={{ fontSize: 11, color: 'var(--text-muted)' }}>
          {props.meta}
        </div>
      )}
      {props.actions && (
        <div style={{ marginLeft: 'auto', display: 'flex', gap: 6 }}>{props.actions}</div>
      )}
    </div>
  )
}
