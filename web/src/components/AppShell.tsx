import { Outlet } from 'react-router-dom'
import { Sidebar } from './Sidebar'
import { TopBar } from './TopBar'

export function AppShell(): JSX.Element {
  return (
    <div
      style={{
        display: 'grid',
        gridTemplateColumns: 'var(--sidebar-width) 1fr',
        gridTemplateRows: 'var(--header-height) 1fr',
        minHeight: '100vh',
      }}
    >
      <div style={{ gridColumn: 1, gridRow: '1 / 3' }}>
        <Sidebar />
      </div>
      <div style={{ gridColumn: 2, gridRow: 1 }}>
        <TopBar />
      </div>
      <main
        style={{
          gridColumn: 2,
          gridRow: 2,
          padding: 'var(--panel-pad)',
          overflow: 'auto',
          display: 'flex',
          flexDirection: 'column',
          gap: 12,
        }}
      >
        <Outlet />
      </main>
    </div>
  )
}
