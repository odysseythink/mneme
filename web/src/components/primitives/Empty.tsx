import type { ReactNode } from 'react'

export function Empty(props: { icon?: ReactNode; title: ReactNode; hint?: ReactNode }): JSX.Element {
  return (
    <div
      style={{
        textAlign: 'center',
        padding: '32px 16px',
        color: 'var(--text-muted)',
        fontSize: 12,
      }}
    >
      {props.icon !== undefined && (
        <div style={{ fontSize: 25, opacity: 0.3, marginBottom: 8 }}>{props.icon}</div>
      )}
      <div>{props.title}</div>
      {props.hint && (
        <div style={{ fontSize: 11, marginTop: 6, color: 'var(--text-faint)' }}>{props.hint}</div>
      )}
    </div>
  )
}
