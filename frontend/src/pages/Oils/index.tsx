import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useNavigate } from 'react-router-dom'
import { ChevronRight, AlertTriangle } from 'lucide-react'
import { api } from '@/api/client'
import type { BatchView, Compound, Lot } from '@/types'
import { batchDateLabel, fmtMg, lotTotal } from '@/lib/oilDisplay'
import { ProvenanceChips } from '@/components/oils/Provenance'
import { CoaLinks } from '@/components/oils/CoaLinks'

type Tab = 'batches' | 'lots'

// Read-only: lots and batches are entered by talking to Claude through the
// MCP connector, so there are no add/edit buttons here.
export function OilsPage() {
  const [tab, setTab] = useState<Tab>('batches')

  return (
    <div className="p-4">
      <h1 className="text-lg font-semibold mb-3">Oils</h1>

      <div className="flex gap-1 p-1 mb-4 bg-surface border border-border rounded-xl">
        {(['batches', 'lots'] as const).map(t => (
          <button
            key={t}
            onClick={() => setTab(t)}
            className={`flex-1 py-1.5 rounded-lg text-sm capitalize transition-colors ${
              tab === t ? 'bg-raised text-primary' : 'text-muted'
            }`}
          >
            {t}
          </button>
        ))}
      </div>

      {tab === 'batches' ? <BatchList /> : <LotList />}
    </div>
  )
}

// ── Batches ──────────────────────────────────────────────────────────────────

function BatchList() {
  const navigate = useNavigate()
  const { data, isLoading, error } = useQuery({ queryKey: ['oil-batches'], queryFn: api.oils.listBatches })

  if (isLoading) return <div className="text-muted text-center py-16">Loading…</div>
  if (error) return <div className="text-red-400 text-center py-16">{(error as Error).message}</div>
  if (!data?.length) {
    return <div className="text-muted text-center py-16">No batches yet. Log one by telling Claude what you made.</div>
  }

  return (
    <div className="space-y-3">
      {data.map(v => (
        <BatchCard key={v.batch.batchId} view={v} onOpen={() => navigate(`/oils/${encodeURIComponent(v.batch.code)}`)} />
      ))}
    </div>
  )
}

function BatchCard({ view, onOpen }: { view: BatchView; onOpen: () => void }) {
  const { batch, profile, profileError } = view
  return (
    <button onClick={onOpen} className="w-full text-left p-4 bg-surface rounded-xl border border-border active:opacity-80">
      <div className="flex items-center gap-2">
        <span className="font-mono text-sm font-semibold text-primary truncate">{batch.code}</span>
        {profile?.fromLabCoa && <span className="text-[10px] px-1.5 py-0.5 rounded bg-forest text-mint">lab tested</span>}
        <ChevronRight size={16} className="ml-auto text-muted flex-shrink-0" />
      </div>
      <p className="text-xs text-dim mt-0.5">
        {batchDateLabel(batch.date)} · {batch.totalGrams} g
      </p>

      {profileError && (
        <p className="flex items-center gap-1 text-xs text-red-400 mt-2">
          <AlertTriangle size={13} /> {profileError}
        </p>
      )}

      {profile && (
        <div className="grid grid-cols-2 gap-3 mt-3">
          <TotalTile label="CBD" c={profile.totalCbd} />
          <TotalTile label="THC" c={profile.totalThc} />
        </div>
      )}
    </button>
  )
}

function TotalTile({ label, c }: { label: string; c: Compound }) {
  const reported = Object.keys(c.shares).length > 0 || c.mgPerG > 0
  return (
    <div>
      <p className="text-[11px] uppercase tracking-wide text-muted">{label}</p>
      {reported ? (
        <>
          <p className="text-sm text-primary">
            <span className="font-semibold">{fmtMg(c.mgPerG)}</span>
            {c.mgPerG > 0 && <span className="text-dim"> mg/g · {fmtMg(c.mgPerMl)} mg/mL</span>}
          </p>
          <ProvenanceChips shares={c.shares} className="mt-1" />
        </>
      ) : (
        <p className="text-sm text-muted">not reported</p>
      )}
    </div>
  )
}

// ── Lots ─────────────────────────────────────────────────────────────────────

function LotList() {
  const { data, isLoading, error } = useQuery({ queryKey: ['oil-lots'], queryFn: api.oils.listLots })

  if (isLoading) return <div className="text-muted text-center py-16">Loading…</div>
  if (error) return <div className="text-red-400 text-center py-16">{(error as Error).message}</div>
  if (!data?.length) {
    return <div className="text-muted text-center py-16">No lots yet. Give Claude a COA to record one.</div>
  }

  const sorted = [...data].sort((a, b) => b.createdAt.localeCompare(a.createdAt))
  return (
    <div className="space-y-3">
      {sorted.map(lot => <LotCard key={lot.lotId} lot={lot} />)}
    </div>
  )
}

function LotCard({ lot }: { lot: Lot }) {
  const cbd = lotTotal(lot.profile, 'cbd', 'cbda')
  const thc = lotTotal(lot.profile, 'd9thc', 'thca')
  return (
    <div className="p-4 bg-surface rounded-xl border border-border">
      <p className="font-semibold text-primary truncate">{lot.supplier} · {lot.product}</p>
      <p className="text-xs text-dim">
        Lot <span className="font-mono">{lot.supplierLotNumber}</span>
        {lot.gramsPurchased ? ` · ${lot.gramsPurchased} g` : ''}
      </p>
      <div className="flex gap-4 mt-2 text-sm">
        <span><span className="text-muted text-xs">CBD </span>{cbd === undefined ? '—' : `${fmtMg(cbd)} mg/g`}</span>
        <span><span className="text-muted text-xs">THC </span>{thc === undefined ? '—' : `${fmtMg(thc)} mg/g`}</span>
      </div>
      <div className="mt-2">
        <CoaLinks url={lot.coaUrl} s3Key={lot.coaS3Key} />
      </div>
      {lot.notes && <p className="text-xs text-muted mt-2">{lot.notes}</p>}
    </div>
  )
}
