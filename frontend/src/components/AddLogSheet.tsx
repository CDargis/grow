import { useState, useRef, useEffect } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { Droplets, Zap, Ruler, MessageSquare, Camera, GitBranch, Scissors, ImagePlus, X } from 'lucide-react'
import { BottomSheet } from './BottomSheet'
import { MediaImage } from './MediaImage'
import { api } from '@/api/client'
import type { Plant, Log, LogType, WateringData, FeedingData, TrainingData, TrimmingData, NoteData, PhotoData, TransplantData, HeightData } from '@/types'

// ── Types config ─────────────────────────────────────────────────────────────

type TypeConfig = {
  type: LogType
  label: string
  icon: React.ReactNode
  ready: boolean
}

const LOG_TYPES: TypeConfig[] = [
  { type: 'watering',   label: 'Water',    icon: <Droplets  size={22} />, ready: true  },
  { type: 'feeding',    label: 'Feed',     icon: <Zap       size={22} />, ready: true  },
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

// ── Watering form ─────────────────────────────────────────────────────────────

function WateringForm({ plantId, datetime, onSuccess, logId, init }: { plantId: string; datetime: string; onSuccess: () => void; logId?: string; init?: WateringData }) {
  const qc = useQueryClient()
  const [amount, setAmount]  = useState(init?.amount?.toString() ?? '')
  const [unit, setUnit]      = useState<WateringData['unit']>(init?.unit ?? 'ml')
  const [ph, setPh]          = useState(init?.ph?.toString() ?? '')
  const [runoff, setRunoff]  = useState(init?.runoff?.toString() ?? '')
  const [note, setNote]      = useState(init?.note ?? '')

  const body = {
    logType: 'watering' as const,
    date: datetimeToDate(datetime),
    loggedAt: datetimeToISO(datetime),
    data: {
      ...(amount ? { amount: Number(amount) } : {}),
      unit,
      ...(ph     ? { ph: Number(ph) }         : {}),
      ...(runoff ? { runoff: Number(runoff) }  : {}),
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
        onClick={() => mutation.mutate()}
        disabled={mutation.isPending}
        className="w-full py-3 bg-fern text-base font-semibold rounded-xl active:opacity-80 disabled:opacity-50"
      >
        {mutation.isPending ? 'Saving…' : 'Log Watering'}
      </button>
    </div>
  )
}

// ── Feeding form ──────────────────────────────────────────────────────────────

type NutrientRow = { name: string; amount: string; unit: string }

function FeedingForm({ plantId, datetime, onSuccess, logId, init }: { plantId: string; datetime: string; onSuccess: () => void; logId?: string; init?: FeedingData }) {
  const qc = useQueryClient()
  const [nutrients, setNutrients] = useState<NutrientRow[]>(
    init?.nutrients?.map(n => ({ name: n.name, amount: String(n.amount), unit: n.unit })) ?? [{ name: '', amount: '', unit: 'ml' }]
  )
  const [ph, setPh]         = useState(init?.ph?.toString() ?? '')
  const [totalVol, setVol]  = useState(init?.totalVol?.toString() ?? '')

  function updateNutrient(i: number, field: keyof NutrientRow, value: string) {
    setNutrients(rows => rows.map((r, idx) => idx === i ? { ...r, [field]: value } : r))
  }

  const valid = nutrients.some(n => n.name && n.amount)

  const body = {
    logType: 'feeding' as const,
    date: datetimeToDate(datetime),
    loggedAt: datetimeToISO(datetime),
    data: {
      nutrients: nutrients
        .filter(n => n.name && n.amount)
        .map(n => ({ name: n.name, amount: Number(n.amount), unit: n.unit })),
      ...(ph       ? { ph: Number(ph) }             : {}),
      ...(totalVol ? { totalVol: Number(totalVol) } : {}),
    } as FeedingData,
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
        <label className={labelCls}>Nutrients</label>
        <div className="space-y-2">
          {nutrients.map((n, i) => (
            <div key={i} className="flex gap-2 items-center">
              <input
                className={inputCls + ' flex-1'}
                placeholder="Name"
                value={n.name}
                onChange={e => updateNutrient(i, 'name', e.target.value)}
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
              {nutrients.length > 1 && (
                <button onClick={() => setNutrients(rows => rows.filter((_, idx) => idx !== i))} className="text-muted active:opacity-60 flex-shrink-0">✕</button>
              )}
            </div>
          ))}
        </div>
        <button
          type="button"
          onClick={() => setNutrients(rows => [...rows, { name: '', amount: '', unit: 'ml' }])}
          className="mt-2 text-xs text-fern active:opacity-70"
        >
          + Add nutrient
        </button>
      </div>

      <div className="grid grid-cols-2 gap-3">
        <div>
          <label className={labelCls}>pH</label>
          <input type="number" step="0.1" className={inputCls} placeholder="6.2" value={ph} onChange={e => setPh(e.target.value)} />
        </div>
        <div>
          <label className={labelCls}>Total Vol (ml)</label>
          <input type="number" className={inputCls} placeholder="1000" value={totalVol} onChange={e => setVol(e.target.value)} />
        </div>
      </div>

      {mutation.isError && <p className="text-red-400 text-sm">{(mutation.error as Error).message}</p>}

      <button
        onClick={() => { if (valid) mutation.mutate() }}
        disabled={!valid || mutation.isPending}
        className="w-full py-3 bg-fern text-base font-semibold rounded-xl active:opacity-80 disabled:opacity-50"
      >
        {mutation.isPending ? 'Saving…' : 'Log Feeding'}
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
  const fileRef = useRef<HTMLInputElement>(null)
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
      <input ref={fileRef} type="file" accept="image/*" className="hidden" onChange={handleFile} />
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
      <button
        onClick={() => fileRef.current?.click()}
        className={`w-full h-36 rounded-xl border-2 border-dashed flex flex-col items-center justify-center gap-2 transition-colors overflow-hidden ${preview || init?.photoKey ? 'border-transparent p-0' : 'border-border text-muted active:opacity-70'}`}
      >
        {preview
          ? <img src={preview} alt="preview" className="w-full h-full object-cover" />
          : init?.photoKey
            ? <MediaImage photoKey={init.photoKey} alt="current" className="w-full h-full object-cover" />
            : <><ImagePlus size={24} /><span className="text-sm">Add photo (optional)</span></>
        }
      </button>
      {preview && (
        <button onClick={() => { setFile(null); setPreview(null); if (fileRef.current) fileRef.current.value = '' }} className="text-xs text-muted active:opacity-70 flex items-center gap-1">
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
  const fileRef = useRef<HTMLInputElement>(null)
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
      <input ref={fileRef} type="file" accept="image/*" className="hidden" onChange={handleFile} />
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
      <button
        onClick={() => fileRef.current?.click()}
        className={`w-full h-36 rounded-xl border-2 border-dashed flex flex-col items-center justify-center gap-2 transition-colors overflow-hidden ${preview || init?.photoKey ? 'border-transparent p-0' : 'border-border text-muted active:opacity-70'}`}
      >
        {preview
          ? <img src={preview} alt="preview" className="w-full h-full object-cover" />
          : init?.photoKey
            ? <MediaImage photoKey={init.photoKey} alt="current" className="w-full h-full object-cover" />
            : <><ImagePlus size={24} /><span className="text-sm">Add photo (optional)</span></>
        }
      </button>
      {preview && (
        <button onClick={() => { setFile(null); setPreview(null); if (fileRef.current) fileRef.current.value = '' }} className="text-xs text-muted active:opacity-70 flex items-center gap-1">
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

function PhotoForm({ plantId, datetime, onSuccess, logId, init }: { plantId: string; datetime: string; onSuccess: () => void; logId?: string; init?: PhotoData }) {
  const qc = useQueryClient()
  const fileRef = useRef<HTMLInputElement>(null)
  const [file, setFile] = useState<File | null>(null)
  const [preview, setPreview] = useState<string | null>(null)
  const [caption, setCaption] = useState(init?.caption ?? '')
  const [uploading, setUploading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  function handleFile(e: React.ChangeEvent<HTMLInputElement>) {
    const f = e.target.files?.[0]
    if (!f) return
    setFile(f)
    setPreview(URL.createObjectURL(f))
  }

  function removePhoto() {
    setFile(null)
    setPreview(null)
    if (fileRef.current) fileRef.current.value = ''
  }

  async function handleSubmit() {
    if (!file && !init?.photoKey) return
    setUploading(true)
    setError(null)
    try {
      const photoKey = file
        ? await api.media.uploadFile(file, `plants/${plantId}/logs`)
        : init!.photoKey
      const body = {
        logType: 'photo' as const,
        date: datetimeToDate(datetime),
        loggedAt: datetimeToISO(datetime),
        data: { photoKey, ...(caption ? { caption } : {}) } as PhotoData,
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
      <input ref={fileRef} type="file" accept="image/*" className="hidden" onChange={handleFile} />

      <button
        onClick={() => fileRef.current?.click()}
        className={`w-full h-52 rounded-xl border-2 border-dashed flex flex-col items-center justify-center gap-2 transition-colors overflow-hidden ${
          preview || init?.photoKey ? 'border-transparent p-0' : 'border-border text-muted active:opacity-70'
        }`}
      >
        {preview
          ? <img src={preview} alt="preview" className="w-full h-full object-cover" />
          : init?.photoKey
            ? <MediaImage photoKey={init.photoKey} alt="current photo" className="w-full h-full object-cover" />
            : <>
                <ImagePlus size={28} />
                <span className="text-sm">Tap to select photo</span>
              </>
        }
      </button>

      {preview && (
        <button onClick={removePhoto} className="text-xs text-muted active:opacity-70 flex items-center gap-1">
          <X size={12} /> Remove photo
        </button>
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
        disabled={(!file && !init?.photoKey) || uploading}
        className="w-full py-3 bg-fern text-base font-semibold rounded-xl active:opacity-80 disabled:opacity-50"
      >
        {uploading ? 'Uploading…' : logId ? 'Save Changes' : 'Save Photo'}
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
}

export function AddLogSheet({ open, onClose, plant, defaultDate, editLog }: Props) {
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
        setSelected(null)
      }
    }
  }, [open, defaultDate, editLog])

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
        <WateringForm plantId={plant.plantId} datetime={datetime} onSuccess={handleClose} logId={editLog?.logId} init={editLog?.data as WateringData | undefined} />
      ) : selected === 'feeding' ? (
        <FeedingForm plantId={plant.plantId} datetime={datetime} onSuccess={handleClose} logId={editLog?.logId} init={editLog?.data as FeedingData | undefined} />
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
