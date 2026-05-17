import { NavLink } from 'react-router-dom'
import { Sprout, Leaf, Wind } from 'lucide-react'

const tabs = [
  { to: '/garden',       label: 'Garden',       Icon: Sprout },
  { to: '/plants',       label: 'Plants',        Icon: Leaf   },
  { to: '/environments', label: 'Environments',  Icon: Wind   },
]

export function BottomNav() {
  return (
    <nav className="flex bg-surface border-t border-border pb-safe">
      {tabs.map(({ to, label, Icon }) => (
        <NavLink
          key={to}
          to={to}
          className={({ isActive }) => `
            flex flex-col items-center justify-center flex-1 py-3 gap-1
            text-xs transition-colors
            ${isActive ? 'text-fern' : 'text-muted hover:text-dim'}
          `}
        >
          <Icon size={20} strokeWidth={1.5} />
          <span>{label}</span>
        </NavLink>
      ))}
    </nav>
  )
}
