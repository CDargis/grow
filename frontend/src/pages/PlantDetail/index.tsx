import { useMemo, useRef, useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useParams, useNavigate } from 'react-router-dom'
import { ArrowLeft, Droplets, Zap, Scissors, Ruler, MessageSquare, Camera, Wind, ChevronRight, ChevronDown, ChevronUp, Plus, Trash2, CalendarDays, List, Sun, Pencil, Gauge } from 'lucide-react'
import { api } from '@/api/client'
import { BottomSheet } from '@/components/BottomSheet'
import { DatePicker } from '@/components/DatePicker'
import { AddLogSheet } from '@/components/AddLogSheet'
import { EditPlantSheet } from '@/components/EditPlantSheet'
import { MediaImage } from '@/components/MediaImage'
import { PhotoLightbox, type PhotoLightboxItem } from '@/components/PhotoLightbox'
import { isFeedLog, logTypeLabel } from '@/lib/logDisplay'
import type { Log, LogType, Plant, PlantPhase, EnvironmentChangeData, LightingChangeData, Environment } from '@/types'

const EDITABLE_LOG_TYPES = new Set<LogType>(['watering', 'training', 'trimming', 'note', 'height', 'transplant', 'photo'])

function getPhotoKeys(data: any): string[] {
  if (Array.isArray(data?.photoKeys)) return data.photoKeys
  if (data?.photoKey) return [data.photoKey]
  return []
}

const PHASES: PlantPhase[] = ['germination', 'seedling', 'veg', 'flower', 'harvest', 'drying', 'curing', 'archived', 'dead']

const PHASE_COLORS: Record<PlantPhase, string> = {
  germination: 'bg-yellow-400/20 text-yellow-300 border-yellow-400/30',
  seedling:    'bg-lime/20 text-lime border-lime/30',
  veg:         'bg-fern/20 text-fern border-fern/30',
  flower:      'bg-purple-400/20 text-purple-300 border-purple-400/30',
  harvest:     'bg-orange-400/20 text-orange-300 border-orange-400/30',
  drying:      'bg-amber-500/20 text-amber-400 border-amber-500/30',
  curing:      'bg-amber-400/20 text-amber-300 border-amber-400/30',
  archived:    'bg-muted/10 text-muted border-muted/20',
  dead:        'bg-red-500/20 text-red-400 border-red-500/30',
}

const LOG_ICONS: Record<LogType, React.ReactNode> = {
  watering:           <Droplets size={16} />,
  feeding:            <Zap size={16} />,
  training:           <Scissors size={16} />,
  trimming:           <Scissors size={16} />,
  sprout:             <span className="text-xs">🌱</span>,
  transplant:         <span className="text-xs">🪴</span>,
  height:             <Ruler size={16} />,
  note:               <MessageSquare size={16} />,
  photo:              <Camera size={16} />,
  phase_change:       <span className="text-xs">🔄</span>,
  environment_change: <Wind size={16} />,
  lighting_change:    <Sun size={16} />,
  vpd_change:         <Gauge size={16} />,
}

function envName(id: string | undefined, envMap: Map<string, string>): string {
  if (!id) return 'None'
  return envMap.get(id) ?? id
}

function logSummary(log: Log, envMap: Map<string, string>): string | null {
  switch (log.logType) {
    case 'environment_change': {
      const d = log.data as unknown as EnvironmentChangeData
      const from = envName(d.fromEnvironmentId, envMap)
      const to   = envName(d.toEnvironmentId, envMap)
      if (d.toEnvironmentId) return from !== 'None' ? `${from} → ${to}` : `Assigned to ${to}`
      return `Removed from ${from}`
    }
    case 'phase_change': {
      const d = log.data as any
      return d.fromPhase ? `${d.fromPhase} → ${d.toPhase}` : `→ ${d.toPhase}`
    }
    case 'lighting_change': {
      const d = log.data as unknown as LightingChangeData
      if (d.fromSchedule && d.toSchedule) return `${d.fromSchedule} → ${d.toSchedule}`
      if (d.toSchedule) return `Set to ${d.toSchedule}`
      return null
    }
    case 'vpd_change': {
      const d = log.data as any
      if (d.fromVpd != null && d.toVpd != null) return `${d.fromVpd} → ${d.toVpd} kPa`
      if (d.toVpd   != null) return `Set to ${d.toVpd} kPa`
      if (d.fromVpd != null) return `Target cleared (was ${d.fromVpd} kPa)`
      return null
    }
    case 'training': {
      const d = log.data as any
      return d.method ? `${d.method}${d.notes ? ` · ${d.notes}` : ''}` : (d.notes ?? null)
    }
    case 'trimming': {
      const d = log.data as any
      const parts = [d.method, d.notes].filter(Boolean)
      return parts.length ? parts.join(' · ') : null
    }
    case 'sprout':   return 'Sprouted'
    case 'note':     return (log.data as any).text
    case 'height':   { const d = log.data as any; return `${d.height} ${d.unit}` }
    case 'watering': {
      const d = log.data as any
      const parts: string[] = []
      if (d.amount != null) parts.push(`${d.amount} ${d.unit}`)
      const nuts = (d.nutrients as Array<{ name: string; amount: number; unit: string }> ?? [])
        .filter(n => n.name)
        .map(n => `${n.name} ${n.amount}${n.unit}`)
        .join(' · ')
      if (nuts)                  parts.push(nuts)
      if (d.ph        != null)   parts.push(`pH ${d.ph}`)
      if (d.tds       != null)   parts.push(`TDS ${d.tds}`)
      if (d.runoff    != null)   parts.push(`runoff ${d.runoff}`)
      if (d.runoffTds != null)   parts.push(`runoff TDS ${d.runoffTds}`)
      if (d.note)                parts.push(d.note)
      return parts.length ? parts.join(' · ') : null
    }
    case 'feeding': {
      const d = log.data as any
      const nutrients = (d.nutrients as Array<{ name: string; amount: number; unit: string }> ?? [])
        .filter(n => n.name)
        .map(n => `${n.name} ${n.amount}${n.unit}`)
        .join(' · ')
      const ph    = d.ph       != null ? ` · pH ${d.ph}`       : ''
      const total = d.totalVol != null ? ` · ${d.totalVol}ml`  : ''
      return nutrients ? `${nutrients}${ph}${total}` : null
    }
    case 'transplant': { const d = log.data as any; return d.medium ? `${d.potSize} · ${d.medium}` : d.potSize }
    case 'photo':      return (log.data as any).caption ?? null
    default:         return null
  }
}

function elapsed(dateStr: string): string {
  const days = Math.floor((Date.now() - new Date(dateStr).getTime()) / 86400000)
  if (days === 0) return 'today'
  if (days < 7) return `${days} ${days === 1 ? 'day' : 'days'}`
  const weeks = Math.floor(days / 7)
  const rem   = days % 7
  const wLabel = `${weeks} ${weeks === 1 ? 'week' : 'weeks'}`
  const base   = rem === 0 ? wLabel : `${wLabel} and ${rem} ${rem === 1 ? 'day' : 'days'}`
  return `${base} (${days} days)`
}

function todayDate() {
  return new Date().toLocaleDateString('en-CA')
}

function formatDateHeader(date: string): string {
  const today     = todayDate()
  const yesterday = new Date(Date.now() - 86400000).toLocaleDateString('en-CA')
  if (date === today)     return 'Today'
  if (date === yesterday) return 'Yesterday'
  return new Date(date + 'T12:00:00').toLocaleDateString([], { weekday: 'short', month: 'short', day: 'numeric' })
}

// ── Phase hex colors (for timeline line/dot) ──────────────────────────────────

const PHASE_HEX: Record<PlantPhase, string> = {
  germination: '#facc15',
  seedling:    '#7CC96E',
  veg:         '#4A9E50',
  flower:      '#c084fc',
  harvest:     '#fb923c',
  drying:      '#f59e0b',
  curing:      '#fbbf24',
  archived:    '#666666',
  dead:        '#f87171',
}

// ── Phase period helpers ───────────────────────────────────────────────────────

interface PhasePeriod {
  phase: PlantPhase
  startDate: string
  endDate: string | null
  logs: Log[]
}

function buildPhasePeriods(plant: Plant, logs: Log[]): PhasePeriod[] {
  const phaseChangeLogs = logs
    .filter(l => l.logType === 'phase_change')
    .sort((a, b) => a.date.localeCompare(b.date) || a.loggedAt.localeCompare(b.loggedAt))

  const allDates = [...phaseChangeLogs.map(l => l.date), plant.phaseStartDate].filter(Boolean)
  const plantStart = allDates.length > 0
    ? allDates.reduce((min, d) => d < min ? d : min)
    : plant.phaseStartDate

  const periods: PhasePeriod[] = []

  if (phaseChangeLogs.length === 0) {
    periods.push({ phase: plant.phase, startDate: plantStart, endDate: null, logs: [] })
  } else {
    const first = phaseChangeLogs[0]
    const initialPhase = (first.data as any).fromPhase as PlantPhase | undefined
    if (initialPhase) {
      periods.push({ phase: initialPhase, startDate: plantStart, endDate: first.date, logs: [] })
    }
    for (let i = 0; i < phaseChangeLogs.length; i++) {
      const log  = phaseChangeLogs[i]
      const next = phaseChangeLogs[i + 1] ?? null
      periods.push({
        phase:     (log.data as any).toPhase as PlantPhase,
        startDate: log.date,
        endDate:   next?.date ?? null,
        logs:      [],
      })
    }
  }

  for (const log of logs.filter(l => l.logType !== 'phase_change')) {
    for (let i = periods.length - 1; i >= 0; i--) {
      const p = periods[i]
      if (log.date >= p.startDate && (p.endDate === null || log.date < p.endDate)) {
        p.logs.push(log)
        break
      }
    }
  }

  for (const p of periods) {
    p.logs.sort((a, b) => b.date.localeCompare(a.date) || b.loggedAt.localeCompare(a.loggedAt))
  }

  return periods
}

function phaseDuration(startDate: string, endDate: string | null): string {
  const end   = endDate ? new Date(endDate + 'T00:00:00') : new Date()
  const days  = Math.max(0, Math.floor((end.getTime() - new Date(startDate + 'T00:00:00').getTime()) / 86400000))
  if (days < 7)  return `${days}d`
  const weeks = Math.floor(days / 7)
  const rem   = days % 7
  return rem === 0 ? `${weeks}w` : `${weeks}w ${rem}d`
}

// ── Phase log entry (compact row inside an expanded phase) ────────────────────

function PhotoGrid({ photoKeys, size = 'sm', onPhotoTap }: { photoKeys: string[]; size?: 'sm' | 'md'; onPhotoTap: (photoKey: string) => void }) {
  const px = size === 'sm' ? 'w-14 h-14' : 'w-20 h-20'
  if (!photoKeys.length) return null
  return (
    <div className="flex flex-wrap gap-1.5 mt-1.5">
      {photoKeys.map(key => (
        <button key={key} onClick={() => onPhotoTap(key)} className={`${px} rounded-lg overflow-hidden flex-shrink-0 active:opacity-80`}>
          <MediaImage photoKey={key} alt="photo" className="w-full h-full object-cover" />
        </button>
      ))}
    </div>
  )
}

function PhaseLogEntry({ log, color, envMap, onDelete, onEdit, onOpenPhoto }: {
  log: Log
  color: string
  envMap: Map<string, string>
  onDelete: (logId: string) => void
  onEdit: (log: Log) => void
  onOpenPhoto: (log: Log, photoKey: string) => void
}) {
  const summary   = logSummary(log, envMap)
  const photoKeys = getPhotoKeys(log.data)

  return (
    <div className="flex items-start gap-2 py-2 border-b border-border/40 last:border-0">
      <div
        className="w-1.5 h-1.5 rounded-full flex-shrink-0 mt-[5px]"
        style={{ backgroundColor: color, opacity: 0.65 }}
      />
      <div className="w-5 h-5 rounded-full bg-raised border border-border flex items-center justify-center text-dim flex-shrink-0 mt-0.5">
        {isFeedLog(log) ? <Zap size={16} /> : LOG_ICONS[log.logType]}
      </div>
      <div className="flex-1 min-w-0">
        <div className="flex items-center justify-between gap-2">
          <span className="text-xs font-medium capitalize text-primary">
            {logTypeLabel(log)}
          </span>
          <div className="flex items-center gap-1.5 flex-shrink-0">
            <span className="text-[10px] text-muted">
              {new Date(log.date + 'T12:00:00').toLocaleDateString([], { month: 'short', day: 'numeric' })}
            </span>
            {EDITABLE_LOG_TYPES.has(log.logType) && (
              <button onClick={() => onEdit(log)} className="text-muted/40 active:text-primary p-0.5">
                <Pencil size={11} />
              </button>
            )}
            <button onClick={() => onDelete(log.logId)} className="text-muted/40 active:text-red-400 p-0.5">
              <Trash2 size={11} />
            </button>
          </div>
        </div>
        {summary && <p className="text-[11px] text-dim mt-0.5 break-words">{summary}</p>}
        <PhotoGrid photoKeys={photoKeys} size="sm" onPhotoTap={key => onOpenPhoto(log, key)} />
      </div>
    </div>
  )
}

// ── Phase period section ───────────────────────────────────────────────────────

function PhasePeriodSection({ period, isLast, envMap, onDelete, onEdit, onOpenPhoto }: {
  period: PhasePeriod
  isLast: boolean
  envMap: Map<string, string>
  onDelete: (logId: string) => void
  onEdit: (log: Log) => void
  onOpenPhoto: (log: Log, photoKey: string) => void
}) {
  const ongoing = period.endDate === null
  const [expanded, setExpanded] = useState(ongoing)
  const color = PHASE_HEX[period.phase] ?? '#666'

  const hasContent = period.logs.length > 0

  return (
    <div className="relative flex items-start gap-3">
      {/* Dot + line column */}
      <div className="flex flex-col items-center flex-shrink-0 w-3">
        <div
          className="w-3 h-3 rounded-full flex-shrink-0 mt-[5px] z-10 relative"
          style={{ backgroundColor: color }}
        />
        {!isLast && (
          <div
            className="w-0.5 flex-1 min-h-[20px] mt-1"
            style={{ backgroundColor: color, opacity: 0.25 }}
          />
        )}
      </div>

      {/* Phase content */}
      <div className="flex-1 min-w-0 pb-4">
        <button
          onClick={() => setExpanded(e => !e)}
          className="flex items-center justify-between w-full text-left"
        >
          <div className="flex items-center gap-2">
            <span className="font-semibold text-sm capitalize" style={{ color }}>
              {period.phase}
            </span>
            {ongoing && (
              <span className="text-[9px] text-muted border border-border rounded px-1 py-px uppercase tracking-wide">
                live
              </span>
            )}
          </div>
          <div className="flex items-center gap-1.5 text-muted flex-shrink-0">
            <span className="text-xs">{phaseDuration(period.startDate, period.endDate)}</span>
            {period.logs.length > 0 && (
              <span className="text-[10px] opacity-50">· {period.logs.length}</span>
            )}
            {expanded ? <ChevronUp size={12} /> : <ChevronDown size={12} />}
          </div>
        </button>

        {expanded && (
          <div className="mt-2 border-t border-border/30">
            {!hasContent ? (
              <p className="text-xs text-muted py-2">No activity logged this phase.</p>
            ) : (
              period.logs.map(log => (
                <PhaseLogEntry key={log.logId} log={log} color={color} envMap={envMap} onDelete={onDelete} onEdit={onEdit} onOpenPhoto={onOpenPhoto} />
              ))
            )}
          </div>
        )}
      </div>
    </div>
  )
}

// ── Timeline view ─────────────────────────────────────────────────────────────

function TimelineView({ plant, logs, envMap, onDelete, onEdit, onOpenPhoto }: {
  plant: Plant
  logs: Log[]
  envMap: Map<string, string>
  onDelete: (logId: string) => void
  onEdit: (log: Log) => void
  onOpenPhoto: (log: Log, photoKey: string) => void
}) {
  const periods = [...buildPhasePeriods(plant, logs)].reverse()

  return (
    <div className="pt-2">
      {periods.map((period, idx) => (
        <PhasePeriodSection
          key={period.phase + period.startDate}
          period={period}
          isLast={idx === periods.length - 1}
          envMap={envMap}
          onDelete={onDelete}
          onEdit={onEdit}
          onOpenPhoto={onOpenPhoto}
        />
      ))}
    </div>
  )
}

function LogEntry({ log, envMap, onDelete, onEdit, onOpenPhoto }: { log: Log; envMap: Map<string, string>; onDelete: (logId: string) => void; onEdit: (log: Log) => void; onOpenPhoto: (log: Log, photoKey: string) => void }) {
  const summary   = logSummary(log, envMap)
  const photoKeys = getPhotoKeys(log.data)
  return (
    <div className="flex items-start gap-3 py-3 border-b border-border last:border-0">
      <div className="w-7 h-7 rounded-full bg-raised border border-border flex items-center justify-center text-dim flex-shrink-0 mt-0.5">
        {isFeedLog(log) ? <Zap size={16} /> : LOG_ICONS[log.logType]}
      </div>
      <div className="flex-1 min-w-0">
        <div className="flex items-center justify-between gap-2">
          <span className="text-sm font-medium capitalize text-primary">
            {logTypeLabel(log)}
          </span>
          <div className="flex items-center gap-2 flex-shrink-0">
            <span className="text-xs text-muted">
              {new Date(log.loggedAt).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}
            </span>
            {EDITABLE_LOG_TYPES.has(log.logType) && (
              <button onClick={() => onEdit(log)} className="text-muted/50 active:text-primary p-0.5">
                <Pencil size={12} />
              </button>
            )}
            <button onClick={() => onDelete(log.logId)} className="text-muted/50 active:text-red-400 active:opacity-80 p-0.5">
              <Trash2 size={13} />
            </button>
          </div>
        </div>
        {summary && <p className="text-xs text-dim mt-0.5 break-words">{summary}</p>}
        <PhotoGrid photoKeys={photoKeys} size="md" onPhotoTap={key => onOpenPhoto(log, key)} />
      </div>
    </div>
  )
}


function PhaseChangeSheet({ plant, open, onClose }: { plant: Plant; open: boolean; onClose: () => void }) {
  const qc = useQueryClient()
  const [toPhase, setToPhase] = useState<PlantPhase | null>(null)

  const mutation = useMutation({
    mutationFn: () => api.plants.updatePhase(
      plant.plantId, toPhase!,
      new Date().toLocaleDateString('en-CA'),
      new Date().toISOString(),
    ),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['plant', plant.plantId] })
      qc.invalidateQueries({ queryKey: ['logs', 'plant', plant.plantId] })
      qc.invalidateQueries({ queryKey: ['plants'] })
      setToPhase(null)
      onClose()
    },
  })

  return (
    <BottomSheet title="Change Phase" open={open} onClose={onClose}>
      <div className="space-y-4 mt-2">
        <div className="grid grid-cols-3 gap-2">
          {PHASES.filter(p => p !== plant.phase).map(phase => (
            <button
              key={phase}
              onClick={() => setToPhase(phase)}
              className={`py-2 px-2 rounded-xl text-xs font-medium border capitalize transition-colors ${
                toPhase === phase
                  ? PHASE_COLORS[phase]
                  : 'border-border text-muted bg-raised active:opacity-70'
              }`}
            >
              {phase}
            </button>
          ))}
        </div>
        {mutation.isError && <p className="text-red-400 text-sm">{(mutation.error as Error).message}</p>}
        <button
          onClick={() => { if (toPhase) mutation.mutate() }}
          disabled={!toPhase || mutation.isPending}
          className="w-full py-3 bg-fern text-base font-semibold rounded-xl active:opacity-80 disabled:opacity-50"
        >
          {mutation.isPending ? 'Saving…' : 'Change Phase'}
        </button>
      </div>
    </BottomSheet>
  )
}

function ChangeEnvironmentSheet({
  open, onClose, currentEnvId, plantId,
}: {
  open: boolean
  onClose: () => void
  currentEnvId: string | undefined
  plantId: string
}) {
  const qc = useQueryClient()
  const { data: envs } = useQuery({
    queryKey: ['environments'],
    queryFn: api.environments.list,
    enabled: open,
  })

  const mutation = useMutation({
    mutationFn: (environmentId: string | null) =>
      api.plants.assignEnvironment(plantId, environmentId),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['plant', plantId] })
      qc.invalidateQueries({ queryKey: ['logs', 'plant', plantId] })
      onClose()
    },
  })

  return (
    <BottomSheet title="Change Environment" open={open} onClose={onClose}>
      <div className="space-y-2 mt-2">
        <button
          onClick={() => mutation.mutate(null)}
          disabled={mutation.isPending || !currentEnvId}
          className={`w-full flex items-center gap-3 p-3 rounded-xl border ${
            !currentEnvId
              ? 'border-fern bg-forest/20 text-primary'
              : 'border-border bg-raised text-dim active:opacity-70'
          }`}
        >
          <span className="text-base">—</span>
          <span className="text-sm">No environment</span>
        </button>

        {envs?.map((env: Environment) => (
          <button
            key={env.environmentId}
            onClick={() => mutation.mutate(env.environmentId)}
            disabled={mutation.isPending || env.environmentId === currentEnvId}
            className={`w-full flex items-center gap-3 p-3 rounded-xl border ${
              env.environmentId === currentEnvId
                ? 'border-fern bg-forest/20 text-primary'
                : 'border-border bg-raised text-primary active:opacity-70'
            }`}
          >
            <span className="text-base">⛺</span>
            <span className="text-sm flex-1 text-left">{env.name}</span>
            {env.environmentId === currentEnvId && (
              <span className="text-xs text-fern">current</span>
            )}
          </button>
        ))}

        {mutation.isError && (
          <p className="text-red-400 text-sm">{(mutation.error as Error).message}</p>
        )}
      </div>
    </BottomSheet>
  )
}

function AvatarPickerSheet({ open, onClose, logs, plantId }: {
  open: boolean
  onClose: () => void
  logs: Log[]
  plantId: string
}) {
  const qc = useQueryClient()
  const photoLogs = logs.filter(l => getPhotoKeys(l.data).length > 0)

  const mutation = useMutation({
    mutationFn: (photoKey: string) => api.plants.updateAvatar(plantId, photoKey),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['plant', plantId] })
      qc.invalidateQueries({ queryKey: ['plants'] })
      onClose()
    },
  })

  return (
    <BottomSheet title="Choose Cover Photo" open={open} onClose={onClose}>
      {photoLogs.length === 0 ? (
        <p className="py-12 text-center text-muted text-sm">
          No photos yet — add a photo log entry first.
        </p>
      ) : (
        <div className="grid grid-cols-3 gap-1.5 mt-2">
          {photoLogs.flatMap(log =>
            getPhotoKeys(log.data).map(photoKey => (
              <button
                key={`${log.logId}-${photoKey}`}
                onClick={() => mutation.mutate(photoKey)}
                disabled={mutation.isPending}
                className="aspect-square rounded-lg overflow-hidden active:opacity-70 disabled:opacity-50"
              >
                <MediaImage photoKey={photoKey} alt="log photo" className="w-full h-full object-cover" />
              </button>
            ))
          )}
        </div>
      )}
      {mutation.isError && (
        <p className="text-red-400 text-sm mt-2">{(mutation.error as Error).message}</p>
      )}
    </BottomSheet>
  )
}

type View = 'journal' | 'timeline'

export function PlantDetailPage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const qc = useQueryClient()
  const [envSheetOpen, setEnvSheetOpen] = useState(false)
  const [logSheetOpen, setLogSheetOpen] = useState(false)
  const [selectedDate, setSelectedDate] = useState<string | null>(null)
  const [avatarPickerOpen, setAvatarPickerOpen] = useState(false)
  const [view, setView]           = useState<View>('journal')
  const [editOpen, setEditOpen]         = useState(false)
  const [editLog, setEditLog]           = useState<Log | null>(null)
  const [phaseSheetOpen, setPhaseSheetOpen] = useState(false)
  const [weekRef, setWeekRef] = useState(new Date())
  const [quickLogType, setQuickLogType] = useState<LogType | null>(null)
  const [lightboxIndex, setLightboxIndex] = useState<number | null>(null)

  function pdWeekStart(d: Date): Date {
    const r = new Date(d); r.setHours(0, 0, 0, 0)
    const dow = r.getDay()
    r.setDate(r.getDate() - (dow === 0 ? 6 : dow - 1))
    return r
  }
  function pdAddDays(d: Date, n: number): Date {
    const r = new Date(d); r.setDate(r.getDate() + n); return r
  }
  function pdToYMD(d: Date): string { return d.toISOString().slice(0, 10) }

  const currentWeek = Array.from({ length: 7 }, (_, i) =>
    pdToYMD(pdAddDays(pdWeekStart(weekRef), i))
  )

  function handleWeekRefChange(newRef: Date) {
    setWeekRef(newRef)
    const newWeek = Array.from({ length: 7 }, (_, i) =>
      pdToYMD(pdAddDays(pdWeekStart(newRef), i))
    )
    const effective = selectedDate ?? todayDate()
    if (!newWeek.includes(effective)) {
      const dow = new Date(effective + 'T12:00:00').getDay()
      const mondayIdx = dow === 0 ? 6 : dow - 1
      const candidate = newWeek[mondayIdx]
      const today = todayDate()
      setSelectedDate(candidate <= today ? candidate : (newWeek.filter(d => d <= today).pop() ?? null))
    }
  }

  // Swiping the log area moves one day at a time; if the new day falls
  // outside the visible week, the date strip follows.
  function changeDay(delta: number) {
    const next = pdAddDays(new Date(journalDate + 'T12:00:00'), delta)
    const nextStr = pdToYMD(next)
    setSelectedDate(nextStr)
    if (!currentWeek.includes(nextStr)) {
      setWeekRef(next)
    }
  }

  const dayTouch = useRef<{ x: number; y: number } | null>(null)

  function handleDayTouchStart(e: React.TouchEvent) {
    dayTouch.current = { x: e.touches[0].clientX, y: e.touches[0].clientY }
  }

  function handleDayTouchEnd(e: React.TouchEvent) {
    if (!dayTouch.current) return
    const dx = e.changedTouches[0].clientX - dayTouch.current.x
    const dy = e.changedTouches[0].clientY - dayTouch.current.y
    dayTouch.current = null
    // Must be a clearly horizontal gesture so vertical scrolling never triggers it
    if (Math.abs(dx) < 50 || Math.abs(dx) < Math.abs(dy) * 1.5) return
    changeDay(dx < 0 ? 1 : -1)
  }

  // Slide direction derived from the date actually changing, so tapping the
  // strip animates the day content the same way a swipe does.
  const prevJournalDate = useRef<string | null>(null)

  function openQuickLog(type: LogType) {
    setQuickLogType(type)
    setEditLog(null)
    setLogSheetOpen(true)
  }

  const deleteLog = useMutation({
    mutationFn: (logId: string) => api.logs.delete(id!, logId),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['logs', 'plant', id] }),
  })

  function handleEditLog(log: Log) {
    setEditLog(log)
    setLogSheetOpen(true)
  }

  const { data: plant, isLoading: plantLoading } = useQuery({
    queryKey: ['plant', id],
    queryFn: () => api.plants.get(id!),
    enabled: !!id,
  })

  const { data: logs, isLoading: logsLoading } = useQuery({
    queryKey: ['logs', 'plant', id],
    queryFn: () => api.logs.listForPlant(id!),
    enabled: !!id,
  })

  const { data: envs } = useQuery({
    queryKey: ['environments'],
    queryFn: api.environments.list,
  })



  const envMap = new Map<string, string>(
    envs?.map((e: Environment) => [e.environmentId, e.name]) ?? []
  )

  // Every photo on this plant, oldest → newest, so the lightbox flows across
  // log entries as a time-lapse. Left = back in time, right = forward.
  const photoItems: Array<PhotoLightboxItem & { logId: string }> = useMemo(() => {
    if (!plant || !logs) return []
    const periods = buildPhasePeriods(plant, logs)
    return [...logs]
      .sort((a, b) => a.date.localeCompare(b.date) || a.loggedAt.localeCompare(b.loggedAt))
      .flatMap(log =>
        getPhotoKeys(log.data).map(photoKey => {
          const period = periods.find(p => log.date >= p.startDate && (p.endDate === null || log.date < p.endDate))
          const day = period
            ? Math.floor((new Date(log.date + 'T00:00:00').getTime() - new Date(period.startDate + 'T00:00:00').getTime()) / 86400000) + 1
            : null
          return {
            logId: log.logId,
            photoKey,
            title: new Date(log.date + 'T12:00:00').toLocaleDateString([], { month: 'short', day: 'numeric', year: 'numeric' }),
            subtitle: period ? `${period.phase} · day ${day}` : undefined,
            accent: period ? PHASE_HEX[period.phase] : undefined,
          }
        })
      )
  }, [plant, logs])

  function openPhoto(log: Log, photoKey: string) {
    const idx = photoItems.findIndex(p => p.logId === log.logId && p.photoKey === photoKey)
    if (idx >= 0) setLightboxIndex(idx)
  }

  const activeDates = new Set(logs?.map((l: Log) => l.date) ?? [])
  const journalDate = selectedDate ?? todayDate()

  const dayAnim = prevJournalDate.current && prevJournalDate.current !== journalDate
    ? (journalDate > prevJournalDate.current ? 'animate-slide-in-right' : 'animate-slide-in-left')
    : ''
  prevJournalDate.current = journalDate

  const plantStartDate = (() => {
    const base = plant?.phaseStartDate ?? ''
    if (!logs || !base) return base
    const dates = logs.filter(l => l.logType === 'phase_change').map(l => l.date)
    return [...dates, base].reduce((min, d) => d < min ? d : min)
  })()

  const sproutDate = (() => {
    if (!logs || !plant) return undefined
    const toSeedling = logs
      .filter(l => l.logType === 'phase_change' && (l.data as any).toPhase === 'seedling')
      .sort((a, b) => a.date.localeCompare(b.date))[0]
    if (toSeedling) return toSeedling.date
    if (plant.phase === 'seedling') return plant.phaseStartDate
    return undefined
  })()
  const journalLogs = logs?.filter((l: Log) => l.date === journalDate) ?? []

  function toggleView() {
    setView(v => v === 'journal' ? 'timeline' : 'journal')
  }

  if (plantLoading) {
    return <div className="flex items-center justify-center h-full text-muted">Loading…</div>
  }
  if (!plant) {
    return <div className="flex items-center justify-center h-full text-muted">Plant not found.</div>
  }

  const currentEnvName = plant.environmentId ? (envMap.get(plant.environmentId) ?? plant.environmentId) : null

  const ViewToggle = ({ dark }: { dark?: boolean }) => (
    <button
      onClick={toggleView}
      className={`w-9 h-9 rounded-full flex items-center justify-center active:opacity-70 ${
        dark
          ? 'bg-black/40 backdrop-blur-sm text-white'
          : 'text-muted active:text-primary'
      }`}
    >
      {view === 'journal' ? <List size={18} /> : <CalendarDays size={18} />}
    </button>
  )

  return (
    <div className="flex flex-col">
      {plant.avatarKey ? (
        /* Hero banner when photo is set */
        <div className="relative h-60 w-full flex-shrink-0">
          <MediaImage photoKey={plant.avatarKey} alt={plant.name} className="w-full h-full object-cover" />
          <div className="absolute inset-0 bg-gradient-to-t from-black/75 via-black/20 to-transparent" />

          <div className="absolute top-0 left-0 right-0 flex items-center justify-between p-4 pt-5">
            <button
              onClick={() => navigate(-1)}
              className="w-9 h-9 rounded-full bg-black/40 backdrop-blur-sm flex items-center justify-center text-white active:opacity-70"
            >
              <ArrowLeft size={20} />
            </button>
            <div className="flex items-center gap-2">
              <ViewToggle dark />
              <button
                onClick={() => setEditOpen(true)}
                className="w-9 h-9 rounded-full bg-black/40 backdrop-blur-sm flex items-center justify-center text-white active:opacity-70"
              >
                <Pencil size={16} />
              </button>
            </div>
          </div>

          <button
            onClick={() => setAvatarPickerOpen(true)}
            className="absolute bottom-0 left-0 right-0 p-4 text-left"
          >
            <div className="flex items-end justify-between gap-2">
              <div className="min-w-0">
                <h1 className="font-bold text-xl text-white leading-tight truncate">{plant.name}</h1>
                <p className="text-sm text-white/70 truncate">{plant.strain}</p>
                <div className="flex flex-col gap-px mt-0.5">
                  <span className="text-xs" style={{ color: PHASE_HEX[plant.phase] }}>{elapsed(plant.phaseStartDate)} in {plant.phase}</span>
                  {sproutDate && <span className="text-xs text-amber-300">{elapsed(sproutDate)} from sprout</span>}
                  <span className="text-xs text-white/40">{elapsed(plantStartDate)} old</span>
                </div>
              </div>
              <button
                onClick={e => { e.stopPropagation(); setPhaseSheetOpen(true) }}
                className={`px-2.5 py-1 rounded-full text-xs font-semibold border capitalize flex-shrink-0 mb-0.5 ${PHASE_COLORS[plant.phase]}`}
              >
                {plant.phase}
              </button>
            </div>
          </button>
        </div>
      ) : (
        /* Compact header when no photo */
        <div className="flex items-center gap-2 p-4 border-b border-border flex-shrink-0">
          <button onClick={() => navigate(-1)} className="text-dim active:opacity-70 mr-1">
            <ArrowLeft size={20} />
          </button>
          <button
            onClick={() => setAvatarPickerOpen(true)}
            className="w-11 h-11 rounded-full bg-raised border border-border flex items-center justify-center text-2xl flex-shrink-0 overflow-hidden active:opacity-70"
          >
            <span>🌱</span>
          </button>
          <div className="flex-1 min-w-0">
            <h1 className="font-semibold text-primary">{plant.name}</h1>
            <p className="text-xs text-dim truncate">{plant.strain}</p>
            <div className="flex flex-col gap-px">
              <span className="text-xs" style={{ color: PHASE_HEX[plant.phase] }}>{elapsed(plant.phaseStartDate)} in {plant.phase}</span>
              {sproutDate && <span className="text-xs text-amber-400">{elapsed(sproutDate)} from sprout</span>}
              <span className="text-xs text-muted">{elapsed(plantStartDate)} old</span>
            </div>
          </div>
          <button
            onClick={() => setPhaseSheetOpen(true)}
            className={`px-2.5 py-1 rounded-full text-xs font-semibold border capitalize flex-shrink-0 ${PHASE_COLORS[plant.phase]}`}
          >
            {plant.phase}
          </button>
          <ViewToggle />
          <button onClick={() => setEditOpen(true)} className="text-muted active:opacity-70">
            <Pencil size={16} />
          </button>
        </div>
      )}

      {/* Environment row */}
      <button
        onClick={() => setEnvSheetOpen(true)}
        className="flex items-center gap-3 px-4 py-3 border-b border-border active:bg-raised transition-colors"
      >
        <Wind size={16} className="text-dim flex-shrink-0" />
        <span className="text-sm text-primary flex-1 text-left">
          {currentEnvName ?? <span className="text-muted">No environment</span>}
        </span>
        <ChevronRight size={16} className="text-muted" />
      </button>

      {/* Journal view: date strip + instant actions + single-day logs */}
      {view === 'journal' && (
        <>
          <DatePicker
            activeDates={activeDates}
            selected={selectedDate}
            onSelect={setSelectedDate}
            weekRef={weekRef}
            onWeekRefChange={handleWeekRefChange}
          />
          {/* Instant action tray */}
          {(() => {
            const colIndex = currentWeek.indexOf(journalDate)
            return (
              <div className="relative bg-raised border-b border-border">
                {colIndex >= 0 && (
                  <div
                    className="absolute w-3 h-3 rotate-45 bg-raised border-t border-l border-border z-10"
                    style={{ top: '-6px', left: `calc(30px + ${colIndex + 0.5} * (100% - 102px) / 7)` }}
                  />
                )}
                <div className="flex justify-around px-2 py-2">
                  {([
                    { type: 'watering' as LogType, icon: <Droplets size={20} />, label: 'Water' },
                    { type: 'photo'    as LogType, icon: <Camera size={20} />,   label: 'Photo' },
                    { type: 'note'     as LogType, icon: <MessageSquare size={20} />, label: 'Note' },
                    { type: 'height'   as LogType, icon: <Ruler size={20} />,    label: 'Height' },
                  ] as const).map(({ type, icon, label }) => (
                    <button
                      key={type}
                      onClick={() => openQuickLog(type)}
                      className="flex flex-col items-center gap-1 px-3 py-1.5 rounded-xl active:bg-surface active:opacity-70"
                    >
                      <span className="text-dim">{icon}</span>
                      <span className="text-[10px] text-muted leading-none">{label}</span>
                    </button>
                  ))}
                  <button
                    onClick={() => { setEditLog(null); setQuickLogType(null); setLogSheetOpen(true) }}
                    className="flex flex-col items-center gap-1 px-3 py-1.5 rounded-xl active:bg-surface active:opacity-70"
                  >
                    <span className="text-dim"><Plus size={20} /></span>
                    <span className="text-[10px] text-muted leading-none">More</span>
                  </button>
                </div>
              </div>
            )
          })()}
          <div
            className="p-4 min-h-[40vh] overflow-x-hidden"
            onTouchStart={handleDayTouchStart}
            onTouchEnd={handleDayTouchEnd}
          >
            <div key={journalDate} className={dayAnim}>
              <p className="text-xs text-muted mb-2 uppercase tracking-wide font-medium">
                {formatDateHeader(journalDate)}
              </p>
              {logsLoading ? (
                <div className="text-muted text-sm">Loading…</div>
              ) : journalLogs.length === 0 ? (
                <div className="text-muted text-sm py-8 text-center">No activity on this day.</div>
              ) : (
                journalLogs.map((log: Log) => (
                  <LogEntry key={log.logId} log={log} envMap={envMap} onDelete={deleteLog.mutate} onEdit={handleEditLog} onOpenPhoto={openPhoto} />
                ))
              )}
            </div>
          </div>
        </>
      )}

      {/* Timeline view: all logs grouped by date */}
      {view === 'timeline' && (
        <div className="p-4">
          {logsLoading ? (
            <div className="text-muted text-sm">Loading…</div>
          ) : (
            <TimelineView
              plant={plant}
              logs={logs ?? []}
              envMap={envMap}
              onDelete={deleteLog.mutate}
              onEdit={handleEditLog}
              onOpenPhoto={openPhoto}
            />
          )}
        </div>
      )}

      <PhotoLightbox
        items={photoItems}
        index={lightboxIndex}
        onClose={() => setLightboxIndex(null)}
        onNavigate={setLightboxIndex}
      />
      <AvatarPickerSheet
        open={avatarPickerOpen}
        onClose={() => setAvatarPickerOpen(false)}
        logs={logs ?? []}
        plantId={plant.plantId}
      />
      <EditPlantSheet
        open={editOpen}
        onClose={() => setEditOpen(false)}
        plant={plant}
      />
      <AddLogSheet
        open={logSheetOpen}
        onClose={() => { setLogSheetOpen(false); setEditLog(null); setQuickLogType(null) }}
        plant={plant}
        defaultDate={selectedDate ?? undefined}
        editLog={editLog ?? undefined}
        defaultLogType={quickLogType ?? undefined}
      />
      <ChangeEnvironmentSheet
        open={envSheetOpen}
        onClose={() => setEnvSheetOpen(false)}
        currentEnvId={plant.environmentId}
        plantId={plant.plantId}
      />
      <PhaseChangeSheet
        plant={plant}
        open={phaseSheetOpen}
        onClose={() => setPhaseSheetOpen(false)}
      />
    </div>
  )
}
