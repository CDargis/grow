import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useParams, useNavigate } from 'react-router-dom'
import { ArrowLeft, Droplets, Zap, Scissors, Ruler, MessageSquare, Camera, Wind, ChevronRight } from 'lucide-react'
import { api } from '@/api/client'
import { BottomSheet } from '@/components/BottomSheet'
import { DatePicker } from '@/components/DatePicker'
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
    case 'watering': { const d = log.data as any; return `${d.amount} ${d.unit}` }
    default:         return null
  }
}

function LogEntry({ log, envMap }: { log: Log; envMap: Map<string, string> }) {
  const summary = logSummary(log, envMap)
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
  const [envSheetOpen, setEnvSheetOpen] = useState(false)
  const [selectedDate, setSelectedDate] = useState<string | null>(null)

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
      {/* Header */}
      <div className="flex items-center gap-3 p-4 border-b border-border">
        <button onClick={() => navigate(-1)} className="text-dim active:opacity-70">
          <ArrowLeft size={20} />
        </button>
        <div className="flex-1 min-w-0">
          <h1 className="font-semibold text-primary">{plant.name}</h1>
          <p className="text-xs text-dim">{plant.strain}</p>
        </div>
        <span className="text-xs text-fern capitalize font-medium">{plant.phase}</span>
      </div>

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

      <ChangeEnvironmentSheet
        open={envSheetOpen}
        onClose={() => setEnvSheetOpen(false)}
        currentEnvId={plant.environmentId}
        plantId={plant.plantId}
      />
    </div>
  )
}
