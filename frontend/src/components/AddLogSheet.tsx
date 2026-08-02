import { useState, useRef, useEffect } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Droplets, Ruler, MessageSquare, Camera, GitBranch, Scissors, ImagePlus, X } from 'lucide-react'
import { BottomSheet } from './BottomSheet'
import { MediaImage } from './MediaImage'
import { NutrientAutocomplete } from './NutrientAutocomplete'
import { api } from '@/api/client'
import { scaledFullDose, pctOfDose as computePctOfDose, deliveredGrams, convertDoseAmount, isConvertibleDoseUnit, CONVERTIBLE_DOSE_UNITS } from '@/lib/nutrientDose'
import type { Plant, Log, LogType, WateringData, TrainingData, TrimmingData, NoteData, PhotoData, TransplantData, HeightData, Product, ProductForm, ReferenceDose, NPK } from '@/types'

// ── Types config ─────────────────────────────────────────────────────────────

type TypeConfig = {
  type: LogType
  label: string
  icon: React.ReactNode
  ready: boolean
}

const LOG_TYPES: TypeConfig[] = [
  { type: 'watering',   label: 'Water / Feed', icon: <Droplets  size={22} />, ready: true  },
  { type: 'note',       label: 'Note',     icon: <MessageSquare size={22} />, ready: true },
  { type: 'training',   label: 'Training', icon: <GitBranch size={22} />, ready: true  },
  { type: 'trimming',   label: 'Trim',     icon: <Scissors  size={22} />, ready: true  },
  { type: 'height',     label: 'Height',   icon: <Ruler     size={22} />, ready: true  },
  { type: 'photo',      label: 'Photo',    icon: <Camera    size={22} />, ready: true  },
  { type: 'transplant', label: 'Transplant', icon: <span className="text-xl">🪴</span>, ready: true },
]

const LABELS: Record<LogType, string> = Object.fromEntries(
  LOG_TYPES.map(t => [t.type, t.label])
) as Record<LogType, string>

function nowDatetime(): string {
  const d = new Date()
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth()+1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`
}

function datetimeToDate(dt: string): string {
  return dt.slice(0, 10)
}

function datetimeToISO(dt: string): string {
  return new Date(dt).toISOString()
}

function isoToDatetime(iso: string): string {
  const d = new Date(iso)
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth()+1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`
}

// ── Shared field styles ───────────────────────────────────────────────────────

const inputCls = 'w-full bg-raised border border-border rounded-lg px-3 py-2.5 text-sm text-primary placeholder:text-muted focus:outline-none focus:border-fern'
const labelCls = 'block text-xs text-dim mb-1'

// ── Watering / feeding form ───────────────────────────────────────────────────

type NutrientRow = {
  name: string
  amount: string
  unit: string
  productId?: string
  npk?: NPK
  // Client-side only (not part of the submitted Nutrient shape) -- kept on
  // the row so we can compute pctOfDose/deliveredN-P-K at submit time from
  // whatever batch amount is entered.
  referenceDose?: ReferenceDose
  form?: ProductForm
  elementalNpk?: NPK
  density?: number
}

// Liquid products scale off the log's own water amount, same as always.
// Dry amendments are labeled per pot/container size, not per water volume
// -- using the day's water amount for those was silently wrong (e.g. a
// light 2L watering next to a full-strength dry topdress reads as a
// nonsense 900%+ dose), so they scale off the plant's own pot size instead.
function resolveBatch(
  form: ProductForm | undefined,
  referenceDose: ReferenceDose,
  waterAmount: number,
  waterUnit: string,
  plant: Plant,
): { amount: number; unit: string } | null {
  if (form !== 'dry') {
    return waterAmount > 0 ? { amount: waterAmount, unit: waterUnit } : null
  }
  if (referenceDose.perVolumeUnit === 'in') {
    return plant.potSizeDiameterIn ? { amount: plant.potSizeDiameterIn, unit: 'in' } : null
  }
  return plant.potSizeGal ? { amount: plant.potSizeGal, unit: 'gal' } : null
}

function WizardDoseRow({
  row, plant, waterAmount, waterUnit, onAmountChange, onUnitChange, onRemove,
}: {
  row: NutrientRow
  plant: Plant
  waterAmount: number
  waterUnit: string
  onAmountChange: (v: string) => void
  onUnitChange: (unit: string) => void
  onRemove: () => void
}) {
  const rd = row.referenceDose!
  const isRange = rd.min !== rd.max
  const batch = resolveBatch(row.form, rd, waterAmount, waterUnit, plant)
  const full = batch ? scaledFullDose(rd, batch.amount, batch.unit) : 0
  // full is always computed in the product's own labeled unit (rd.unit) --
  // convert it into whatever unit this row is currently displayed in.
  const fullInRowUnit = convertDoseAmount(full, rd.unit, row.unit) ?? full
  const presets = [
    { label: 'Full', pct: 100 },
    { label: '3/4',  pct: 75 },
    { label: '1/2',  pct: 50 },
    { label: '1/4',  pct: 25 },
  ]
  const amountInRdUnit = row.amount ? convertDoseAmount(Number(row.amount), row.unit, rd.unit) : undefined
  const pct = full > 0 && amountInRdUnit !== undefined ? (amountInRdUnit / full) * 100 : null

  function changeUnit(nextUnit: string) {
    if (row.amount) {
      const converted = convertDoseAmount(Number(row.amount), row.unit, nextUnit)
      if (converted !== undefined) onAmountChange(converted.toFixed(2))
    }
    onUnitChange(nextUnit)
  }

  return (
    <div className="p-3 bg-raised rounded-lg border border-border space-y-2">
      <div className="flex items-center justify-between">
        <span className="text-sm font-medium text-primary">{row.name}</span>
        <button type="button" onClick={onRemove} className="text-muted active:opacity-60">✕</button>
      </div>
      <p className="text-xs text-muted">
        Labeled: {isRange ? `${rd.min}–${rd.max}` : rd.min} {rd.unit} / {rd.perVolume}{rd.perVolumeUnit === 'in' ? 'in pot diameter' : ` ${rd.perVolumeUnit}`}
      </p>
      <div className="flex gap-1.5 flex-wrap">
        {presets.map(({ label, pct: p }) => (
          <button
            key={label}
            type="button"
            disabled={!fullInRowUnit}
            onClick={() => onAmountChange((fullInRowUnit * p / 100).toFixed(2))}
            className="px-2.5 py-1 text-xs rounded-full border border-border text-dim active:bg-surface disabled:opacity-40"
          >
            {label}
          </button>
        ))}
      </div>
      <div className="flex gap-2 items-center">
        <input
          type="number"
          className="w-24 bg-surface border border-border rounded-lg px-2 py-1.5 text-sm text-primary focus:outline-none focus:border-fern"
          placeholder={fullInRowUnit ? fullInRowUnit.toFixed(2) : '0'}
          value={row.amount}
          onChange={e => onAmountChange(e.target.value)}
        />
        {isConvertibleDoseUnit(rd.unit) ? (
          <select
            className="bg-surface border border-border rounded-lg px-2 py-1.5 text-sm text-primary focus:outline-none focus:border-fern"
            value={row.unit}
            onChange={e => changeUnit(e.target.value)}
          >
            {CONVERTIBLE_DOSE_UNITS.map(u => <option key={u} value={u} className="bg-raised">{u}</option>)}
          </select>
        ) : (
          <span className="text-xs text-muted">{rd.unit}</span>
        )}
        {pct !== null && <span className="text-xs text-fern ml-auto">{pct.toFixed(0)}%</span>}
      </div>
      {!batch && (
        <p className="text-[11px] text-amber-400">
          {row.form === 'dry'
            ? "Set this plant's pot size (Edit Plant) to compute suggested doses."
            : 'Enter a water amount above to compute suggested doses.'}
        </p>
      )}
    </div>
  )
}

function WateringForm({ plant, datetime, onSuccess, logId, init }: { plant: Plant; datetime: string; onSuccess: () => void; logId?: string; init?: WateringData }) {
  const plantId = plant.plantId
  const qc = useQueryClient()
  const [amount, setAmount]  = useState(init?.amount?.toString() ?? '')
  const [unit, setUnit]      = useState<WateringData['unit']>(init?.unit ?? 'ml')
  const [ph, setPh]          = useState(init?.ph?.toString() ?? '')
  const [runoff, setRunoff]  = useState(init?.runoff?.toString() ?? '')
  const [tds, setTds]        = useState(init?.tds?.toString() ?? '')
  const [runoffTds, setRunoffTds] = useState(init?.runoffTds?.toString() ?? '')
  const [note, setNote]      = useState(init?.note ?? '')
  const [showNutrients, setShowNutrients] = useState((init?.nutrients?.length ?? 0) > 0)
  const [nutrients, setNutrients] = useState<NutrientRow[]>(
    init?.nutrients?.length
      ? init.nutrients.map(n => ({ name: n.name, amount: String(n.amount), unit: n.unit, productId: n.productId, npk: n.npk }))
      : [{ name: '', amount: '', unit: 'ml' }]
  )
  const [wizardSearch, setWizardSearch] = useState('')

  const { data: settings } = useQuery({ queryKey: ['settings'], queryFn: api.settings.get })
  const { data: products = [] } = useQuery({ queryKey: ['products'], queryFn: api.products.list })

  // Editing an existing log only round-trips the fields stored on the log
  // itself (name/amount/unit/productId/npk) -- referenceDose/form/elementalNpk/
  // density are client-side-only (see NutrientRow), needed to recompute
  // pctOfDose/deliveredN-P-K on save. Without this, re-saving an edited feed
  // silently dropped delivered NPK for every linked product in that log, not
  // just whichever row was actually being edited.
  useEffect(() => {
    if (!products.length) return
    setNutrients(rows => rows.map(r => {
      if (!r.productId || r.referenceDose) return r
      const product = products.find(p => p.productId === r.productId)
      if (!product) return r
      return { ...r, npk: product.npk, referenceDose: product.referenceDose, form: product.form, elementalNpk: product.elementalNpk, density: product.density }
    }))
  }, [products])
  const [mode, setMode] = useState<'wizard' | 'manual'>(logId ? 'manual' : 'wizard')
  const [modeTouched, setModeTouched] = useState(false)
  useEffect(() => {
    if (!logId && !modeTouched && settings?.nutrientEntryMode === 'manual') setMode('manual')
  }, [settings, modeTouched, logId])

  function setModeManually(m: 'wizard' | 'manual') {
    setMode(m)
    setModeTouched(true)
  }

  const batchAmount = Number(amount) || 0

  function updateNutrient(i: number, field: 'name' | 'amount' | 'unit', value: string) {
    setNutrients(rows => rows.map((r, idx) => idx === i ? { ...r, [field]: value } : r))
  }

  function selectProductForRow(i: number, product: Product | null) {
    setNutrients(rows => rows.map((r, idx) => {
      if (idx !== i) return r
      if (!product) {
        return { ...r, productId: undefined, npk: undefined, referenceDose: undefined, form: undefined, elementalNpk: undefined, density: undefined }
      }
      return {
        ...r,
        name: product.name,
        productId: product.productId,
        npk: product.npk,
        referenceDose: product.referenceDose,
        form: product.form,
        elementalNpk: product.elementalNpk,
        density: product.density,
      }
    }))
  }

  function addBlankRow() {
    setNutrients(rows => [...rows, { name: '', amount: '', unit: 'ml' }])
  }

  function addWizardProduct(product: Product) {
    const batch = resolveBatch(product.form, product.referenceDose, batchAmount, unit, plant)
    const full = batch ? scaledFullDose(product.referenceDose, batch.amount, batch.unit) : 0
    const isRange = product.referenceDose.min !== product.referenceDose.max
    // Default to mL when the label's own unit can be freely converted --
    // mL is the preferred entry unit regardless of how the product was
    // labeled; grams stays grams since there's no density-free conversion.
    const doseUnit = isConvertibleDoseUnit(product.referenceDose.unit) ? 'ml' : product.referenceDose.unit
    const fullInDoseUnit = convertDoseAmount(full, product.referenceDose.unit, doseUnit) ?? full
    const prefillAmount = !isRange && fullInDoseUnit > 0 ? fullInDoseUnit.toFixed(2) : ''
    setNutrients(rows => {
      const withoutBlank = rows.length === 1 && !rows[0].name && !rows[0].amount ? [] : rows
      return [...withoutBlank, {
        name: product.name,
        amount: prefillAmount,
        unit: doseUnit,
        productId: product.productId,
        npk: product.npk,
        referenceDose: product.referenceDose,
        form: product.form,
        elementalNpk: product.elementalNpk,
        density: product.density,
      }]
    })
  }

  function removeNutrientRow(i: number) {
    if (nutrients.length === 1) { setShowNutrients(false); setNutrients([{ name: '', amount: '', unit: 'ml' }]) }
    else setNutrients(rows => rows.filter((_, idx) => idx !== i))
  }

  const activeNutrients = showNutrients ? nutrients.filter(n => n.name && n.amount) : []
  const valid = Boolean(amount) || activeNutrients.length > 0

  const nutrientEntries = activeNutrients.map(n => {
    const entry: {
      name: string; amount: number; unit: string
      productId?: string; npk?: NPK; pctOfDose?: number
      deliveredN?: number; deliveredP?: number; deliveredK?: number
    } = {
      name: n.name.trim(),
      amount: Number(n.amount),
      unit: n.unit,
    }
    if (n.productId) entry.productId = n.productId
    if (n.npk) entry.npk = n.npk
    if (n.productId && n.referenceDose) {
      const batch = resolveBatch(n.form, n.referenceDose, batchAmount, unit, plant)
      const pct = batch ? computePctOfDose(Number(n.amount), n.unit, n.referenceDose, batch.amount, batch.unit) : undefined
      if (pct !== undefined) entry.pctOfDose = pct
      if (n.elementalNpk && n.density) {
        const delivered = deliveredGrams(Number(n.amount), n.unit, n.referenceDose.unit, n.elementalNpk, n.density)
        if (delivered) {
          entry.deliveredN = delivered.n
          entry.deliveredP = delivered.p
          entry.deliveredK = delivered.k
        }
      }
    }
    return entry
  })

  const body = {
    logType: 'watering' as const,
    date: datetimeToDate(datetime),
    loggedAt: datetimeToISO(datetime),
    data: {
      ...(amount ? { amount: Number(amount) } : {}),
      unit,
      ...(ph        ? { ph: Number(ph) }               : {}),
      ...(runoff    ? { runoff: Number(runoff) }       : {}),
      ...(tds       ? { tds: Number(tds) }             : {}),
      ...(runoffTds ? { runoffTds: Number(runoffTds) } : {}),
      ...(nutrientEntries.length ? { nutrients: nutrientEntries } : {}),
      ...(note   ? { note }                   : {}),
    } as WateringData,
  }

  const mutation = useMutation({
    mutationFn: () => logId ? api.logs.update(plantId, logId, body) : api.logs.create(plantId, body),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['logs', 'plant', plantId] })
      onSuccess()
    },
  })

  return (
    <div className="space-y-3 mt-2">
      <div>
        <label className={labelCls}>Amount</label>
        <div className="flex gap-2">
          <input
            type="number"
            className={inputCls}
            placeholder="0"
            value={amount}
            onChange={e => setAmount(e.target.value)}
          />
          <select
            className="bg-raised border border-border rounded-lg px-3 py-2.5 text-sm text-primary focus:outline-none focus:border-fern"
            value={unit}
            onChange={e => setUnit(e.target.value as WateringData['unit'])}
          >
            {(['ml','l','oz','gal'] as const).map(u => (
              <option key={u} value={u} className="bg-raised">{u}</option>
            ))}
          </select>
        </div>
      </div>

      <div className="grid grid-cols-2 gap-3">
        <div>
          <label className={labelCls}>pH</label>
          <input type="number" step="0.1" className={inputCls} placeholder="6.2" value={ph} onChange={e => setPh(e.target.value)} />
        </div>
        <div>
          <label className={labelCls}>Runoff pH</label>
          <input type="number" step="0.1" className={inputCls} placeholder="6.5" value={runoff} onChange={e => setRunoff(e.target.value)} />
        </div>
      </div>

      <div className="grid grid-cols-2 gap-3">
        <div>
          <label className={labelCls}>TDS (ppm)</label>
          <input type="number" className={inputCls} placeholder="315" value={tds} onChange={e => setTds(e.target.value)} />
        </div>
        <div>
          <label className={labelCls}>Runoff TDS</label>
          <input type="number" className={inputCls} placeholder="270" value={runoffTds} onChange={e => setRunoffTds(e.target.value)} />
        </div>
      </div>

      {!showNutrients ? (
        <button
          type="button"
          onClick={() => setShowNutrients(true)}
          className="text-xs text-fern active:opacity-70"
        >
          + Add nutrients / amendments
        </button>
      ) : (
        <div>
          <div className="flex items-center justify-between mb-1.5">
            <label className={labelCls + ' mb-0'}>Nutrients / amendments</label>
            <div className="flex gap-0.5 bg-raised rounded-full p-0.5 border border-border">
              {(['wizard', 'manual'] as const).map(m => (
                <button
                  key={m}
                  type="button"
                  onClick={() => setModeManually(m)}
                  className={`px-2.5 py-1 text-[11px] rounded-full transition-colors ${
                    mode === m ? 'bg-fern/20 text-fern' : 'text-muted'
                  }`}
                >
                  {m === 'wizard' ? 'Wizard' : 'Manual'}
                </button>
              ))}
            </div>
          </div>

          <div className="space-y-2">
            {nutrients.map((n, i) => (
              mode === 'wizard' && n.productId && n.referenceDose ? (
                <WizardDoseRow
                  key={i}
                  row={n}
                  plant={plant}
                  waterAmount={batchAmount}
                  waterUnit={unit}
                  onAmountChange={v => updateNutrient(i, 'amount', v)}
                  onUnitChange={u => updateNutrient(i, 'unit', u)}
                  onRemove={() => removeNutrientRow(i)}
                />
              ) : (
                <div key={i} className="flex gap-2 items-center">
                  <NutrientAutocomplete
                    value={n.name}
                    onChange={v => updateNutrient(i, 'name', v)}
                    onSelectProduct={p => selectProductForRow(i, p)}
                    placeholder="Name"
                    inputClassName={inputCls}
                  />
                  <input
                    type="number"
                    className="w-20 bg-raised border border-border rounded-lg px-3 py-2.5 text-sm text-primary focus:outline-none focus:border-fern"
                    placeholder="0"
                    value={n.amount}
                    onChange={e => updateNutrient(i, 'amount', e.target.value)}
                  />
                  <select
                    className="bg-raised border border-border rounded-lg px-2 py-2.5 text-sm text-primary focus:outline-none focus:border-fern"
                    value={n.unit}
                    onChange={e => updateNutrient(i, 'unit', e.target.value)}
                  >
                    {['ml','oz','g','tsp','tbsp'].map(u => <option key={u} value={u} className="bg-raised">{u}</option>)}
                  </select>
                  <button
                    onClick={() => removeNutrientRow(i)}
                    className="text-muted active:opacity-60 flex-shrink-0"
                  >✕</button>
                </div>
              )
            ))}
          </div>

          {mode === 'manual' ? (
            <button
              type="button"
              onClick={addBlankRow}
              className="mt-2 text-xs text-fern active:opacity-70"
            >
              + Add nutrient
            </button>
          ) : (
            <div className="mt-2">
              <NutrientAutocomplete
                value={wizardSearch}
                onChange={setWizardSearch}
                onSelectProduct={p => { if (p) { addWizardProduct(p); setWizardSearch('') } }}
                placeholder="Search products to add…"
                inputClassName={inputCls}
                wrapperClassName="relative"
              />
            </div>
          )}
        </div>
      )}

      <div>
        <label className={labelCls}>Note</label>
        <textarea
          className={`${inputCls} resize-none`}
          rows={2}
          placeholder="Any observations…"
          value={note}
          onChange={e => setNote(e.target.value)}
        />
      </div>

      {mutation.isError && <p className="text-red-400 text-sm">{(mutation.error as Error).message}</p>}

      <button
        onClick={() => { if (valid) mutation.mutate() }}
        disabled={!valid || mutation.isPending}
        className="w-full py-3 bg-fern text-base font-semibold rounded-xl active:opacity-80 disabled:opacity-50"
      >
        {mutation.isPending
          ? 'Saving…'
          : activeNutrients.length
            ? (amount ? 'Log Feeding' : 'Log Top Dress')
            : 'Log Watering'}
      </button>
    </div>
  )
}

// ── Note form ─────────────────────────────────────────────────────────────────

function NoteForm({ plantId, datetime, onSuccess, logId, init }: { plantId: string; datetime: string; onSuccess: () => void; logId?: string; init?: NoteData }) {
  const qc = useQueryClient()
  const [text, setText] = useState(init?.text ?? '')

  const mutation = useMutation({
    mutationFn: () => {
      const body = { logType: 'note' as const, date: datetimeToDate(datetime), loggedAt: datetimeToISO(datetime), data: { text } as NoteData }
      return logId ? api.logs.update(plantId, logId, body) : api.logs.create(plantId, body)
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['logs', 'plant', plantId] })
      onSuccess()
    },
  })

  return (
    <div className="space-y-3 mt-2">
      <textarea
        className={inputCls + ' h-32 resize-none'}
        placeholder="Write a note…"
        value={text}
        onChange={e => setText(e.target.value)}
      />

      {mutation.isError && <p className="text-red-400 text-sm">{(mutation.error as Error).message}</p>}

      <button
        onClick={() => { if (text.trim()) mutation.mutate() }}
        disabled={!text.trim() || mutation.isPending}
        className="w-full py-3 bg-fern text-base font-semibold rounded-xl active:opacity-80 disabled:opacity-50"
      >
        {mutation.isPending ? 'Saving…' : 'Save Note'}
      </button>
    </div>
  )
}

// ── Height form ───────────────────────────────────────────────────────────────

function HeightForm({ plantId, datetime, onSuccess, logId, init }: { plantId: string; datetime: string; onSuccess: () => void; logId?: string; init?: HeightData }) {
  const qc = useQueryClient()
  const [height, setHeight] = useState(init?.height?.toString() ?? '')
  const [unit, setUnit]     = useState<HeightData['unit']>(init?.unit ?? 'in')

  const mutation = useMutation({
    mutationFn: () => {
      const body = { logType: 'height' as const, date: datetimeToDate(datetime), loggedAt: datetimeToISO(datetime), data: { height: Number(height), unit } as HeightData }
      return logId ? api.logs.update(plantId, logId, body) : api.logs.create(plantId, body)
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['logs', 'plant', plantId] })
      onSuccess()
    },
  })

  return (
    <div className="space-y-3 mt-2">
      <div>
        <label className={labelCls}>Height</label>
        <div className="flex gap-2">
          <input
            type="number"
            step="0.1"
            className={inputCls}
            placeholder="0"
            value={height}
            onChange={e => setHeight(e.target.value)}
          />
          <select
            className="bg-raised border border-border rounded-lg px-3 py-2.5 text-sm text-primary focus:outline-none focus:border-fern"
            value={unit}
            onChange={e => setUnit(e.target.value as HeightData['unit'])}
          >
            <option value="cm" className="bg-raised">cm</option>
            <option value="in" className="bg-raised">in</option>
          </select>
        </div>
      </div>

      {mutation.isError && <p className="text-red-400 text-sm">{(mutation.error as Error).message}</p>}

      <button
        onClick={() => { if (height) mutation.mutate() }}
        disabled={!height || mutation.isPending}
        className="w-full py-3 bg-fern text-base font-semibold rounded-xl active:opacity-80 disabled:opacity-50"
      >
        {mutation.isPending ? 'Saving…' : 'Log Height'}
      </button>
    </div>
  )
}

// ── Transplant form ───────────────────────────────────────────────────────────

function TransplantForm({ plantId, datetime, onSuccess, logId, init }: { plantId: string; datetime: string; onSuccess: () => void; logId?: string; init?: TransplantData }) {
  const qc = useQueryClient()
  const [potSize, setPotSize] = useState(init?.potSize ?? '')
  const [medium, setMedium]   = useState(init?.medium ?? '')

  const mutation = useMutation({
    mutationFn: () => {
      const body = { logType: 'transplant' as const, date: datetimeToDate(datetime), loggedAt: datetimeToISO(datetime), data: { potSize, ...(medium ? { medium } : {}) } as TransplantData }
      return logId ? api.logs.update(plantId, logId, body) : api.logs.create(plantId, body)
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['logs', 'plant', plantId] })
      onSuccess()
    },
  })

  return (
    <div className="space-y-3 mt-2">
      <div>
        <label className={labelCls}>Pot size</label>
        <input
          className={inputCls}
          placeholder="e.g. 3 gal, 5L, 1 gal"
          value={potSize}
          onChange={e => setPotSize(e.target.value)}
        />
      </div>
      <div>
        <label className={labelCls}>Medium (optional)</label>
        <input
          className={inputCls}
          placeholder="e.g. coco, FFOF, ProMix"
          value={medium}
          onChange={e => setMedium(e.target.value)}
        />
      </div>

      {mutation.isError && <p className="text-red-400 text-sm">{(mutation.error as Error).message}</p>}

      <button
        onClick={() => { if (potSize.trim()) mutation.mutate() }}
        disabled={!potSize.trim() || mutation.isPending}
        className="w-full py-3 bg-fern text-base font-semibold rounded-xl active:opacity-80 disabled:opacity-50"
      >
        {mutation.isPending ? 'Saving…' : 'Log Transplant'}
      </button>
    </div>
  )
}

// ── Training form ─────────────────────────────────────────────────────────────

const TRAINING_METHODS = ['LST', 'Topping', 'FIM', 'ScrOG', 'Supercropping', 'Mainlining', 'Other']

function TrainingForm({ plantId, datetime, onSuccess, logId, init }: { plantId: string; datetime: string; onSuccess: () => void; logId?: string; init?: TrainingData }) {
  const qc = useQueryClient()
  const cameraRef = useRef<HTMLInputElement>(null)
  const libraryRef = useRef<HTMLInputElement>(null)
  const [method, setMethod] = useState(init?.method ?? '')
  const [notes, setNotes]   = useState(init?.notes ?? '')
  const [file, setFile]     = useState<File | null>(null)
  const [preview, setPreview] = useState<string | null>(null)
  const [uploading, setUploading] = useState(false)
  const [error, setError]   = useState<string | null>(null)

  function handleFile(e: React.ChangeEvent<HTMLInputElement>) {
    const f = e.target.files?.[0]
    if (!f) return
    setFile(f)
    setPreview(URL.createObjectURL(f))
  }

  async function handleSubmit() {
    if (!method) return
    setUploading(true)
    setError(null)
    try {
      const photoKey = file
        ? await api.media.uploadFile(file, `plants/${plantId}/logs`)
        : init?.photoKey
      const data: TrainingData = { method, ...(notes ? { notes } : {}), ...(photoKey ? { photoKey } : {}) }
      const body = { logType: 'training' as const, date: datetimeToDate(datetime), loggedAt: datetimeToISO(datetime), data }
      if (logId) { await api.logs.update(plantId, logId, body) } else { await api.logs.create(plantId, body) }
      qc.invalidateQueries({ queryKey: ['logs', 'plant', plantId] })
      onSuccess()
    } catch (e) {
      setError((e as Error).message)
    } finally {
      setUploading(false)
    }
  }

  return (
    <div className="space-y-3 mt-2">
      <input ref={cameraRef} type="file" accept="image/*" capture="environment" className="hidden" onChange={handleFile} />
      <input ref={libraryRef} type="file" accept="image/*" className="hidden" onChange={handleFile} />
      <div>
        <label className={labelCls}>Method</label>
        <select className={inputCls} value={method} onChange={e => setMethod(e.target.value)}>
          <option value="">Select method</option>
          {TRAINING_METHODS.map(m => <option key={m} value={m} className="bg-raised">{m}</option>)}
        </select>
      </div>
      <div>
        <label className={labelCls}>Notes (optional)</label>
        <textarea className={inputCls} rows={2} placeholder="Any details…" value={notes} onChange={e => setNotes(e.target.value)} />
      </div>
      {preview || init?.photoKey ? (
        <button
          onClick={() => libraryRef.current?.click()}
          className="w-full h-36 rounded-xl border-2 border-transparent flex items-center justify-center overflow-hidden"
        >
          {preview
            ? <img src={preview} alt="preview" className="w-full h-full object-cover" />
            : <MediaImage photoKey={init!.photoKey!} alt="current" className="w-full h-full object-cover" />
          }
        </button>
      ) : (
        <div className="grid grid-cols-2 gap-2">
          <button
            onClick={() => cameraRef.current?.click()}
            className="h-36 rounded-xl border-2 border-dashed border-border text-muted flex flex-col items-center justify-center gap-2 active:opacity-70"
          >
            <Camera size={24} />
            <span className="text-sm">Take Photo</span>
          </button>
          <button
            onClick={() => libraryRef.current?.click()}
            className="h-36 rounded-xl border-2 border-dashed border-border text-muted flex flex-col items-center justify-center gap-2 active:opacity-70"
          >
            <ImagePlus size={24} />
            <span className="text-sm">Library</span>
          </button>
        </div>
      )}
      {preview && (
        <button onClick={() => { setFile(null); setPreview(null); if (cameraRef.current) cameraRef.current.value = ''; if (libraryRef.current) libraryRef.current.value = '' }} className="text-xs text-muted active:opacity-70 flex items-center gap-1">
          <X size={12} /> Remove photo
        </button>
      )}
      {error && <p className="text-red-400 text-sm">{error}</p>}
      <button onClick={handleSubmit} disabled={!method || uploading} className="w-full py-3 bg-fern text-base font-semibold rounded-xl active:opacity-80 disabled:opacity-50">
        {uploading ? 'Uploading…' : 'Log Training'}
      </button>
    </div>
  )
}

// ── Trimming form ─────────────────────────────────────────────────────────────

const TRIMMING_METHODS = ['Defoliation', 'Lollipopping', 'Schwazzing', 'Fan Leaf Removal', 'Larf Removal', 'Other']

function TrimmingForm({ plantId, datetime, onSuccess, logId, init }: { plantId: string; datetime: string; onSuccess: () => void; logId?: string; init?: TrimmingData }) {
  const qc = useQueryClient()
  const cameraRef = useRef<HTMLInputElement>(null)
  const libraryRef = useRef<HTMLInputElement>(null)
  const [method, setMethod] = useState(init?.method ?? '')
  const [notes, setNotes]   = useState(init?.notes ?? '')
  const [file, setFile]     = useState<File | null>(null)
  const [preview, setPreview] = useState<string | null>(null)
  const [uploading, setUploading] = useState(false)
  const [error, setError]   = useState<string | null>(null)

  function handleFile(e: React.ChangeEvent<HTMLInputElement>) {
    const f = e.target.files?.[0]
    if (!f) return
    setFile(f)
    setPreview(URL.createObjectURL(f))
  }

  async function handleSubmit() {
    setUploading(true)
    setError(null)
    try {
      const photoKey = file
        ? await api.media.uploadFile(file, `plants/${plantId}/logs`)
        : init?.photoKey
      const data: TrimmingData = { ...(method ? { method } : {}), ...(notes ? { notes } : {}), ...(photoKey ? { photoKey } : {}) }
      const body = { logType: 'trimming' as const, date: datetimeToDate(datetime), loggedAt: datetimeToISO(datetime), data }
      if (logId) { await api.logs.update(plantId, logId, body) } else { await api.logs.create(plantId, body) }
      qc.invalidateQueries({ queryKey: ['logs', 'plant', plantId] })
      onSuccess()
    } catch (e) {
      setError((e as Error).message)
    } finally {
      setUploading(false)
    }
  }

  return (
    <div className="space-y-3 mt-2">
      <input ref={cameraRef} type="file" accept="image/*" capture="environment" className="hidden" onChange={handleFile} />
      <input ref={libraryRef} type="file" accept="image/*" className="hidden" onChange={handleFile} />
      <div>
        <label className={labelCls}>Method (optional)</label>
        <select className={inputCls} value={method} onChange={e => setMethod(e.target.value)}>
          <option value="">Select method</option>
          {TRIMMING_METHODS.map(m => <option key={m} value={m} className="bg-raised">{m}</option>)}
        </select>
      </div>
      <div>
        <label className={labelCls}>Notes (optional)</label>
        <textarea className={inputCls} rows={2} placeholder="Any details…" value={notes} onChange={e => setNotes(e.target.value)} />
      </div>
      {preview || init?.photoKey ? (
        <button
          onClick={() => libraryRef.current?.click()}
          className="w-full h-36 rounded-xl border-2 border-transparent flex items-center justify-center overflow-hidden"
        >
          {preview
            ? <img src={preview} alt="preview" className="w-full h-full object-cover" />
            : <MediaImage photoKey={init!.photoKey!} alt="current" className="w-full h-full object-cover" />
          }
        </button>
      ) : (
        <div className="grid grid-cols-2 gap-2">
          <button
            onClick={() => cameraRef.current?.click()}
            className="h-36 rounded-xl border-2 border-dashed border-border text-muted flex flex-col items-center justify-center gap-2 active:opacity-70"
          >
            <Camera size={24} />
            <span className="text-sm">Take Photo</span>
          </button>
          <button
            onClick={() => libraryRef.current?.click()}
            className="h-36 rounded-xl border-2 border-dashed border-border text-muted flex flex-col items-center justify-center gap-2 active:opacity-70"
          >
            <ImagePlus size={24} />
            <span className="text-sm">Library</span>
          </button>
        </div>
      )}
      {preview && (
        <button onClick={() => { setFile(null); setPreview(null); if (cameraRef.current) cameraRef.current.value = ''; if (libraryRef.current) libraryRef.current.value = '' }} className="text-xs text-muted active:opacity-70 flex items-center gap-1">
          <X size={12} /> Remove photo
        </button>
      )}
      {error && <p className="text-red-400 text-sm">{error}</p>}
      <button onClick={handleSubmit} disabled={uploading} className="w-full py-3 bg-fern text-base font-semibold rounded-xl active:opacity-80 disabled:opacity-50">
        {uploading ? 'Uploading…' : 'Log Trim'}
      </button>
    </div>
  )
}

// ── Photo form ────────────────────────────────────────────────────────────────

function getInitPhotoKeys(init?: PhotoData): string[] {
  if (!init) return []
  if (Array.isArray((init as any).photoKeys)) return (init as any).photoKeys
  if ((init as any).photoKey) return [(init as any).photoKey]
  return []
}

function PhotoForm({ plantId, datetime, onSuccess, logId, init }: { plantId: string; datetime: string; onSuccess: () => void; logId?: string; init?: PhotoData }) {
  const qc = useQueryClient()
  const cameraRef = useRef<HTMLInputElement>(null)
  const libraryRef = useRef<HTMLInputElement>(null)
  const existingKeys = getInitPhotoKeys(init)
  // file+preview kept as one array (not two parallel ones) so adding more
  // photos later can never desync which preview belongs to which file.
  const [items, setItems]           = useState<{ file: File; preview: string }[]>([])
  const [caption, setCaption]       = useState(init?.caption ?? '')
  const [uploading, setUploading]   = useState(false)
  const [error, setError]           = useState<string | null>(null)

  function handleFiles(e: React.ChangeEvent<HTMLInputElement>) {
    const selected = Array.from(e.target.files ?? [])
    if (!selected.length) return
    setItems(prev => [...prev, ...selected.map(file => ({ file, preview: URL.createObjectURL(file) }))])
    e.target.value = ''
  }

  function removeNew(idx: number) {
    setItems(prev => prev.filter((_, i) => i !== idx))
  }

  const totalCount = existingKeys.length + items.length
  const canSave    = totalCount > 0

  async function handleSubmit() {
    if (!canSave) return
    setUploading(true)
    setError(null)
    try {
      const uploadedKeys = await Promise.all(
        items.map(({ file }) => api.media.uploadFile(file, `plants/${plantId}/logs`))
      )
      const photoKeys = [...existingKeys, ...uploadedKeys]
      const body = {
        logType: 'photo' as const,
        date: datetimeToDate(datetime),
        loggedAt: datetimeToISO(datetime),
        data: { photoKeys, ...(caption ? { caption } : {}) } as PhotoData,
      }
      if (logId) {
        await api.logs.update(plantId, logId, body)
      } else {
        await api.logs.create(plantId, body)
      }
      qc.invalidateQueries({ queryKey: ['logs', 'plant', plantId] })
      onSuccess()
    } catch (e) {
      setError((e as Error).message)
    } finally {
      setUploading(false)
    }
  }

  return (
    <div className="space-y-3 mt-2">
      <input ref={cameraRef} type="file" accept="image/*" capture="environment" className="hidden" onChange={handleFiles} />
      <input ref={libraryRef} type="file" accept="image/*" multiple className="hidden" onChange={handleFiles} />

      {totalCount === 0 ? (
        <div className="grid grid-cols-2 gap-2">
          <button
            onClick={() => cameraRef.current?.click()}
            className="h-48 rounded-xl border-2 border-dashed border-border text-muted flex flex-col items-center justify-center gap-2 active:opacity-70"
          >
            <Camera size={28} />
            <span className="text-sm">Take Photo</span>
          </button>
          <button
            onClick={() => libraryRef.current?.click()}
            className="h-48 rounded-xl border-2 border-dashed border-border text-muted flex flex-col items-center justify-center gap-2 active:opacity-70"
          >
            <ImagePlus size={28} />
            <span className="text-sm">Choose from Library</span>
          </button>
        </div>
      ) : (
        <div className="grid grid-cols-3 gap-2">
          {existingKeys.map(key => (
            <div key={key} className="aspect-square rounded-lg overflow-hidden">
              <MediaImage photoKey={key} alt="photo" className="w-full h-full object-cover" />
            </div>
          ))}
          {items.map(({ preview }, i) => (
            <div key={i} className="relative aspect-square rounded-lg overflow-hidden">
              <img src={preview} alt="preview" className="w-full h-full object-cover" />
              <button
                onClick={() => removeNew(i)}
                className="absolute top-1 right-1 w-5 h-5 rounded-full bg-black/60 flex items-center justify-center"
              >
                <X size={11} className="text-white" />
              </button>
            </div>
          ))}
          <button
            onClick={() => cameraRef.current?.click()}
            className="aspect-square rounded-lg border-2 border-dashed border-border text-muted flex items-center justify-center active:opacity-70"
          >
            <Camera size={20} />
          </button>
          <button
            onClick={() => libraryRef.current?.click()}
            className="aspect-square rounded-lg border-2 border-dashed border-border text-muted flex items-center justify-center active:opacity-70"
          >
            <ImagePlus size={20} />
          </button>
        </div>
      )}

      <div>
        <label className={labelCls}>Caption (optional)</label>
        <input
          className={inputCls}
          placeholder="Add a caption…"
          value={caption}
          onChange={e => setCaption(e.target.value)}
        />
      </div>

      {error && <p className="text-red-400 text-sm">{error}</p>}

      <button
        onClick={handleSubmit}
        disabled={!canSave || uploading}
        className="w-full py-3 bg-fern text-base font-semibold rounded-xl active:opacity-80 disabled:opacity-50"
      >
        {uploading
          ? `Uploading ${items.length > 1 ? `${items.length} photos` : 'photo'}…`
          : logId ? 'Save Changes' : `Save ${totalCount > 1 ? `${totalCount} Photos` : 'Photo'}`
        }
      </button>
    </div>
  )
}

// ── Type picker ───────────────────────────────────────────────────────────────

function TypePicker({ onSelect }: { onSelect: (type: LogType) => void }) {
  return (
    <div className="grid grid-cols-3 gap-3 mt-3 pb-2">
      {LOG_TYPES.map(({ type, label, icon, ready }) => (
        <button
          key={type}
          onClick={() => ready && onSelect(type)}
          disabled={!ready}
          className={`relative flex flex-col items-center gap-2 py-4 rounded-2xl border transition-colors ${
            ready
              ? 'border-border bg-raised active:scale-95 active:opacity-80 text-primary'
              : 'border-border/50 bg-raised/50 text-muted/40 cursor-default'
          }`}
        >
          {icon}
          <span className="text-xs font-medium">{label}</span>
          {!ready && (
            <span className="absolute top-1.5 right-1.5 text-[9px] font-medium text-muted/60 bg-raised rounded px-1">Soon</span>
          )}
        </button>
      ))}
    </div>
  )
}

// ── Main sheet ────────────────────────────────────────────────────────────────

interface Props {
  open: boolean
  onClose: () => void
  plant: Plant
  defaultDate?: string
  editLog?: Log
  defaultLogType?: LogType
}

export function AddLogSheet({ open, onClose, plant, defaultDate, editLog, defaultLogType }: Props) {
  const [selected, setSelected] = useState<LogType | null>(null)
  const [datetime, setDatetime] = useState(nowDatetime)

  useEffect(() => {
    if (open) {
      if (editLog) {
        setSelected(editLog.logType)
        setDatetime(isoToDatetime(editLog.loggedAt || (editLog.date + 'T12:00')))
      } else {
        const now = nowDatetime()
        setDatetime(defaultDate ? `${defaultDate}T${now.slice(11)}` : now)
        setSelected(defaultLogType ?? null)
      }
    }
  }, [open, defaultDate, editLog, defaultLogType])

  function reset() { setSelected(null) }
  function handleClose() { reset(); onClose() }

  const title = editLog
    ? `Edit ${LABELS[editLog.logType] ?? editLog.logType}`
    : selected ? LABELS[selected] : 'Log Activity'

  return (
    <BottomSheet
      title={title}
      open={open}
      onClose={handleClose}
      onBack={selected && !editLog ? reset : undefined}
    >
      <div className="flex items-center justify-between mt-2 pb-3 border-b border-border">
        <span className="text-xs text-dim">When</span>
        <input
          type="datetime-local"
          value={datetime}
          max={nowDatetime()}
          onChange={e => setDatetime(e.target.value)}
          className="bg-raised border border-border rounded-lg px-3 py-1.5 text-sm text-primary focus:outline-none focus:border-fern"
        />
      </div>

      {!selected ? (
        <TypePicker onSelect={setSelected} />
      ) : selected === 'watering' ? (
        <WateringForm plant={plant} datetime={datetime} onSuccess={handleClose} logId={editLog?.logId} init={editLog?.data as WateringData | undefined} />
      ) : selected === 'note' ? (
        <NoteForm plantId={plant.plantId} datetime={datetime} onSuccess={handleClose} logId={editLog?.logId} init={editLog?.data as NoteData | undefined} />
      ) : selected === 'height' ? (
        <HeightForm plantId={plant.plantId} datetime={datetime} onSuccess={handleClose} logId={editLog?.logId} init={editLog?.data as HeightData | undefined} />
      ) : selected === 'transplant' ? (
        <TransplantForm plantId={plant.plantId} datetime={datetime} onSuccess={handleClose} logId={editLog?.logId} init={editLog?.data as TransplantData | undefined} />
      ) : selected === 'training' ? (
        <TrainingForm plantId={plant.plantId} datetime={datetime} onSuccess={handleClose} logId={editLog?.logId} init={editLog?.data as TrainingData | undefined} />
      ) : selected === 'trimming' ? (
        <TrimmingForm plantId={plant.plantId} datetime={datetime} onSuccess={handleClose} logId={editLog?.logId} init={editLog?.data as TrimmingData | undefined} />
      ) : selected === 'photo' ? (
        <PhotoForm plantId={plant.plantId} datetime={datetime} onSuccess={handleClose} logId={editLog?.logId} init={editLog?.data as PhotoData | undefined} />
      ) : null}
    </BottomSheet>
  )
}
