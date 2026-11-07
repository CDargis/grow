import { useMemo } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { ArrowLeft, AlertTriangle } from 'lucide-react'
import { api } from '@/api/client'
import type { Batch, BatchInput, BatchProfile, Compound, Lot, Plant } from '@/types'
import { batchDateLabel, fmtMg, SOURCE_STYLE } from '@/lib/oilDisplay'
import { ProvenanceChips, ProvenanceLegend } from '@/components/oils/Provenance'
import { CoaLinks } from '@/components/oils/CoaLinks'

// The ROBP (Record of Batch Profile): one batch's inputs and its calculated
// profile, with the lab vs estimated split for every compound.
export function OilBatchPage() {
  const { batch: batchParam = '' } = useParams()
  const navigate = useNavigate()

  const { data: view, isLoading, error } = useQuery({
    queryKey: ['oil-batch', batchParam],
    queryFn: () => api.oils.getBatch(batchParam),
  })
  // Used only to label inputs (lot names, source batch codes, plant names).
  const { data: lots } = useQuery({ queryKey: ['oil-lots'], queryFn: api.oils.listLots })
  const { data: batches } = useQuery({ queryKey: ['oil-batches'], queryFn: api.oils.listBatches })
  const needsPlants = view?.batch.inputs.some(i => i.kind === 'flower' && i.plantId) ?? false
  const { data: plants } = useQuery({ queryKey: ['plants'], queryFn: api.plants.list, enabled: needsPlants })

  const lookups = useMemo(() => ({
    lots: new Map((lots ?? []).map(l => [l.lotId, l])),
    batches: new Map((batches ?? []).map(v => [v.batch.batchId, v.batch])),
    plants: new Map((plants ?? []).map(p => [p.plantId, p])),
  }), [lots, batches, plants])

  return (
    <div className="pb-8">
      <div className="flex items-center gap-2 p-4 border-b border-border">
        <button onClick={() => navigate(-1)} className="text-dim active:opacity-70 mr-1" aria-label="Back">
          <ArrowLeft size={20} />
        </button>
        <div className="min-w-0">
          <p className="font-mono font-semibold text-primary truncate">{view?.batch.code ?? batchParam}</p>
          <p className="text-xs text-muted">Record of Batch Profile</p>
        </div>
      </div>

      {isLoading && <div className="text-muted text-center py-16">Loading…</div>}
      {error && <div className="text-red-400 text-center py-16 px-4">{(error as Error).message}</div>}

      {view && (
        <div className="p-4 space-y-5">
          <Summary batch={view.batch} />

          {view.profileError && (
            <p className="flex items-start gap-2 text-sm text-red-400">
              <AlertTriangle size={16} className="flex-shrink-0 mt-0.5" /> {view.profileError}
            </p>
          )}

          {view.profile && <Totals profile={view.profile} />}

          <Section title="Inputs">
            <div className="space-y-2">
              {view.batch.inputs.map((input, i) => (
                <InputRow key={i} input={input} lookups={lookups} />
              ))}
              {view.batch.carrier && (
                <Row
                  title={view.batch.carrier.name}
                  sub="carrier"
                  grams={view.batch.carrier.grams}
                />
              )}
            </div>
          </Section>

          {view.profile && <ProfileTable profile={view.profile} />}

          {view.batch.labCoa && (
            <Section title="Batch lab COA">
              <CoaLinks url={view.batch.labCoa.url} s3Key={view.batch.labCoa.s3Key} />
            </Section>
          )}

          {view.batch.notes && (
            <Section title="Notes">
              <p className="text-sm text-dim whitespace-pre-wrap">{view.batch.notes}</p>
            </Section>
          )}
        </div>
      )}
    </div>
  )
}

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <section>
      <h2 className="text-xs uppercase tracking-wide text-muted mb-2">{title}</h2>
      {children}
    </section>
  )
}

function Summary({ batch }: { batch: Batch }) {
  return (
    <div className="grid grid-cols-3 gap-2 text-center">
      <Stat label="Made" value={batchDateLabel(batch.date)} />
      <Stat label="Total" value={`${batch.totalGrams} g`} />
      <Stat label="Density" value={`${batch.densityGPerMl} g/mL`} />
    </div>
  )
}

function Stat({ label, value }: { label: string; value: string }) {
  return (
    <div className="p-2 bg-surface border border-border rounded-lg">
      <p className="text-[11px] text-muted">{label}</p>
      <p className="text-sm text-primary">{value}</p>
    </div>
  )
}

function Totals({ profile }: { profile: BatchProfile }) {
  return (
    <div className="space-y-2">
      <div className="grid grid-cols-2 gap-3">
        <TotalCard label="Total CBD" c={profile.totalCbd} />
        <TotalCard label="Total THC" c={profile.totalThc} />
      </div>
      <p className="text-xs text-muted">
        {profile.fromLabCoa
          ? 'Numbers from this batch’s own lab COA.'
          : 'Calculated from the inputs, not lab tested.'}
      </p>
    </div>
  )
}

function TotalCard({ label, c }: { label: string; c: Compound }) {
  const reported = Object.keys(c.shares).length > 0 || c.mgPerG > 0
  return (
    <div className="p-3 bg-surface border border-border rounded-xl">
      <p className="text-xs text-muted">{label}</p>
      {reported ? (
        <>
          <p className="text-xl font-semibold text-primary">{fmtMg(c.mgPerG)}{c.mgPerG > 0 && <span className="text-xs font-normal text-dim"> mg/g</span>}</p>
          {c.mgPerG > 0 && <p className="text-sm text-dim">{fmtMg(c.mgPerMl)} mg/mL</p>}
          <ProvenanceChips shares={c.shares} className="mt-2" />
        </>
      ) : (
        <p className="text-sm text-muted mt-1">not reported</p>
      )}
    </div>
  )
}

interface Lookups {
  lots: Map<string, Lot>
  batches: Map<string, Batch>
  plants: Map<string, Plant>
}

function InputRow({ input, lookups }: { input: BatchInput; lookups: Lookups }) {
  switch (input.kind) {
    case 'lot': {
      const lot = input.lotId ? lookups.lots.get(input.lotId) : undefined
      return (
        <Row
          title={lot ? `${lot.supplier} · ${lot.product}` : 'Lot'}
          sub={lot ? `lot ${lot.supplierLotNumber}` : input.lotId}
          grams={input.grams}
          extra={lot && <CoaLinks url={lot.coaUrl} s3Key={lot.coaS3Key} />}
        />
      )
    }
    case 'batch': {
      const src = input.batchId ? lookups.batches.get(input.batchId) : undefined
      return (
        <Row
          title={src
            ? <Link to={`/oils/${encodeURIComponent(src.code)}`} className="font-mono text-fern">{src.code}</Link>
            : 'Batch'}
          sub="earlier batch"
          grams={input.grams}
        />
      )
    }
    case 'flower': {
      const plant = input.plantId ? lookups.plants.get(input.plantId) : undefined
      const conf = input.confidence
      return (
        <Row
          title={input.strain ?? 'Flower'}
          sub={`flower · ${input.method ?? ''}${input.efficiency ? ` · ${Math.round(input.efficiency * 100)}% efficiency` : ''}`}
          grams={input.grams}
          extra={
            <div className="flex flex-wrap items-center gap-x-3 text-xs">
              {conf && <span className={SOURCE_STYLE[conf].text}>{conf} confidence estimate</span>}
              {input.plantId && (
                <Link to={`/plants/${input.plantId}`} className="text-fern">
                  {plant ? plant.name : 'plant'}
                </Link>
              )}
            </div>
          }
        />
      )
    }
    case 'additive':
      return (
        <Row
          title={input.name ?? 'Additive'}
          sub={input.note ? `additive · ${input.note}` : 'additive'}
          grams={input.grams}
        />
      )
  }
}

function Row({ title, sub, grams, extra }: { title: React.ReactNode; sub?: string; grams: number; extra?: React.ReactNode }) {
  return (
    <div className="p-3 bg-surface border border-border rounded-xl">
      <div className="flex items-baseline gap-2">
        <div className="min-w-0 flex-1">
          <p className="text-sm text-primary truncate">{title}</p>
          {sub && <p className="text-xs text-muted truncate">{sub}</p>}
        </div>
        <p className="text-sm text-dim flex-shrink-0">{grams} g</p>
      </div>
      {extra && <div className="mt-1.5">{extra}</div>}
    </div>
  )
}

function ProfileTable({ profile }: { profile: BatchProfile }) {
  // When a lab COA supplied the numbers, show what the calculation said
  // alongside, so the estimate's accuracy is visible.
  const calcByKey = new Map((profile.calculated ?? []).map(c => [`${c.group}:${c.key}`, c]))
  const groups: Array<{ title: string; group: Compound['group'] }> = [
    { title: 'Cannabinoids', group: 'cannabinoid' },
    { title: 'Terpenes', group: 'terpene' },
    { title: 'Other', group: 'other' },
  ]

  return (
    <Section title="Profile">
      <div className="space-y-4">
        {groups.map(({ title, group }) => {
          const rows = profile.compounds.filter(c => c.group === group)
          if (rows.length === 0) return null
          return (
            <div key={group}>
              <p className="text-xs text-dim mb-1">{title}</p>
              <div className="bg-surface border border-border rounded-xl divide-y divide-border">
                <div className="flex px-3 py-1.5 text-[11px] text-muted">
                  <span className="flex-1">Compound</span>
                  <span className="w-16 text-right">mg/g</span>
                  <span className="w-16 text-right">mg/mL</span>
                  {profile.fromLabCoa && <span className="w-14 text-right">calc</span>}
                </div>
                {rows.map(c => {
                  const calc = calcByKey.get(`${c.group}:${c.key}`)
                  return (
                    <div key={c.key} className="px-3 py-2">
                      <div className="flex items-baseline text-sm">
                        <span className="flex-1 text-primary truncate">{c.name}</span>
                        <span className="w-16 text-right text-primary">{fmtMg(c.mgPerG)}</span>
                        <span className="w-16 text-right text-dim">{c.mgPerG > 0 ? fmtMg(c.mgPerMl) : ''}</span>
                        {profile.fromLabCoa && (
                          <span className="w-14 text-right text-muted">{calc ? fmtMg(calc.mgPerG) : '—'}</span>
                        )}
                      </div>
                      <ProvenanceChips shares={c.shares} className="mt-1" />
                    </div>
                  )
                })}
              </div>
            </div>
          )
        })}
        <ProvenanceLegend />
      </div>
    </Section>
  )
}
