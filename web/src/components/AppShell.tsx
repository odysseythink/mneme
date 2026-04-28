import { Link, NavLink, Outlet } from 'react-router-dom'
import { useSSEConnected } from '../hooks/useSSE'
import { useActiveProject } from '../hooks/useActiveProject'
import { ProjectPicker } from './ProjectPicker'

export function AppShell(): JSX.Element {
  const connected = useSSEConnected()
  const { active } = useActiveProject()
  const q = active ? `?project=${encodeURIComponent(active)}` : ''

  return (
    <div className="flex min-h-screen">
      <aside className="w-60 border-r bg-gray-50 p-4">
        <Link to="/" className="block text-lg font-semibold mb-4">mneme</Link>
        <div className="mb-4"><ProjectPicker /></div>
        <nav className="flex flex-col gap-1">
          <NavLink to="/" end className={navClass}>Overview</NavLink>
          <NavLink to="/activity" className={navClass}>Activity</NavLink>
          <NavLink to="/cron" className={navClass}>Cron</NavLink>
          <hr className="my-2" />
          <NavLink to={'/cerebrum' + q} className={navClass}>Cerebrum</NavLink>
          <NavLink to="/memory" className={navClass}>Memory</NavLink>
          <NavLink to={'/anatomy' + q} className={navClass}>Anatomy</NavLink>
          <NavLink to={'/buglog' + q} className={navClass}>BugLog</NavLink>
          <NavLink to={'/suggestions' + q} className={navClass}>Suggestions</NavLink>
          <NavLink to={'/token' + q} className={navClass}>Token</NavLink>
          <NavLink to={'/designqc' + q} className={navClass}>DesignQC</NavLink>
        </nav>
        <div className="mt-8 text-xs text-gray-500">
          stream: <span className={connected ? 'text-green-600' : 'text-red-600'}>{connected ? 'live' : 'offline'}</span>
        </div>
      </aside>
      <main className="flex-1 p-6 overflow-auto"><Outlet /></main>
    </div>
  )
}

function navClass({ isActive }: { isActive: boolean }) {
  return 'px-2 py-1 rounded ' + (isActive ? 'bg-gray-200 font-medium' : 'hover:bg-gray-100')
}
