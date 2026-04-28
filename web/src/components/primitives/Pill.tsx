import type { ReactNode } from 'react'

export type PillVariant = 'write' | 'read' | 'warn' | 'err' | 'info' | 'neutral' | 'solid'

const COLOR: Record<Exclude<PillVariant, 'solid'>, string> = {
  write: 'var(--accent)',
  read: 'var(--ok)',
  warn: 'var(--warn)',
  err: 'var(--err)',
  info: 'var(--info)',
  neutral: 'var(--neutral)',
}

export function Pill(props: { variant: PillVariant; children: ReactNode }): JSX.Element {
  const base = {
    display: 'inline-block',
    padding: '1px 6px',
    borderRadius: 'var(--radius-1)',
    fontFamily: 'var(--font-mono)',
    fontSize: 9,
    fontWeight: 500 as const,
    lineHeight: 1.4,
  }
  if (props.variant === 'solid') {
    return (
      <span style={{ ...base, background: 'var(--accent)', color: 'var(--bg-base)', fontWeight: 600 }}>
        {props.children}
      </span>
    )
  }
  const c = COLOR[props.variant]
  return (
    <span
      style={{
        ...base,
        color: c,
        background: `color-mix(in srgb, ${c} 15%, transparent)`,
      }}
    >
      {props.children}
    </span>
  )
}
