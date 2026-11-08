import { NavLink } from 'react-router-dom'
import { Leaf, Wind, Activity, FlaskConical, Pipette } from 'lucide-react'

const tabs = [
  { to: '/plants',       label: 'Plants',       Icon: Leaf         },
  { to: '/overview',     label: 'Activity',     Icon: Activity     },
  { to: '/environments', label: 'Environments', Icon: Wind         },
  { to: '/inventory',    label: 'Inventory',    Icon: FlaskConical },
  { to: '/oils',         label: 'Oils',         Icon: Pipette      },
]

// Equal-width grid columns: with flex-1 the long "Environments" label pushed
// its column wider and the icons sat unevenly.
export function BottomNav() {
  return (
    <nav className="grid grid-cols-5 bg-surface border-t border-border pb-safe">
      {tabs.map(({ to, label, Icon }) => (
        <NavLink
          key={to}
          to={to}
          className={({ isActive }) =>
            `flex flex-col items-center justify-center min-w-0 py-3 gap-1 text-[10px] transition-colors ${
              isActive ? 'text-fern' : 'text-muted'
            }`
          }
        >
          <Icon size={20} strokeWidth={1.5} />
          <span className="max-w-full truncate px-0.5">{label}</span>
        </NavLink>
      ))}
    </nav>
  )
}
