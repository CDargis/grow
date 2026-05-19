import { useState, useRef } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useParams, useNavigate } from 'react-router-dom'
import { ArrowLeft, Droplets, Zap, Scissors, Ruler, MessageSquare, Camera, Wind, ChevronRight, Plus, Trash2, X, CalendarDays, List } from 'lucide-react'
import { api } from '@/api/client'
import { BottomSheet } from '@/components/BottomSheet'
import { DatePicker } from '@/components/DatePicker'
import { AddLogSheet } from '@/components/AddLogSheet'
import { MediaImage } from '@/components/MediaImage'
import type { Log, LogType, EnvironmentChangeData, Environment } from '@/types'

const LOG_ICONS: Record<LogType, React.ReactNode> = {
  watering:           <Droplets size={16} />,
  feeding:            <Zap size={16} />,
  training:           <Scissors size={16} />,
  trimming:           <Scissors size={16} />,
  transplant:         <span className="text-xs">🪴</span>,
  height:             <Ruler size={16} />,
  note:               <MessageSquare size={16} />,
  photo:              <Camera size={16} />,
  phase_change:       <span className="text-xs">🔄</span>,
  environment_change: <Wind size={16} />,
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
    case 'note':     return (log.data as any).text
    case 'height':   { const d = log.data as any; return `${d.height} ${d.unit}` }
    case 'watering': { const d = log.data as any; return d.amount != null ? `${d.amount} ${d.unit}` : d.unit }
    case 'photo':    return (log.data as any).caption ?? null
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
  if (rem === 0) return wLabel
  return `${wLabel} and ${rem} ${rem === 1 ? 'day' : 'days'}`
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

function TimelineView({ logs, envMap, onDelete }: { logs: Log[]; envMap: Map<string, string>; onDelete: (logId: string) => void }) {
  const sorted = [...logs].sort((a, b) =>
    b.date.localeCompare(a.date) || b.loggedAt.localeCompare(a.loggedAt)
  )
  const grouped: Record<string, Log[]> = {}
  for (const log of sorted) {
    if (!grouped[log.date]) grouped[log.date] = []
    grouped[log.date].push(log)
  }
  const dates = Object.keys(grouped).sort((a, b) => b.localeCompare(a))

  if (dates.length === 0) {
    return <div className="text-muted text-sm py-8 text-center">No logs yet.</div>
  }
  return (
    <>
      {dates.map(date => (
        <div key={date}>
          <p className="text-xs text-muted uppercase tracking-wide font-medium pt-4 pb-1 sticky top-0 bg-base">
            {formatDateHeader(date)}
          </p>
          {grouped[date].map(log => (
            <LogEntry key={log.logId} log={log} envMap={envMap} onDelete={onDelete} />
          ))}
        </div>
      ))}
    </>
  )
}

function LogEntry({ log, envMap, onDelete }: { log: Log; envMap: Map<string, string>; onDelete: (logId: string) => void }) {
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
              <button
                onClick={() => onDelete(log.logId)}
                className="text-muted/50 active:text-red-400 active:opacity-80 p-0.5"
              >
                <Trash2 size={13} />
              </button>
            </div>
          </div>
          {summary && <p className="text-xs text-dim mt-0.5 truncate">{summary}</p>}
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
  const [view, setView] = useState<View>('journal')

  const deleteLog = useMutation({
    mutationFn: (logId: string) => api.logs.delete(id!, logId),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['logs', 'plant', id] }),
  })

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

  const envMap = new Map<string, string>(
    envs?.map((e: Environment) => [e.environmentId, e.name]) ?? []
  )

  const activeDates = new Set(logs?.map((l: Log) => l.date) ?? [])
  const journalDate = selectedDate ?? todayDate()
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
                <p className="text-xs text-white/50 mt-0.5 truncate">
                  {elapsed(plant.phaseStartDate)} in {plant.phase}
                </p>
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
            <p className="text-xs text-muted truncate">
              {elapsed(plant.phaseStartDate)} in {plant.phase}
            </p>
          </div>
          <span className="text-xs text-fern capitalize font-medium flex-shrink-0">{plant.phase}</span>
          <ViewToggle />
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
                <LogEntry key={log.logId} log={log} envMap={envMap} onDelete={deleteLog.mutate} />
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
            <TimelineView logs={logs ?? []} envMap={envMap} onDelete={deleteLog.mutate} />
          )}
        </div>
      )}

      <AddLogSheet
        open={logSheetOpen}
        onClose={() => setLogSheetOpen(false)}
        plant={plant}
        defaultDate={selectedDate ?? undefined}
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
