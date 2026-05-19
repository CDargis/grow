import { useState, useRef } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useParams, useNavigate } from 'react-router-dom'
import { ArrowLeft, Droplets, Zap, Scissors, Ruler, MessageSquare, Camera, Wind, ChevronRight, Plus } from 'lucide-react'
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
      return `${d.fromPhase} → ${d.toPhase}`
    }
    case 'note':     return (log.data as any).text
    case 'height':   { const d = log.data as any; return `${d.height} ${d.unit}` }
    case 'watering': { const d = log.data as any; return d.amount != null ? `${d.amount} ${d.unit}` : d.unit }
    case 'photo':    return (log.data as any).caption ?? null
    default:         return null
  }
}

function LogEntry({ log, envMap }: { log: Log; envMap: Map<string, string> }) {
  const summary = logSummary(log, envMap)
  const photoKey = log.logType === 'photo' ? (log.data as any).photoKey as string | undefined : undefined
  return (
    <div className="flex items-start gap-3 py-3 border-b border-border last:border-0">
      <div className="w-7 h-7 rounded-full bg-raised border border-border flex items-center justify-center text-dim flex-shrink-0 mt-0.5">
        {LOG_ICONS[log.logType]}
      </div>
      <div className="flex-1 min-w-0">
        <div className="flex items-center justify-between gap-2">
          <span className="text-sm font-medium capitalize text-primary">
            {log.logType.replace(/_/g, ' ')}
          </span>
          <span className="text-xs text-muted flex-shrink-0">
            {new Date(log.loggedAt).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}
          </span>
        </div>
        {summary && <p className="text-xs text-dim mt-0.5 truncate">{summary}</p>}
        {photoKey && (
          <div className="mt-2 w-28 h-28 rounded-lg overflow-hidden">
            <MediaImage photoKey={photoKey} alt="photo log" className="w-full h-full object-cover" />
          </div>
        )}
      </div>
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

export function PlantDetailPage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const qc = useQueryClient()
  const avatarRef = useRef<HTMLInputElement>(null)
  const [envSheetOpen, setEnvSheetOpen] = useState(false)
  const [logSheetOpen, setLogSheetOpen] = useState(false)
  const [selectedDate, setSelectedDate] = useState<string | null>(null)
  const [avatarUploading, setAvatarUploading] = useState(false)

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
  const displayLogs = selectedDate
    ? (logs?.filter((l: Log) => l.date === selectedDate) ?? [])
    : (logs ?? [])

  if (plantLoading) {
    return <div className="flex items-center justify-center h-full text-muted">Loading…</div>
  }
  if (!plant) {
    return <div className="flex items-center justify-center h-full text-muted">Plant not found.</div>
  }

  const currentEnvName = plant.environmentId ? (envMap.get(plant.environmentId) ?? plant.environmentId) : null

  return (
    <div className="flex flex-col">
      <input ref={avatarRef} type="file" accept="image/*" className="hidden" onChange={handleAvatarChange} />

      {plant.avatarKey ? (
        /* Hero banner when photo is set */
        <div className="relative h-60 w-full flex-shrink-0">
          <MediaImage
            photoKey={plant.avatarKey}
            alt={plant.name}
            className="w-full h-full object-cover"
          />
          <div className="absolute inset-0 bg-gradient-to-t from-black/75 via-black/20 to-transparent" />

          {/* Top controls */}
          <div className="absolute top-0 left-0 right-0 flex items-center justify-between p-4 pt-5">
            <button
              onClick={() => navigate(-1)}
              className="w-9 h-9 rounded-full bg-black/40 backdrop-blur-sm flex items-center justify-center text-white active:opacity-70"
            >
              <ArrowLeft size={20} />
            </button>
            <button
              onClick={() => setLogSheetOpen(true)}
              className="w-9 h-9 rounded-full bg-black/40 backdrop-blur-sm flex items-center justify-center text-white active:opacity-70"
            >
              <Plus size={20} />
            </button>
          </div>

          {/* Bottom info overlay — tap to change photo */}
          <button
            onClick={() => avatarRef.current?.click()}
            disabled={avatarUploading}
            className="absolute bottom-0 left-0 right-0 p-4 text-left"
          >
            <div className="flex items-end justify-between gap-2">
              <div className="min-w-0">
                <h1 className="font-bold text-xl text-white leading-tight truncate">{plant.name}</h1>
                <p className="text-sm text-white/70 truncate">{plant.strain}</p>
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
        <div className="flex items-center gap-3 p-4 border-b border-border flex-shrink-0">
          <button onClick={() => navigate(-1)} className="text-dim active:opacity-70">
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
            <p className="text-xs text-dim">{plant.strain}</p>
          </div>
          <span className="text-xs text-fern capitalize font-medium">{plant.phase}</span>
          <button onClick={() => setLogSheetOpen(true)} className="text-fern active:opacity-70 ml-2">
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

      {/* Date picker */}
      <DatePicker
        activeDates={activeDates}
        selected={selectedDate}
        onSelect={setSelectedDate}
      />

      {/* Log timeline */}
      <div className="p-4">
        {selectedDate && (
          <p className="text-xs text-muted mb-2 uppercase tracking-wide font-medium">
            {new Date(selectedDate + 'T00:00:00').toLocaleDateString([], { weekday: 'long', month: 'long', day: 'numeric' })}
          </p>
        )}
        {!selectedDate && (
          <p className="text-xs text-muted mb-2 uppercase tracking-wide font-medium">All Activity</p>
        )}

        {logsLoading ? (
          <div className="text-muted text-sm">Loading…</div>
        ) : displayLogs.length === 0 ? (
          <div className="text-muted text-sm py-8 text-center">
            {selectedDate ? 'No activity on this day.' : 'No logs yet.'}
          </div>
        ) : (
          <div>
            {displayLogs.map((log: Log) => (
              <LogEntry key={log.logId} log={log} envMap={envMap} />
            ))}
          </div>
        )}
      </div>

      <AddLogSheet
        open={logSheetOpen}
        onClose={() => setLogSheetOpen(false)}
        plant={plant}
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
