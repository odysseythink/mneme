import type { ReactNode } from 'react'

type Props = {
  open: boolean
  onClose: () => void
  title: ReactNode
  children: ReactNode
  actions?: ReactNode
}

export function Modal({ open, onClose, title, children, actions }: Props): JSX.Element | null {
  if (!open) return null
  return (
    <div
      data-testid="modal-backdrop"
      onClick={onClose}
      style={{
        position: 'fixed',
        inset: 0,
        background: 'rgba(0,0,0,0.5)',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        zIndex: 100,
      }}
    >
      <div
        onClick={(e) => e.stopPropagation()}
        style={{
          background: 'var(--bg-surface)',
          border: '1px solid var(--border-strong)',
          borderRadius: 'var(--radius-4)',
          padding: 20,
          boxShadow: '0 20px 50px rgba(0,0,0,0.5)',
          maxWidth: 400,
          width: '90%',
        }}
      >
        <div style={{ fontSize: 15, fontWeight: 600, color: 'var(--text-strong)', marginBottom: 8 }}>
          {title}
        </div>
        <div style={{ fontSize: 12, color: 'var(--text-muted)', marginBottom: 14, lineHeight: 1.5 }}>
          {children}
        </div>
        {actions && (
          <div style={{ display: 'flex', gap: 8, justifyContent: 'flex-end' }}>{actions}</div>
        )}
      </div>
    </div>
  )
}
