import type { ReactNode } from 'react'

export function Kbd({ children }: { children: ReactNode }): JSX.Element {
  return (
    <kbd
      style={{
        fontFamily: 'var(--font-mono)',
        fontSize: 10,
        padding: '1px 5px',
        borderRadius: 'var(--radius-1)',
        background: 'var(--bg-base)',
        border: '1px solid var(--border-default)',
        color: 'var(--text-muted)',
      }}
    >
      {children}
    </kbd>
  )
}
