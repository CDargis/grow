import type { Compound, OilProfile, ProvenanceSource } from '@/types'

// Display order: lab-backed first, then estimates from most to least trusted.
export const SOURCE_ORDER: ProvenanceSource[] = ['lab', 'high', 'medium', 'low']

export const SOURCE_STYLE: Record<ProvenanceSource, { label: string; bar: string; text: string }> = {
  lab:    { label: 'lab',    bar: 'bg-fern',       text: 'text-fern' },
  high:   { label: 'high',   bar: 'bg-sky-400',    text: 'text-sky-400' },
  medium: { label: 'medium', bar: 'bg-amber-400',  text: 'text-amber-400' },
  low:    { label: 'low',    bar: 'bg-red-400',    text: 'text-red-400' },
}

// A stored 0 means the lab tested for it and found none, so show "ND"
// rather than a number that reads like a measurement.
export function fmtMg(v: number): string {
  if (v === 0) return 'ND'
  if (v >= 100) return v.toFixed(0)
  if (v >= 10) return v.toFixed(1)
  if (v >= 1) return v.toFixed(2)
  return v.toPrecision(2)
}

// "60% medium · 40% lab" -- whole percents, dropping shares that round to 0.
export function sharesText(shares: Compound['shares']): string {
  const parts = SOURCE_ORDER
    .filter(s => (shares[s] ?? 0) > 0)
    .map(s => ({ s, pct: Math.round((shares[s] ?? 0) * 100) }))
    .filter(p => p.pct > 0)
  if (parts.length === 0) return ''
  if (parts.length === 1 && parts[0].s === 'lab') return 'lab'
  return parts.map(p => `${p.pct}% ${SOURCE_STYLE[p.s].label}`).join(' · ')
}

export function isEstimated(shares: Compound['shares']): boolean {
  return SOURCE_ORDER.some(s => s !== 'lab' && (shares[s] ?? 0) > 0)
}

// Same convention the backend uses: total = neutral + 0.877 × acid.
// Undefined when the lot's COA reports neither.
export function lotTotal(profile: OilProfile, neutral: string, acid: string): number | undefined {
  const c = profile.cannabinoids ?? {}
  if (!(neutral in c) && !(acid in c)) return undefined
  return (c[neutral] ?? 0) + 0.877 * (c[acid] ?? 0)
}

export function batchDateLabel(date?: string): string {
  if (!date) return 'date unknown'
  const d = new Date(`${date}T00:00:00`)
  return d.toLocaleDateString(undefined, { month: 'short', day: 'numeric', year: 'numeric' })
}
