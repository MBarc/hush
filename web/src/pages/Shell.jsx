import { NavLink, useNavigate } from 'react-router-dom'
import { api } from '../api'
import { useMe } from '../App'
import { Logo, RoleBadge } from '../components/ui'

const nav = [
  { to: '/', label: 'Secrets', icon: FolderIcon, end: true },
  { to: '/tokens', label: 'Tokens', icon: KeyIcon },
  { to: '/devices', label: 'Devices', icon: DeviceIcon },
  { to: '/users', label: 'Users', icon: UsersIcon, admin: true },
  { to: '/audit', label: 'Audit', icon: PulseIcon, admin: true },
]

export default function Shell({ children }) {
  const { me, setMe } = useMe()
  const navigate = useNavigate()

  const logout = async () => {
    await api.logout().catch(() => {})
    setMe(null)
    navigate('/')
  }

  return (
    <div className="flex min-h-screen">
      <aside className="flex w-60 flex-col border-r border-border bg-surface">
        <div className="flex h-16 items-center gap-2.5 border-b border-border px-5">
          <Logo size={26} />
          <span className="text-lg font-bold tracking-tight">hush</span>
        </div>

        <nav className="flex-1 space-y-1 p-3">
          {nav
            .filter((n) => !n.admin || me.admin)
            .map((n) => (
              <NavLink
                key={n.to}
                to={n.to}
                end={n.end}
                className={({ isActive }) =>
                  `flex items-center gap-3 rounded-control px-3 py-2 text-sm transition-colors ${
                    isActive
                      ? 'bg-raised text-primary'
                      : 'text-secondary hover:bg-raised/60 hover:text-primary'
                  }`
                }
              >
                <n.icon />
                {n.label}
              </NavLink>
            ))}
        </nav>

        <div className="border-t border-border p-3">
          <div className="mb-2 flex items-center justify-between px-2">
            <span className="mono text-sm text-primary">{me.username}</span>
            <RoleBadge role={me.role} />
          </div>
          <button onClick={logout} className="btn-ghost w-full">
            Sign out
          </button>
        </div>
      </aside>

      <main className="flex-1 overflow-x-hidden">{children}</main>
    </div>
  )
}

export function PageHeader({ title, subtitle, children }) {
  return (
    <div className="flex h-16 items-center justify-between border-b border-border px-8">
      <div>
        <h1 className="text-lg font-semibold leading-tight">{title}</h1>
        {subtitle && <p className="text-xs text-muted">{subtitle}</p>}
      </div>
      <div className="flex items-center gap-2">{children}</div>
    </div>
  )
}

// --- icons (stroke, 18px) ---

function FolderIcon() {
  return (
    <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8">
      <path d="M3 7a2 2 0 0 1 2-2h4l2 2h8a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V7z" />
    </svg>
  )
}
function KeyIcon() {
  return (
    <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8">
      <circle cx="8" cy="15" r="4" />
      <path d="M10.8 12.2 20 3M17 6l2 2M14 9l1.5 1.5" />
    </svg>
  )
}
function DeviceIcon() {
  return (
    <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8">
      <rect x="3" y="4" width="18" height="12" rx="2" />
      <path d="M8 20h8M12 16v4" />
    </svg>
  )
}
function UsersIcon() {
  return (
    <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8">
      <circle cx="9" cy="8" r="3" />
      <path d="M3 20a6 6 0 0 1 12 0M16 6.5a3 3 0 0 1 0 5.5M21 20a6 6 0 0 0-4-5.6" />
    </svg>
  )
}
function PulseIcon() {
  return (
    <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8">
      <path d="M3 12h4l2 6 4-14 2 8h6" />
    </svg>
  )
}
