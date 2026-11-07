import type { OilProfile, ProvenanceSource } from '@/types'

// Display order: lab-backed first, then estimates from most to least trusted.
export const SOURCE_ORDER: ProvenanceSource[] = ['lab', 'high', 'medium', 'low']

export const SOURCE_STYLE: Record<ProvenanceSource, { label: string; chip: string; text: string }> = {
  lab:    { label: 'lab',    chip: 'bg-fern/15 text-fern',           text: 'text-fern' },
  high:   { label: 'high',   chip: 'bg-sky-400/15 text-sky-300',     text: 'text-sky-400' },
  medium: { label: 'medium', chip: 'bg-amber-400/15 text-amber-300', text: 'text-amber-400' },
  low:    { label: 'low',    chip: 'bg-red-400/15 text-red-300',     text: 'text-red-400' },
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
