import type { Compound, ProvenanceSource } from '@/types'
import { SOURCE_ORDER, SOURCE_STYLE } from '@/lib/oilDisplay'

// Where a compound's number comes from, as small tinted pills: one per
// source, sized by its share of the compound's mg (lab-backed vs flower
// estimates by confidence). Deliberately not a bar -- a full-width bar reads
// as progress, and an all-lab one looks like "100% complete".
export function ProvenanceChips({ shares, className = '' }: { shares: Compound['shares']; className?: string }) {
  const parts = SOURCE_ORDER
    .map(s => ({ s, pct: Math.round((shares[s] ?? 0) * 100) }))
    .filter(p => p.pct > 0)
  if (parts.length === 0) return null

  return (
    <span className={`inline-flex flex-wrap gap-1 ${className}`}>
      {parts.map(({ s, pct }) => (
        <Chip key={s} source={s}>
          {parts.length === 1 ? SOURCE_STYLE[s].label : `${pct}% ${SOURCE_STYLE[s].label}`}
        </Chip>
      ))}
    </span>
  )
}

function Chip({ source, children }: { source: ProvenanceSource; children: React.ReactNode }) {
  const { chip } = SOURCE_STYLE[source]
  return (
    <span className={`inline-flex items-center rounded-full px-1.5 py-px text-[10px] font-medium leading-4 whitespace-nowrap ${chip}`}>
      {children}
    </span>
  )
}

export function ProvenanceLegend() {
  return (
    <div className="flex flex-wrap items-center gap-x-3 gap-y-1.5 text-xs text-muted">
      {SOURCE_ORDER.map(s => (
        <span key={s} className="flex items-center gap-1.5">
          <Chip source={s}>{SOURCE_STYLE[s].label}</Chip>
          {s === 'lab' ? 'from a COA' : 'confidence estimate'}
        </span>
      ))}
    </div>
  )
}
