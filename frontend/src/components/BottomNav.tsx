import { NavLink } from 'react-router-dom'
import { Leaf, Wind, Activity, FlaskConical, Pipette } from 'lucide-react'

const tabs = [
  { to: '/plants',       label: 'Plants',       Icon: Leaf         },
  { to: '/overview',     label: 'Activity',     Icon: Activity     },
  { to: '/environments', label: 'Environments', Icon: Wind         },
  { to: '/inventory',    label: 'Inventory',    Icon: FlaskConical },
  { to: '/oils',         label: 'Oils',         Icon: Pipette      },
]

// Tabs size to their labels and the free space is split evenly between them
// (justify-between). Equal-width columns centered the icons but left short
// labels ("Plants", "Oils") with big visible gaps next to "Environments".
export function BottomNav() {
  return (
    <nav className="flex justify-between px-2 bg-surface border-t border-border pb-safe">
      {tabs.map(({ to, label, Icon }) => (
        <NavLink
          key={to}
          to={to}
          className={({ isActive }) =>
            `flex flex-col items-center justify-center px-2 py-3 gap-1 text-xs transition-colors ${
              isActive ? 'text-fern' : 'text-muted'
            }`
          }
        >
          <Icon size={20} strokeWidth={1.5} />
          <span className="whitespace-nowrap">{label}</span>
        </NavLink>
      ))}
    </nav>
  )
}
