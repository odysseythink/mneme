import { Link, NavLink } from 'react-router-dom'
import type { ReactNode } from 'react'
import { useActiveProject } from '../hooks/useActiveProject'
import { ProjectPicker } from './ProjectPicker'

type NavItem = { to: string; label: string; icon?: string; badge?: ReactNode }

export function Sidebar(): JSX.Element {
  const { active } = useActiveProject()
  const q = active ? `?project=${encodeURIComponent(active)}` : ''

  const groups: { label: string; items: NavItem[] }[] = [
    {
      label: 'GLOBAL',
      items: [
        { to: '/', label: 'Overview', icon: '▣' },
        { to: '/activity', label: 'Activity', icon: '≣' },
        { to: '/cron', label: 'Cron', icon: '⏱' },
      ],
    },
    {
      label: 'PROJECT',
      items: [
        { to: '/cerebrum' + q, label: 'Cerebrum', icon: '⊕' },
        { to: '/memory', label: 'Memory', icon: '▭' },
        { to: '/anatomy' + q, label: 'Anatomy', icon: '⌘' },
        { to: '/buglog' + q, label: 'BugLog', icon: '!' },
        { to: '/suggestions' + q, label: 'Suggestions', icon: '∴' },
      ],
    },
    {
      label: 'TOOLS',
      items: [
        { to: '/token' + q, label: 'Token', icon: '◐' },
        { to: '/designqc' + q, label: 'DesignQC', icon: '◭' },
      ],
    },
  ]

  return (
    <aside
      style={{
        width: 'var(--sidebar-width)',
        background: 'var(--bg-base)',
        borderRight: '1px solid var(--border-default)',
        padding: '14px 0',
        display: 'flex',
        flexDirection: 'column',
        gap: 2,
        minHeight: '100vh',
      }}
    >
      <div
        style={{
          padding: '0 14px 14px',
          fontWeight: 600,
          color: 'var(--text-strong)',
          fontSize: 13,
          display: 'flex',
          alignItems: 'center',
          gap: 8,
          borderBottom: '1px solid var(--border-default)',
          marginBottom: 8,
        }}
      >
        <Link to="/" style={{ display: 'flex', alignItems: 'center', gap: 8, color: 'inherit', textDecoration: 'none' }}>
          <span
            style={{
              width: 14,
              height: 14,
              background: 'linear-gradient(135deg, var(--accent), color-mix(in srgb, var(--accent) 70%, black))',
              borderRadius: 3,
              display: 'inline-block',
            }}
          />
          mneme
        </Link>
      </div>
      <div style={{ padding: '0 14px 8px' }}>
        <ProjectPicker />
      </div>
      {groups.map((g) => (
        <div key={g.label}>
          <div
            style={{
              fontSize: 9,
              textTransform: 'uppercase',
              letterSpacing: '0.08em',
              color: 'var(--text-faint)',
              fontWeight: 600,
              padding: '12px 14px 4px',
            }}
          >
            {g.label}
          </div>
          {g.items.map((it) => (
            <NavLink
              key={it.to}
              to={it.to}
              end={it.to === '/'}
              style={({ isActive }) => ({
                display: 'flex',
                alignItems: 'center',
                gap: 8,
                padding: '5px 14px',
                paddingLeft: isActive ? 12 : 14,
                color: isActive ? 'var(--accent)' : 'var(--text-muted)',
                background: isActive ? 'var(--bg-selected)' : 'transparent',
                borderLeft: isActive ? '2px solid var(--accent)' : 'none',
                fontSize: 11,
                textDecoration: 'none',
              })}
            >
              {it.icon && (
                <span className="mono" style={{ width: 12, opacity: 0.7, fontSize: 10 }}>
                  {it.icon}
                </span>
              )}
              {it.label}
              {it.badge && <span style={{ marginLeft: 'auto' }}>{it.badge}</span>}
            </NavLink>
          ))}
        </div>
      ))}
    </aside>
  )
}
