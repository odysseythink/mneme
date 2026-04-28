import { useState } from 'react'
import { useSSEStatus } from '../hooks/useSSE'
import { useTheme } from '../hooks/useTheme'
import { Modal } from './primitives/Modal'
import { Button } from './primitives/Button'
import { Kbd } from './primitives/Kbd'
import { Dot } from './primitives/Dot'

export function TopBar(): JSX.Element {
  const { connected, latencyMs } = useSSEStatus()
  const { preference, setNext } = useTheme()
  const [searchOpen, setSearchOpen] = useState(false)

  const status = !connected ? 'offline' : latencyMs !== null && latencyMs > 30_000 ? 'err' : latencyMs !== null && latencyMs > 5_000 ? 'warn' : 'ok'
  const label = !connected ? 'SSE · offline' : latencyMs === null ? 'SSE · waiting' : `SSE · ${latencyMs}ms`
  const labelColor = status === 'err' || status === 'offline' ? 'var(--err)' : status === 'warn' ? 'var(--warn)' : 'var(--text-muted)'

  return (
    <header
      style={{
        height: 'var(--header-height)',
        borderBottom: '1px solid var(--border-default)',
        display: 'flex',
        alignItems: 'center',
        padding: '0 14px',
        gap: 14,
        background: 'var(--bg-surface)',
      }}
    >
      <button
        onClick={() => setSearchOpen(true)}
        style={{
          flex: 1,
          maxWidth: 300,
          padding: '5px 10px',
          borderRadius: 'var(--radius-2)',
          border: '1px solid var(--border-default)',
          background: 'var(--bg-base)',
          fontSize: 12,
          color: 'var(--text-muted)',
          display: 'flex',
          alignItems: 'center',
          gap: 6,
          cursor: 'pointer',
          textAlign: 'left',
        }}
      >
        ⌕ Search files, rules, bugs…
        <span style={{ marginLeft: 'auto' }}>
          <Kbd>⌘K</Kbd>
        </span>
      </button>

      <div style={{ flex: 1 }} />

      <div className="mono" style={{ display: 'flex', alignItems: 'center', gap: 6, fontSize: 11, color: labelColor }}>
        <Dot status={status === 'offline' ? 'offline' : status === 'err' ? 'err' : status === 'warn' ? 'warn' : 'ok'} />
        {label}
      </div>

      <button
        onClick={setNext}
        title={`Theme: ${preference}`}
        style={{
          width: 26,
          height: 26,
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          borderRadius: 'var(--radius-2)',
          color: 'var(--text-muted)',
          background: 'transparent',
          border: 'none',
          cursor: 'pointer',
          fontSize: 14,
        }}
      >
        ◐
      </button>

      <Modal
        open={searchOpen}
        onClose={() => setSearchOpen(false)}
        title="Search"
        actions={<Button onClick={() => setSearchOpen(false)}>Close</Button>}
      >
        Search not yet implemented — coming in a follow-up spec.
      </Modal>
    </header>
  )
}
