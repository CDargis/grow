import { useQuery } from '@tanstack/react-query'
import { useParams, useNavigate } from 'react-router-dom'
import { ArrowLeft, Droplets, Zap, Scissors, Ruler, MessageSquare, Camera } from 'lucide-react'
import { api } from '@/api/client'
import type { Log, LogType } from '@/types'

const LOG_ICONS: Record<LogType, React.ReactNode> = {
  watering:     <Droplets size={16} />,
  feeding:      <Zap size={16} />,
  training:     <Scissors size={16} />,
  trimming:     <Scissors size={16} />,
  transplant:   <span className="text-xs">🪴</span>,
  height:       <Ruler size={16} />,
  note:         <MessageSquare size={16} />,
  photo:        <Camera size={16} />,
  phase_change: <span className="text-xs">🔄</span>,
}

function LogEntry({ log }: { log: Log }) {
  return (
    <div className="flex items-start gap-3 py-3 border-b border-border last:border-0">
      <div className="w-7 h-7 rounded-full bg-raised border border-border flex items-center justify-center text-dim flex-shrink-0 mt-0.5">
        {LOG_ICONS[log.logType]}
      </div>
      <div className="flex-1 min-w-0">
        <div className="flex items-center justify-between gap-2">
          <span className="text-sm font-medium capitalize text-primary">
            {log.logType.replace('_', ' ')}
          </span>
          <span className="text-xs text-muted flex-shrink-0">
            {new Date(log.loggedAt).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}
          </span>
        </div>
        <p className="text-xs text-dim mt-0.5 truncate">
          {JSON.stringify(log.data)}
        </p>
      </div>
    </div>
  )
}

export function PlantDetailPage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()

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

  if (plantLoading) {
    return <div className="flex items-center justify-center h-full text-muted">Loading…</div>
  }

  if (!plant) {
    return <div className="flex items-center justify-center h-full text-muted">Plant not found.</div>
  }

  return (
    <div className="flex flex-col">
      {/* Header */}
      <div className="flex items-center gap-3 p-4 border-b border-border">
        <button onClick={() => navigate(-1)} className="text-dim active:opacity-70">
          <ArrowLeft size={20} />
        </button>
        <div className="flex-1">
          <h1 className="font-semibold text-primary">{plant.name}</h1>
          <p className="text-xs text-dim">{plant.strain}</p>
        </div>
        <span className="text-xs text-fern capitalize font-medium">{plant.phase}</span>
      </div>

      {/* Log timeline */}
      <div className="p-4">
        <p className="text-xs text-muted mb-2 uppercase tracking-wide font-medium">Activity</p>
        {logsLoading ? (
          <div className="text-muted text-sm">Loading…</div>
        ) : logs?.length === 0 ? (
          <div className="text-muted text-sm py-8 text-center">No logs yet.</div>
        ) : (
          <div>
            {logs?.map(log => <LogEntry key={log.logId} log={log} />)}
          </div>
        )}
      </div>
    </div>
  )
}
