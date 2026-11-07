import type { Compound } from '@/types'
import { SOURCE_ORDER, SOURCE_STYLE, sharesText } from '@/lib/oilDisplay'

// Thin stacked bar: how much of a compound's mg is lab-backed vs estimated,
// by confidence. Widths are the backend's contribution-weighted shares.
export function ProvenanceBar({ shares, className = '' }: { shares: Compound['shares']; className?: string }) {
  return (
    <div className={`flex h-1.5 w-full overflow-hidden rounded-full bg-raised ${className}`}>
      {SOURCE_ORDER.map(s => {
        const share = shares[s] ?? 0
        if (share <= 0) return null
        return <div key={s} className={SOURCE_STYLE[s].bar} style={{ width: `${share * 100}%` }} />
      })}
    </div>
  )
}

export function ProvenanceText({ shares, className = '' }: { shares: Compound['shares']; className?: string }) {
  const text = sharesText(shares)
  if (!text) return null
  return <span className={`text-xs text-muted ${className}`}>{text}</span>
}

export function ProvenanceLegend() {
  return (
    <div className="flex flex-wrap gap-x-3 gap-y-1 text-xs text-muted">
      {SOURCE_ORDER.map(s => (
        <span key={s} className="flex items-center gap-1">
          <span className={`inline-block h-2 w-2 rounded-full ${SOURCE_STYLE[s].bar}`} />
          {s === 'lab' ? 'lab (COA)' : `${SOURCE_STYLE[s].label} confidence estimate`}
        </span>
      ))}
    </div>
  )
}
