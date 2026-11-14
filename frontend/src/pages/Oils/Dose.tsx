import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { AlertTriangle, ChevronDown } from 'lucide-react'
import { api } from '@/api/client'
import type { DoseLineView, DoseVersion, DoseVersionView, DoseView, ShelfUnit } from '@/types'
import { batchDateLabel } from '@/lib/oilDisplay'
import { ProvenanceChips } from '@/components/oils/Provenance'

// The dose card. Read-only: the plan and shelf are set by talking to Claude.
// Every mL/piece amount is worked out server-side from the shelf's current
// bottles, so opening a new batch changes the mL here without a plan change.
export function DoseTab() {
  const { data, isLoading, error } = useQuery({ queryKey: ['dose'], queryFn: api.oils.getDose })
  const [showDraft, setShowDraft] = useState(false)

  if (isLoading) return <div className="text-muted text-center py-16">Loading…</div>
  if (error) return <div className="text-red-400 text-center py-16">{(error as Error).message}</div>
  if (!data?.current && !data?.draft) {
    return <div className="text-muted text-center py-16">No dose plan yet. Tell Claude your regimen.</div>
  }

  const viewingDraft = showDraft && !!data.draft
  const shown = viewingDraft ? data.draft! : (data.current ?? data.draft!)
  const compareTo = viewingDraft ? data.current : undefined

  return (
    <div className="space-y-4">
      {data.current && data.draft && (
        <div className="flex gap-1 p-1 bg-surface border border-border rounded-xl">
          {([false, true] as const).map(draft => (
            <button
              key={String(draft)}
              onClick={() => setShowDraft(draft)}
              className={`flex-1 py-1.5 rounded-lg text-sm transition-colors ${
                viewingDraft === draft ? 'bg-raised text-primary' : 'text-muted'
              }`}
            >
              {draft ? 'Draft' : 'Current'}
            </button>
          ))}
        </div>
      )}

      <VersionHeader v={shown} />

      {shown.times.map(t => (
        <div key={t.name} className="p-4 bg-surface rounded-xl border border-border">
          <div className="flex items-baseline justify-between pb-2 border-b border-border">
            <span className="text-xs uppercase tracking-wide text-muted">{t.name}</span>
            <span className="text-xs text-dim font-mono">{mgLabel(t.totalCbd, t.totalThc)}</span>
          </div>
          <div className="divide-y divide-border">
            {t.lines.map((l, i) => <Line key={i} l={l} />)}
          </div>
        </div>
      ))}

      <DayTotal v={shown} was={compareTo} />
      <Shelf shelf={data.shelf} />
      {data.history.length > 0 && <History history={data.history} />}
    </div>
  )
}

function VersionHeader({ v }: { v: DoseVersionView }) {
  const { version, status } = v
  let when: string
  if (status === 'current') when = `since ${batchDateLabel(version.startDate)}`
  else if (version.startDate) when = `starts ${batchDateLabel(version.startDate)}`
  else when = 'draft · not scheduled'
  return (
    <div>
      {version.name && <p className="font-semibold text-primary">{version.name}</p>}
      <p className="text-xs text-dim">{when}</p>
      {version.notes && <p className="text-xs text-muted mt-1">{version.notes}</p>}
    </div>
  )
}

function Line({ l }: { l: DoseLineView }) {
  return (
    <div className="flex items-center gap-3 py-3">
      <div className="min-w-0 flex-1">
        <p className="text-primary truncate">{l.shelfName || 'Unknown shelf item'}</p>
        <p className="text-xs text-dim truncate">
          {l.mg} mg {l.compound.toUpperCase()}
          {l.source && (
            <> · {l.batchId
              ? <Link to={`/oils/${encodeURIComponent(l.source)}`} className="font-mono text-fern">{l.source}</Link>
              : l.source}
            </>
          )}
        </p>
        {l.shares && <ProvenanceChips shares={l.shares} className="mt-1" />}
        {l.error && (
          <p className="flex items-center gap-1 text-xs text-red-400 mt-1">
            <AlertTriangle size={12} /> {l.error}
          </p>
        )}
        {l.note && <p className="text-xs text-muted mt-1">{l.note}</p>}
      </div>
      {!l.error && l.unit && (
        <span className="font-mono text-primary text-xl whitespace-nowrap">
          {fmtAmount(l.amount, l.unit)}
          <span className="text-xs text-dim ml-1">{unitLabel(l.unit, l.unitName, l.amount)}</span>
        </span>
      )}
    </div>
  )
}

function DayTotal({ v, was }: { v: DoseVersionView; was?: DoseVersionView }) {
  const wasShelf = new Map(was?.shelf.map(s => [s.shelfItemId, s]))
  const wasLots = new Map(was?.rawLots.map(r => [r.lotId, r]))
  return (
    <div className="p-4 bg-surface rounded-xl border border-border">
      <p className="text-xs uppercase tracking-wide text-muted mb-2">Day total</p>
      <Row label="CBD" value={`${round(v.totalCbd)} mg`} was={was && `${round(was.totalCbd)} mg`} strong />
      {(v.totalThc > 0 || (was?.totalThc ?? 0) > 0) && (
        <Row label="THC" value={`${round(v.totalThc, 1)} mg`} was={was && `${round(was.totalThc, 1)} mg`} strong />
      )}
      {v.minors > 0 && <Row label="Minor cannabinoids" value={`${round(v.minors, 1)} mg`} was={was && `${round(was.minors, 1)} mg`} />}

      {v.shelf.length > 0 && <Divider label="Per day · per 30 days" />}
      {v.shelf.map(s => {
        const w = wasShelf.get(s.shelfItemId)
        return (
          <Row
            key={s.shelfItemId}
            label={s.name}
            value={`${fmtAmount(s.perDay, s.unit)} · ${round(s.per30Days, s.unit === 'mL' ? 0 : 1)} ${unitLabel(s.unit, s.unitName, s.per30Days)}`}
            was={was && (w ? `${round(w.per30Days, w.unit === 'mL' ? 0 : 1)} / 30d` : 'none')}
          />
        )
      })}

      {v.rawLots.length > 0 && <Divider label="Raw material per 30 days" />}
      {v.rawLots.map(r => {
        const w = wasLots.get(r.lotId)
        return (
          <Row key={r.lotId} label={r.label} value={`${round(r.gramsPer30Days, 1)} g`}
            was={was && (w ? `${round(w.gramsPer30Days, 1)} g` : 'none')} />
        )
      })}
    </div>
  )
}

function Row({ label, value, was, strong }: { label: string; value: string; was?: string; strong?: boolean }) {
  return (
    <div className="flex items-baseline justify-between gap-3 py-1">
      <span className="text-sm text-dim min-w-0 truncate">{label}</span>
      <span className="text-right whitespace-nowrap">
        <span className={`font-mono ${strong ? 'text-lg text-primary' : 'text-sm text-primary'}`}>{value}</span>
        {was !== undefined && <span className="block text-[11px] text-muted">current {was}</span>}
      </span>
    </div>
  )
}

function Divider({ label }: { label: string }) {
  return <p className="text-[11px] uppercase tracking-wide text-muted mt-3 mb-1 pt-3 border-t border-border">{label}</p>
}

function Shelf({ shelf }: { shelf: DoseView['shelf'] }) {
  const [open, setOpen] = useState(false)
  // Only to name past batch sources in the history lines.
  const { data: batches } = useQuery({ queryKey: ['oil-batches'], queryFn: api.oils.listBatches, enabled: open })
  const codes = new Map((batches ?? []).map(v => [v.batch.batchId, v.batch.code]))
  if (shelf.length === 0) return null
  return (
    <div className="bg-surface rounded-xl border border-border">
      <button onClick={() => setOpen(o => !o)} className="w-full flex items-center justify-between p-4 text-left">
        <span className="font-semibold text-primary">Shelf</span>
        <ChevronDown size={18} className={`text-muted transition-transform ${open ? 'rotate-180' : ''}`} />
      </button>
      {open && (
        <div className="px-4 pb-4 divide-y divide-border">
          {shelf.map(({ item, strength, strengthError }) => (
            <div key={item.shelfItemId} className="py-3">
              <div className="flex items-baseline justify-between gap-3">
                <span className="text-primary">{item.name}</span>
                {strength && (
                  <span className="text-xs font-mono text-dim whitespace-nowrap">
                    {strengthLabel(strength.totalCbd, strength.totalThc, strength.unit, strength.unitName)}
                  </span>
                )}
              </div>
              <p className="text-xs text-dim">
                {strength?.batchId
                  ? <Link to={`/oils/${encodeURIComponent(strength.label)}`} className="font-mono text-fern">{strength.label}</Link>
                  : (strength?.label ?? item.source.productName ?? item.source.batchId)}
                {' '}· since {batchDateLabel(item.since)}
              </p>
              {strengthError && (
                <p className="flex items-center gap-1 text-xs text-red-400 mt-1"><AlertTriangle size={12} /> {strengthError}</p>
              )}
              {item.history?.slice().reverse().map((h, i) => (
                <p key={i} className="text-[11px] text-muted">
                  was {h.source.productName ?? codes.get(h.source.batchId ?? '') ?? 'a batch'} {batchDateLabel(h.from)} – {batchDateLabel(h.to)}
                </p>
              ))}
            </div>
          ))}
        </div>
      )}
    </div>
  )
}

function History({ history }: { history: DoseView['history'] }) {
  const [open, setOpen] = useState(false)
  return (
    <div className="bg-surface rounded-xl border border-border">
      <button onClick={() => setOpen(o => !o)} className="w-full flex items-center justify-between p-4 text-left">
        <span className="font-semibold text-primary">Past plans</span>
        <ChevronDown size={18} className={`text-muted transition-transform ${open ? 'rotate-180' : ''}`} />
      </button>
      {open && (
        <div className="px-4 pb-4 divide-y divide-border">
          {history.map(({ version, shelfNames }) => <PastVersion key={version.versionId} v={version} names={shelfNames} />)}
        </div>
      )}
    </div>
  )
}

function PastVersion({ v, names }: { v: DoseVersion; names: Record<string, string> }) {
  return (
    <div className="py-3">
      <p className="text-xs text-dim">
        {batchDateLabel(v.startDate)} – {batchDateLabel(v.endDate)}
        {v.name && <span className="text-primary"> · {v.name}</span>}
      </p>
      {v.times.map(t => (
        <p key={t.name} className="text-sm text-primary mt-1">
          <span className="text-muted text-xs uppercase tracking-wide mr-2">{t.name}</span>
          {t.lines.map(l => `${l.mg} mg ${l.compound.toUpperCase()} ${names[l.shelfItemId] ?? ''}`.trim()).join(' · ')}
        </p>
      ))}
    </div>
  )
}

// ── Formatting ───────────────────────────────────────────────────────────────

// Pieces read as fractions ('½ gummy'); mL as a dropper reading.
function fmtAmount(v: number, unit: ShelfUnit): string {
  if (unit === 'mL') return v < 1 ? v.toFixed(2) : v.toFixed(1)
  const whole = Math.floor(v + 1e-9)
  const rem = v - whole
  if (rem < 1e-9) return String(whole)
  const sym = [[0.25, '¼'], [0.5, '½'], [0.75, '¾']].find(([f]) => Math.abs(rem - (f as number)) < 1e-6)?.[1]
  if (!sym) return v.toFixed(2)
  return whole > 0 ? `${whole}${sym}` : (sym as string)
}

function unitLabel(unit: ShelfUnit, unitName: string | undefined, amount: number): string {
  if (unit === 'mL') return 'mL'
  const name = unitName || 'piece'
  return amount > 1 + 1e-9 ? `${name}s` : name
}

function strengthLabel(cbd: number, thc: number, unit: ShelfUnit, unitName?: string): string {
  const per = unit === 'mL' ? 'mL' : (unitName || 'piece')
  const parts = []
  if (cbd > 0) parts.push(`${round(cbd, 1)} mg CBD`)
  if (thc > 0) parts.push(`${round(thc, 1)} mg THC`)
  return parts.length ? `${parts.join(' · ')} /${per}` : '—'
}

function mgLabel(cbd: number, thc: number): string {
  const parts = []
  if (cbd > 0) parts.push(`${round(cbd)} mg CBD`)
  if (thc > 0) parts.push(`${round(thc, 1)} mg THC`)
  return parts.join(' · ')
}

function round(v: number, digits = 0): string {
  return Number(v.toFixed(digits)).toString()
}
