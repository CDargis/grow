import { useState, useRef } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useParams, useNavigate } from 'react-router-dom'
import { ArrowLeft, Droplets, Zap, Scissors, Ruler, MessageSquare, Camera, Wind, ChevronRight, ChevronDown, ChevronUp, Plus, Trash2, X, CalendarDays, List, Sun, Pencil, CheckCircle2, Bot, RefreshCw, Gauge } from 'lucide-react'
import { api } from '@/api/client'
import { BottomSheet } from '@/components/BottomSheet'
import { DatePicker } from '@/components/DatePicker'
import { AddLogSheet } from '@/components/AddLogSheet'
import { EditPlantSheet } from '@/components/EditPlantSheet'
import { MediaImage } from '@/components/MediaImage'
import type { Log, LogType, Plant, PlantPhase, EnvironmentChangeData, LightingChangeData, Environment, Milestone, MilestoneType, PlantObservation } from '@/types'

const EDITABLE_LOG_TYPES = new Set<LogType>(['watering', 'feeding', 'note', 'height', 'transplant', 'photo'])

const LOG_ICONS: Record<LogType, React.ReactNode> = {
  watering:           <Droplets size={16} />,
  feeding:            <Zap size={16} />,
  training:           <Scissors size={16} />,
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
    case 'sprout':   return 'Sprouted'
    case 'note':     return (log.data as any).text
    case 'height':   { const d = log.data as any; return `${d.height} ${d.unit}` }
    case 'watering': {
      const d = log.data as any
      const vol    = d.amount != null ? `${d.amount} ${d.unit}` : d.unit
      const ph     = d.ph     != null ? ` · pH ${d.ph}`         : ''
      const runoff = d.runoff != null ? ` · runoff ${d.runoff}` : ''
      const note   = d.note   ? ` · ${d.note}`                  : ''
      return `${vol}${ph}${runoff}${note}`
    }
    case 'height':     { const d = log.data as any; return `${d.height} ${d.unit}` }
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

// ── Milestone helpers ─────────────────────────────────────────────────────────

const MILESTONE_LABELS: Record<MilestoneType, string> = {
  cotyledons_off: 'Cotyledons Fall Off',
  leaf_set_1:     '1st True Leaves',
  leaf_set_2:     '2nd Leaf Set',
  leaf_set_3:     '3rd Leaf Set',
  leaf_set_4:     '4th Leaf Set',
  early_veg:      'Early Veg',
  full_veg:       'Full Veg',
  pre_flower:     'Pre-Flower / Sex Signs',
  flip_to_flower: 'Flip to Flower',
  peak_flower:    'Full Flower',
  harvest:        'Harvest Window',
  dry_complete:   'Dry Complete',
  cure_ready:     'Cure Ready',
}

const PHASE_ORDER: PlantPhase[] = ['germination', 'seedling', 'veg', 'flower', 'harvest', 'drying', 'curing']

// Which phases each milestone is relevant to
const MILESTONE_PHASES: Record<MilestoneType, PlantPhase[]> = {
  cotyledons_off: ['seedling'],
  leaf_set_1:     ['seedling'],
  leaf_set_2:     ['seedling'],
  leaf_set_3:     ['seedling'],
  leaf_set_4:     ['seedling'],
  early_veg:      ['seedling', 'veg'],
  full_veg:       ['veg'],
  pre_flower:     ['veg', 'flower'],
  flip_to_flower: ['veg'],
  peak_flower:    ['flower'],
  harvest:        ['flower', 'harvest'],
  dry_complete:   ['drying'],
  cure_ready:     ['curing'],
}

const CONFIDENCE_COLORS: Record<string, string> = {
  high:   'text-fern border-fern/40',
  medium: 'text-amber-400 border-amber-400/40',
  low:    'text-muted border-border',
}

function formatMilestoneDate(dateStr: string): string {
  const today = todayDate()
  const d = new Date(dateStr + 'T12:00:00')
  const diff = Math.round((d.getTime() - new Date(today + 'T12:00:00').getTime()) / 86400000)
  if (diff === 0) return 'Today'
  if (diff > 0) return `in ${diff}d · ${d.toLocaleDateString([], { month: 'short', day: 'numeric' })}`
  return `${Math.abs(diff)}d ago · ${d.toLocaleDateString([], { month: 'short', day: 'numeric' })}`
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
    p.logs.sort((a, b) => a.date.localeCompare(b.date) || a.loggedAt.localeCompare(b.loggedAt))
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

function PhaseLogEntry({ log, color, envMap, onDelete, onEdit }: {
  log: Log
  color: string
  envMap: Map<string, string>
  onDelete: (logId: string) => void
  onEdit: (log: Log) => void
}) {
  const [lightbox, setLightbox] = useState(false)
  const summary  = logSummary(log, envMap)
  const photoKey = log.logType === 'photo' ? (log.data as any).photoKey as string | undefined : undefined

  return (
    <>
      <div className="flex items-start gap-2 py-2 border-b border-border/40 last:border-0">
        <div
          className="w-1.5 h-1.5 rounded-full flex-shrink-0 mt-[5px]"
          style={{ backgroundColor: color, opacity: 0.65 }}
        />
        <div className="w-5 h-5 rounded-full bg-raised border border-border flex items-center justify-center text-dim flex-shrink-0 mt-0.5">
          {LOG_ICONS[log.logType]}
        </div>
        <div className="flex-1 min-w-0">
          <div className="flex items-center justify-between gap-2">
            <span className="text-xs font-medium capitalize text-primary">
              {log.logType.replace(/_/g, ' ')}
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
          {photoKey && (
            <button onClick={() => setLightbox(true)} className="mt-1.5 w-16 h-16 rounded-lg overflow-hidden block active:opacity-80">
              <MediaImage photoKey={photoKey} alt="photo" className="w-full h-full object-cover" />
            </button>
          )}
        </div>
      </div>

      {lightbox && photoKey && (
        <div className="fixed inset-0 z-50 bg-black flex items-center justify-center" onClick={() => setLightbox(false)}>
          <button
            className="absolute top-4 right-4 w-9 h-9 rounded-full bg-white/10 flex items-center justify-center text-white"
            onClick={() => setLightbox(false)}
          >
            <X size={20} />
          </button>
          <MediaImage photoKey={photoKey} alt="photo" className="max-w-full max-h-full object-contain" />
        </div>
      )}
    </>
  )
}

// ── Milestone entry ───────────────────────────────────────────────────────────

function MilestoneEntry({ milestone, onConfirm, onSkip }: {
  milestone: Milestone
  onConfirm: (type: MilestoneType, date: string) => void
  onSkip: (type: MilestoneType) => void
}) {
  const today      = todayDate()
  const isPastDue  = (milestone.predictedDate ?? '') <= today
  const canAct     = milestone.status === 'predicted' && isPastDue
  const [picking, setPicking] = useState(false)
  const [pickDate, setPickDate] = useState(today)

  if (milestone.status === 'skipped') return null

  const label       = MILESTONE_LABELS[milestone.milestoneType] ?? milestone.milestoneType.replace(/_/g, ' ')
  const colorCls    = CONFIDENCE_COLORS[milestone.confidence] ?? CONFIDENCE_COLORS.low
  const isConfirmed = milestone.status === 'confirmed'

  function handleConfirm() {
    onConfirm(milestone.milestoneType, pickDate)
    setPicking(false)
  }

  return (
    <div className={`py-2 border-b border-border/40 last:border-0 ${isConfirmed ? 'opacity-70' : ''}`}>
      <div className="flex items-start gap-2">
        <div className="flex-shrink-0 mt-0.5">
          {isConfirmed
            ? <CheckCircle2 size={14} className="text-fern" />
            : <Bot size={14} className={isPastDue ? 'text-amber-400' : 'text-violet-400/70'} />
          }
        </div>
        <div className="flex-1 min-w-0">
          <div className="flex items-center justify-between gap-2">
            <span className="text-xs font-medium text-primary">{label}</span>
            {!isConfirmed && (
              <span className={`text-[10px] border rounded px-1 py-px flex-shrink-0 ${colorCls}`}>
                {milestone.confidence}
              </span>
            )}
          </div>
          <p className="text-[11px] text-dim mt-0.5">
            {isConfirmed
              ? `Confirmed ${new Date(milestone.confirmedDate! + 'T12:00:00').toLocaleDateString([], { month: 'short', day: 'numeric' })}`
              : formatMilestoneDate(milestone.predictedDate ?? '')
            }
            {milestone.notes && !isConfirmed && <span className="ml-1 text-muted">· {milestone.notes}</span>}
          </p>
          {milestone.lastChangedAt && milestone.lastChangedFrom && !isConfirmed && (
            <p className="text-[10px] text-muted mt-0.5">
              ↻ changed from {new Date(milestone.lastChangedFrom + 'T12:00:00').toLocaleDateString([], { month: 'short', day: 'numeric' })}
              {' on '}
              {new Date(milestone.lastChangedAt).toLocaleDateString([], { month: 'short', day: 'numeric' })}
              {milestone.lastChangeReason && <span className="italic"> · {milestone.lastChangeReason}</span>}
            </p>
          )}
          {canAct && !picking && (
            <div className="flex gap-2 mt-1.5">
              <button
                onClick={() => { setPickDate(today); setPicking(true) }}
                className="text-[10px] px-2 py-0.5 rounded bg-fern/20 text-fern border border-fern/30 active:opacity-70"
              >
                Confirm
              </button>
              <button
                onClick={() => onSkip(milestone.milestoneType)}
                className="text-[10px] px-2 py-0.5 rounded bg-raised text-muted border border-border active:opacity-70"
              >
                Skip
              </button>
            </div>
          )}
          {picking && (
            <div className="flex items-center gap-2 mt-1.5">
              <input
                type="date"
                value={pickDate}
                max={today}
                onChange={e => setPickDate(e.target.value)}
                className="text-[11px] bg-surface border border-border rounded px-2 py-0.5 text-primary"
              />
              <button
                onClick={handleConfirm}
                className="text-[10px] px-2 py-0.5 rounded bg-fern/20 text-fern border border-fern/30 active:opacity-70"
              >
                Done
              </button>
              <button
                onClick={() => setPicking(false)}
                className="text-[10px] text-muted active:opacity-70"
              >
                Cancel
              </button>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}

// ── Phase period section ───────────────────────────────────────────────────────

function PhasePeriodSection({ period, isLast, envMap, milestones, onDelete, onEdit, onConfirmMilestone, onSkipMilestone }: {
  period: PhasePeriod
  isLast: boolean
  envMap: Map<string, string>
  milestones: Milestone[]
  onDelete: (logId: string) => void
  onEdit: (log: Log) => void
  onConfirmMilestone: (type: MilestoneType, date: string) => void
  onSkipMilestone: (type: MilestoneType) => void
}) {
  const ongoing = period.endDate === null
  const [expanded, setExpanded] = useState(ongoing)
  const color = PHASE_HEX[period.phase] ?? '#666'

  // Active phase: show predicted + confirmed milestones belonging to this phase
  // Completed phases: show only confirmed milestones belonging to this phase
  const phaseMilestones = milestones
    .filter(m => {
      if (m.status === 'skipped') return false
      if (!MILESTONE_PHASES[m.milestoneType]?.includes(period.phase)) return false
      if (!ongoing && m.status === 'predicted') return false
      return true
    })
    .sort((a, b) => (a.predictedDate ?? '').localeCompare(b.predictedDate ?? ''))

  const hasContent = period.logs.length > 0 || phaseMilestones.length > 0

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
            {phaseMilestones.length > 0 && !expanded && (
              <span className="text-[9px] text-amber-400 border border-amber-400/30 rounded px-1 py-px">
                {phaseMilestones.length} milestone{phaseMilestones.length > 1 ? 's' : ''}
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
            {phaseMilestones.length > 0 && (
              <>
                <div className="flex items-center gap-1.5 pt-2 pb-0.5">
                  <Bot size={11} className="text-violet-400" />
                  <span className="text-[10px] font-medium text-violet-400 uppercase tracking-wide">AI Predictions</span>
                </div>
                {phaseMilestones.map(m => (
                  <MilestoneEntry
                    key={m.milestoneType}
                    milestone={m}
                    onConfirm={onConfirmMilestone}
                    onSkip={onSkipMilestone}
                  />
                ))}
              </>
            )}
            {!hasContent ? (
              <p className="text-xs text-muted py-2">No activity logged this phase.</p>
            ) : (
              period.logs.map(log => (
                <PhaseLogEntry key={log.logId} log={log} color={color} envMap={envMap} onDelete={onDelete} onEdit={onEdit} />
              ))
            )}
          </div>
        )}
      </div>
    </div>
  )
}

// ── Upcoming phase section (future phases with AI-predicted milestones) ───────

function UpcomingPhaseSection({ phase, milestones, isLast, onConfirmMilestone, onSkipMilestone }: {
  phase: PlantPhase
  milestones: Milestone[]
  isLast: boolean
  onConfirmMilestone: (type: MilestoneType, date: string) => void
  onSkipMilestone: (type: MilestoneType) => void
}) {
  const [expanded, setExpanded] = useState(false)
  const color = PHASE_HEX[phase] ?? '#666'

  return (
    <div className="relative flex items-start gap-3">
      <div className="flex flex-col items-center flex-shrink-0 w-3">
        <div
          className="w-3 h-3 rounded-full flex-shrink-0 mt-[5px] z-10 relative opacity-35"
          style={{ backgroundColor: color }}
        />
        {!isLast && (
          <div
            className="w-0.5 flex-1 min-h-[20px] mt-1"
            style={{ backgroundColor: color, opacity: 0.12 }}
          />
        )}
      </div>
      <div className="flex-1 min-w-0 pb-4">
        <button
          onClick={() => setExpanded(e => !e)}
          className="flex items-center justify-between w-full text-left"
        >
          <div className="flex items-center gap-2">
            <span className="font-semibold text-sm capitalize opacity-40" style={{ color }}>{phase}</span>
            <span className="text-[9px] text-muted border border-border rounded px-1 py-px uppercase tracking-wide">upcoming</span>
            {!expanded && (
              <span className="text-[9px] text-violet-400 border border-violet-400/30 rounded px-1 py-px">
                {milestones.length} predicted
              </span>
            )}
          </div>
          {expanded ? <ChevronUp size={12} className="text-muted" /> : <ChevronDown size={12} className="text-muted" />}
        </button>
        {expanded && (
          <div className="mt-2 border-t border-border/30">
            <div className="flex items-center gap-1.5 pt-2 pb-0.5">
              <Bot size={11} className="text-violet-400" />
              <span className="text-[10px] font-medium text-violet-400 uppercase tracking-wide">AI Predictions</span>
            </div>
            {milestones.map(m => (
              <MilestoneEntry
                key={m.milestoneType}
                milestone={m}
                onConfirm={onConfirmMilestone}
                onSkip={onSkipMilestone}
              />
            ))}
          </div>
        )}
      </div>
    </div>
  )
}

// ── Timeline view ─────────────────────────────────────────────────────────────

function TimelineView({ plant, logs, envMap, milestones, onDelete, onEdit, onConfirmMilestone, onSkipMilestone }: {
  plant: Plant
  logs: Log[]
  envMap: Map<string, string>
  milestones: Milestone[]
  onDelete: (logId: string) => void
  onEdit: (log: Log) => void
  onConfirmMilestone: (type: MilestoneType, date: string) => void
  onSkipMilestone: (type: MilestoneType) => void
}) {
  const periods = buildPhasePeriods(plant, logs)
  const currentPhaseIdx = PHASE_ORDER.indexOf(plant.phase)

  // Build upcoming sections: each predicted milestone assigned to its earliest future phase
  const upcomingSections = PHASE_ORDER
    .slice(currentPhaseIdx + 1)
    .map(phase => ({
      phase,
      milestones: milestones
        .filter(m => {
          if (m.status !== 'predicted') return false
          // Already shown in the active phase section
          if (MILESTONE_PHASES[m.milestoneType]?.includes(plant.phase)) return false
          // Assign to the first future phase this milestone belongs to
          const firstFuturePhase = PHASE_ORDER
            .slice(currentPhaseIdx + 1)
            .find(p => MILESTONE_PHASES[m.milestoneType]?.includes(p))
          return firstFuturePhase === phase
        })
        .sort((a, b) => (a.predictedDate ?? '').localeCompare(b.predictedDate ?? '')),
    }))
    .filter(s => s.milestones.length > 0)

  type Section =
    | { kind: 'period'; period: PhasePeriod }
    | { kind: 'upcoming'; phase: PlantPhase; milestones: Milestone[] }

  // Reverse-chronological: furthest future at top, active below upcoming, oldest past at bottom
  const sections: Section[] = [
    ...[...upcomingSections].reverse().map(s => ({ kind: 'upcoming' as const, phase: s.phase, milestones: s.milestones })),
    ...[...periods].reverse().map(p => ({ kind: 'period' as const, period: p })),
  ]

  return (
    <div className="pt-2">
      {sections.map((section, idx) => {
        const isLast = idx === sections.length - 1
        if (section.kind === 'period') {
          return (
            <PhasePeriodSection
              key={section.period.phase + section.period.startDate}
              period={section.period}
              isLast={isLast}
              envMap={envMap}
              milestones={milestones}
              onDelete={onDelete}
              onEdit={onEdit}
              onConfirmMilestone={onConfirmMilestone}
              onSkipMilestone={onSkipMilestone}
            />
          )
        }
        return (
          <UpcomingPhaseSection
            key={section.phase}
            phase={section.phase}
            milestones={section.milestones}
            isLast={isLast}
            onConfirmMilestone={onConfirmMilestone}
            onSkipMilestone={onSkipMilestone}
          />
        )
      })}
    </div>
  )
}

function LogEntry({ log, envMap, onDelete, onEdit }: { log: Log; envMap: Map<string, string>; onDelete: (logId: string) => void; onEdit: (log: Log) => void }) {
  const [lightbox, setLightbox] = useState(false)
  const summary = logSummary(log, envMap)
  const photoKey = log.logType === 'photo' ? (log.data as any).photoKey as string | undefined : undefined
  return (
    <>
      <div className="flex items-start gap-3 py-3 border-b border-border last:border-0">
        <div className="w-7 h-7 rounded-full bg-raised border border-border flex items-center justify-center text-dim flex-shrink-0 mt-0.5">
          {LOG_ICONS[log.logType]}
        </div>
        <div className="flex-1 min-w-0">
          <div className="flex items-center justify-between gap-2">
            <span className="text-sm font-medium capitalize text-primary">
              {log.logType.replace(/_/g, ' ')}
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
          {photoKey && (
            <button onClick={() => setLightbox(true)} className="mt-2 w-28 h-28 rounded-lg overflow-hidden block active:opacity-80">
              <MediaImage photoKey={photoKey} alt="photo log" className="w-full h-full object-cover" />
            </button>
          )}
        </div>
      </div>

      {lightbox && photoKey && (
        <div
          className="fixed inset-0 z-50 bg-black flex items-center justify-center"
          onClick={() => setLightbox(false)}
        >
          <button
            className="absolute top-4 right-4 w-9 h-9 rounded-full bg-white/10 flex items-center justify-center text-white"
            onClick={() => setLightbox(false)}
          >
            <X size={20} />
          </button>
          <MediaImage photoKey={photoKey} alt="photo log" className="max-w-full max-h-full object-contain" />
        </div>
      )}
    </>
  )
}

// ── Observation components ────────────────────────────────────────────────────

function ObservationsSection({ observations, plantId, lastCalibratedAt }: {
  observations: PlantObservation[]
  plantId: string
  lastCalibratedAt?: string
}) {
  const qc = useQueryClient()
  // Auto-expand when lastCalibratedAt changes (new sync). Stays collapsed once user collapses it.
  const seenCalibrationRef = useRef<string | undefined>(undefined)
  const [expanded, setExpanded] = useState(false)

  if (lastCalibratedAt && lastCalibratedAt !== seenCalibrationRef.current) {
    seenCalibrationRef.current = lastCalibratedAt
    if (!expanded) setExpanded(true)
  }

  const calibrate = useMutation({
    mutationFn: () => api.plants.calibrate(plantId),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['plant', plantId] }),
  })

  if (observations.length === 0 && !lastCalibratedAt) return null

  return (
    <div className="border-b border-border">
      <div className="flex items-center gap-2 px-4 py-2.5">
        <button
          onClick={() => setExpanded(e => !e)}
          className="flex items-center gap-2 flex-1 text-left min-w-0"
        >
          <Bot size={13} className="text-violet-400 flex-shrink-0" />
          <span className="text-xs font-medium text-primary">AI Notes</span>
          {lastCalibratedAt && (
            <span className="text-[10px] text-muted/50 truncate">
              · {new Date(lastCalibratedAt).toLocaleString([], { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' })}
            </span>
          )}
        </button>
        <button
          onClick={() => calibrate.mutate()}
          disabled={calibrate.isPending}
          className="text-muted/50 active:text-primary p-0.5 flex-shrink-0 disabled:opacity-40"
          title="Run calibration"
        >
          <RefreshCw size={12} className={calibrate.isPending ? 'animate-spin' : ''} />
        </button>
        {observations.length > 0 && (
          <button onClick={() => setExpanded(e => !e)} className="text-muted flex-shrink-0">
            {expanded ? <ChevronUp size={12} /> : <ChevronDown size={12} />}
          </button>
        )}
      </div>
      {expanded && observations.length > 0 && (
        <div className="px-4 pb-3 space-y-2">
          {observations.map(obs => (
            <div key={obs.observationId} className="flex items-start gap-2.5 rounded-xl p-2.5 bg-raised">
              <Bot size={13} className="text-violet-400/60 flex-shrink-0 mt-0.5" />
              <div className="flex-1 min-w-0">
                <p className="text-[11px] text-primary leading-snug">{obs.text}</p>
                <p className="text-[10px] text-muted/60 mt-0.5">
                  {new Date(obs.observedAt).toLocaleDateString([], { month: 'short', day: 'numeric' })}
                  {' · '}<span className="capitalize">{obs.category}</span>
                </p>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
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

type View = 'journal' | 'timeline'

export function PlantDetailPage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const qc = useQueryClient()
  const avatarRef = useRef<HTMLInputElement>(null)
  const [envSheetOpen, setEnvSheetOpen] = useState(false)
  const [logSheetOpen, setLogSheetOpen] = useState(false)
  const [selectedDate, setSelectedDate] = useState<string | null>(null)
  const [avatarUploading, setAvatarUploading] = useState(false)
  const [view, setView]           = useState<View>('journal')
  const [editOpen, setEditOpen]   = useState(false)
  const [editLog, setEditLog]     = useState<Log | null>(null)

  const deleteLog = useMutation({
    mutationFn: (logId: string) => api.logs.delete(id!, logId),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['logs', 'plant', id] }),
  })

  const confirmMilestone = useMutation({
    mutationFn: ({ type, date }: { type: MilestoneType; date: string }) =>
      api.milestones.confirm(id!, type, date),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['milestones', id] }),
  })

  const skipMilestone = useMutation({
    mutationFn: (type: MilestoneType) => api.milestones.skip(id!, type),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['milestones', id] }),
  })

  function handleEditLog(log: Log) {
    setEditLog(log)
    setLogSheetOpen(true)
  }

  async function handleAvatarChange(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0]
    if (!file || !id) return
    setAvatarUploading(true)
    try {
      const key = await api.media.uploadFile(file, `plants/${id}/avatar`)
      await api.plants.updateAvatar(id, key)
      qc.invalidateQueries({ queryKey: ['plant', id] })
      qc.invalidateQueries({ queryKey: ['plants'] })
    } finally {
      setAvatarUploading(false)
      if (avatarRef.current) avatarRef.current.value = ''
    }
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

  const { data: milestones = [] } = useQuery({
    queryKey: ['milestones', id],
    queryFn: () => api.milestones.listForPlant(id!),
    enabled: !!id,
  })

  const { data: observations = [] } = useQuery({
    queryKey: ['observations', id],
    queryFn: () => api.observations.listForPlant(id!),
    enabled: !!id,
  })


  const envMap = new Map<string, string>(
    envs?.map((e: Environment) => [e.environmentId, e.name]) ?? []
  )

  const activeDates = new Set(logs?.map((l: Log) => l.date) ?? [])
  const journalDate = selectedDate ?? todayDate()

  const plantStartDate = (() => {
    const base = plant?.phaseStartDate ?? ''
    if (!logs || !base) return base
    const dates = logs.filter(l => l.logType === 'phase_change').map(l => l.date)
    return [...dates, base].reduce((min, d) => d < min ? d : min)
  })()

  const sproutDate = logs?.find(l => l.logType === 'sprout')?.date
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
      <input ref={avatarRef} type="file" accept="image/*" className="hidden" onChange={handleAvatarChange} />

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
              <button
                onClick={() => setLogSheetOpen(true)}
                className="w-9 h-9 rounded-full bg-black/40 backdrop-blur-sm flex items-center justify-center text-white active:opacity-70"
              >
                <Plus size={20} />
              </button>
            </div>
          </div>

          <button
            onClick={() => avatarRef.current?.click()}
            disabled={avatarUploading}
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
              <span className="text-xs text-fern capitalize font-semibold flex-shrink-0 mb-0.5">{plant.phase}</span>
            </div>
            {avatarUploading && (
              <div className="absolute inset-0 bg-black/40 flex items-center justify-center">
                <div className="w-6 h-6 border-2 border-white border-t-transparent rounded-full animate-spin" />
              </div>
            )}
          </button>
        </div>
      ) : (
        /* Compact header when no photo */
        <div className="flex items-center gap-2 p-4 border-b border-border flex-shrink-0">
          <button onClick={() => navigate(-1)} className="text-dim active:opacity-70 mr-1">
            <ArrowLeft size={20} />
          </button>
          <button
            onClick={() => avatarRef.current?.click()}
            disabled={avatarUploading}
            className="relative w-11 h-11 rounded-full bg-raised border border-border flex items-center justify-center text-2xl flex-shrink-0 overflow-hidden active:opacity-70"
          >
            <span>🌱</span>
            {avatarUploading && (
              <div className="absolute inset-0 bg-black/50 flex items-center justify-center">
                <div className="w-4 h-4 border-2 border-white border-t-transparent rounded-full animate-spin" />
              </div>
            )}
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
          <span className="text-xs text-fern capitalize font-medium flex-shrink-0">{plant.phase}</span>
          <ViewToggle />
          <button onClick={() => setEditOpen(true)} className="text-muted active:opacity-70">
            <Pencil size={16} />
          </button>
          <button onClick={() => setLogSheetOpen(true)} className="text-fern active:opacity-70">
            <Plus size={18} />
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

      <ObservationsSection observations={observations} plantId={plant.plantId} lastCalibratedAt={plant.lastCalibratedAt} />

      {/* Journal view: date strip + single-day logs */}
      {view === 'journal' && (
        <>
          <DatePicker activeDates={activeDates} selected={selectedDate} onSelect={setSelectedDate} />
          <div className="p-4">
            <p className="text-xs text-muted mb-2 uppercase tracking-wide font-medium">
              {formatDateHeader(journalDate)}
            </p>
            {logsLoading ? (
              <div className="text-muted text-sm">Loading…</div>
            ) : journalLogs.length === 0 ? (
              <div className="text-muted text-sm py-8 text-center">No activity on this day.</div>
            ) : (
              journalLogs.map((log: Log) => (
                <LogEntry key={log.logId} log={log} envMap={envMap} onDelete={deleteLog.mutate} onEdit={handleEditLog} />
              ))
            )}
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
              milestones={milestones}
              onDelete={deleteLog.mutate}
              onEdit={handleEditLog}
              onConfirmMilestone={(type, date) => confirmMilestone.mutate({ type, date })}
              onSkipMilestone={type => skipMilestone.mutate(type)}
            />
          )}
        </div>
      )}

      <EditPlantSheet
        open={editOpen}
        onClose={() => setEditOpen(false)}
        plant={plant}
      />
      <AddLogSheet
        open={logSheetOpen}
        onClose={() => { setLogSheetOpen(false); setEditLog(null) }}
        plant={plant}
        defaultDate={selectedDate ?? undefined}
        editLog={editLog ?? undefined}
      />
      <ChangeEnvironmentSheet
        open={envSheetOpen}
        onClose={() => setEnvSheetOpen(false)}
        currentEnvId={plant.environmentId}
        plantId={plant.plantId}
      />
    </div>
  )
}
