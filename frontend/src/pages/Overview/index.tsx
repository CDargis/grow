import { useState } from 'react'
import { useQuery, useQueries, useMutation, useQueryClient } from '@tanstack/react-query'
import { useNavigate } from 'react-router-dom'
import { Droplets, Zap, Scissors, Ruler, MessageSquare, Camera, ArrowUp, ArrowDown } from 'lucide-react'
import { api } from '@/api/client'
import { MediaImage } from '@/components/MediaImage'
import { LogTypeReorderSheet, type LogTypeCatalogItem } from '@/components/LogTypeReorderSheet'
import { isFeedLog, logTypeLabel } from '@/lib/logDisplay'
import { useLongPress } from '@/lib/useLongPress'
import type { Log, LogType, Plant } from '@/types'

// ── Shared helpers ─────────────────────────────────────────────────────────────

function getPhotoKeys(data: any): string[] {
  if (Array.isArray(data?.photoKeys)) return data.photoKeys
  if (data?.photoKey) return [data.photoKey]
  return []
}

function relativeDate(dateStr: string): string {
  const [y, m, d]    = dateStr.split('-').map(Number)
  const [ty, tm, td] = todayDate().split('-').map(Number)
  const days = Math.round((Date.UTC(ty, tm - 1, td) - Date.UTC(y, m - 1, d)) / 86400000)
  if (days === 0) return 'today'
  if (days === 1) return 'yesterday'
  if (days < 7)  return `${days} days ago`
  const weeks = Math.floor(days / 7)
  return weeks === 1 ? '1 week ago' : `${weeks} weeks ago`
}

function todayDate() {
  return new Date().toLocaleDateString('en-CA')
}

function dateLabel(date: string): string {
  const today     = todayDate()
  const yesterday = new Date(Date.now() - 86400000).toLocaleDateString('en-CA')
  if (date === today)     return 'Today'
  if (date === yesterday) return 'Yesterday'
  return new Date(date + 'T12:00:00').toLocaleDateString([], { weekday: 'short', month: 'short', day: 'numeric' })
}

function feedSummary(log: Log): string | null {
  const d = log.data as any
  switch (log.logType) {
    case 'watering': {
      const parts: string[] = []
      if (d.amount != null) parts.push(`${d.amount} ${d.unit}`)
      const nuts = (d.nutrients as any[] ?? []).filter((n: any) => n.name).map((n: any) => n.name).join(', ')
      if (nuts)            parts.push(nuts)
      if (d.ph != null)    parts.push(`pH ${d.ph}`)
      return parts.length ? parts.join(' · ') : null
    }
    case 'feeding':    return (d.nutrients as any[] ?? []).filter((n: any) => n.name).map((n: any) => n.name).join(', ') || null
    case 'training':   return d.method ?? null
    case 'trimming':   return d.method ?? null
    case 'height':     return `${d.height} ${d.unit}`
    case 'note':       return d.text ?? null
    case 'photo':      return d.caption ?? null
    case 'transplant': return d.potSize ?? null
    default:           return null
  }
}

const LOG_ICONS: Partial<Record<LogType, React.ReactNode>> = {
  watering:  <Droplets size={14} />,
  feeding:   <Zap size={14} />,
  training:  <Scissors size={14} />,
  trimming:  <Scissors size={14} />,
  height:    <Ruler size={14} />,
  note:      <MessageSquare size={14} />,
  photo:     <Camera size={14} />,
}

// ── Sort mode ──────────────────────────────────────────────────────────────────

const SORT_LOG_TYPE_CATALOG: LogTypeCatalogItem[] = [
  { type: 'watering', label: 'Water / Feed', icon: <Droplets size={14} /> },
  { type: 'photo',    label: 'Photo',        icon: <Camera size={14} /> },
  { type: 'height',   label: 'Height',       icon: <Ruler size={14} /> },
  { type: 'note',     label: 'Note',         icon: <MessageSquare size={14} /> },
  { type: 'training', label: 'Training',     icon: <Scissors size={14} /> },
  { type: 'trimming', label: 'Trim',         icon: <Scissors size={14} /> },
]

const DEFAULT_SORT_CHIP_ORDER: LogType[] = SORT_LOG_TYPE_CATALOG.map(c => c.type)

function SortView({ plants }: { plants: Plant[] }) {
  const [logType, setLogType] = useState<LogType>('watering')
  const [dir, setDir]         = useState<'desc' | 'asc'>('asc')
  const [chipEditorOpen, setChipEditorOpen] = useState(false)

  const { data: activity, isLoading } = useQuery({
    queryKey: ['logs', 'last-by-type', logType],
    queryFn:  () => api.logs.lastByType(logType),
  })

  const qc = useQueryClient()
  const { data: settings } = useQuery({
    queryKey: ['settings'],
    queryFn:  api.settings.get,
  })
  const updateSettings = useMutation({
    mutationFn: (body: Parameters<typeof api.settings.update>[0]) => api.settings.update(body),
    onSuccess:  () => qc.invalidateQueries({ queryKey: ['settings'] }),
  })
  const chipOrder = settings?.sortChipOrder ?? DEFAULT_SORT_CHIP_ORDER
  const chipLongPress = useLongPress(() => setChipEditorOpen(true))

  const navigate = useNavigate()

  const activityMap = new Map(activity?.map(a => [a.plantId, a]) ?? [])

  const sorted = [...plants].sort((a, b) => {
    const aDate = activityMap.get(a.plantId)?.date
    const bDate = activityMap.get(b.plantId)?.date
    if (!aDate && !bDate) return 0
    if (!aDate) return dir === 'desc' ? 1 : -1
    if (!bDate) return dir === 'desc' ? -1 : 1
    return dir === 'desc' ? bDate.localeCompare(aDate) : aDate.localeCompare(bDate)
  })

  return (
    <div className="flex flex-col">
      {/* Log type chips */}
      <div className="flex gap-2 overflow-x-auto px-4 py-3 border-b border-border no-scrollbar" {...chipLongPress}>
        {chipOrder.map(type => {
          const item = SORT_LOG_TYPE_CATALOG.find(c => c.type === type)
          if (!item) return null
          return (
            <button
              key={type}
              onClick={() => setLogType(type)}
              className={`flex-shrink-0 px-3 py-1.5 rounded-full text-xs font-medium border transition-colors ${
                logType === type
                  ? 'bg-fern/20 text-fern border-fern/40'
                  : 'bg-raised text-muted border-border'
              }`}
            >
              {item.label}
            </button>
          )
        })}
      </div>

      {/* Sort toggle */}
      <div className="flex items-center justify-between px-4 py-2 border-b border-border">
        <span className="text-xs text-muted">
          {dir === 'desc' ? 'Most recent first' : 'Oldest first'}
        </span>
        <button
          onClick={() => setDir(d => d === 'desc' ? 'asc' : 'desc')}
          className="flex items-center gap-1.5 text-xs text-dim active:opacity-70"
        >
          {dir === 'desc' ? <ArrowDown size={13} /> : <ArrowUp size={13} />}
          <span>Sort</span>
        </button>
      </div>

      {/* Plant list */}
      {isLoading ? (
        <div className="text-muted text-sm text-center py-12">Loading…</div>
      ) : (
        <div className="divide-y divide-border">
          {sorted.map(plant => {
            const item = activityMap.get(plant.plantId)
            return (
              <button
                key={plant.plantId}
                onClick={() => navigate(`/plants/${plant.plantId}`)}
                className="w-full flex items-center gap-3 px-4 py-3 active:bg-raised text-left"
              >
                <div className="w-10 h-10 rounded-full overflow-hidden flex-shrink-0 bg-raised border border-border flex items-center justify-center text-lg">
                  {plant.avatarKey
                    ? <MediaImage photoKey={plant.avatarKey} alt={plant.name} className="w-full h-full object-cover" />
                    : <span>🌱</span>
                  }
                </div>
                <div className="flex-1 min-w-0">
                  <p className="text-sm font-medium text-primary truncate">{plant.name}</p>
                  <p className="text-xs text-dim truncate">{plant.strain}</p>
                </div>
                <span className={`text-xs flex-shrink-0 ${item ? 'text-muted' : 'text-red-400/70'}`}>
                  {item ? relativeDate(item.date) : 'never'}
                </span>
              </button>
            )
          })}
          {plants.length === 0 && (
            <p className="text-muted text-sm text-center py-12">No plants yet.</p>
          )}
        </div>
      )}

      <LogTypeReorderSheet
        open={chipEditorOpen}
        onClose={() => setChipEditorOpen(false)}
        title="Reorder Chips"
        catalog={SORT_LOG_TYPE_CATALOG}
        value={chipOrder}
        mode="plain"
        onSave={order => updateSettings.mutate({ sortChipOrder: order })}
      />
    </div>
  )
}

// ── Feed mode ──────────────────────────────────────────────────────────────────

function FeedView({ plants }: { plants: Plant[] }) {
  const navigate  = useNavigate()
  const plantMap  = new Map(plants.map(p => [p.plantId, p]))

  const dates = Array.from({ length: 7 }, (_, i) => {
    const d = new Date()
    d.setDate(d.getDate() - i)
    return d.toLocaleDateString('en-CA')
  })

  const results = useQueries({
    queries: dates.map(date => ({
      queryKey: ['logs', 'date', date],
      queryFn:  () => api.logs.listForDate(date),
    })),
  })

  const isLoading = results.some(r => r.isLoading)

  const allLogs: Log[] = results
    .flatMap(r => r.data ?? [])
    .filter(l => !['phase_change', 'environment_change', 'lighting_change', 'vpd_change'].includes(l.logType))
    .sort((a, b) => b.date.localeCompare(a.date) || b.loggedAt.localeCompare(a.loggedAt))

  const byDate = allLogs.reduce<Map<string, Log[]>>((acc, log) => {
    const group = acc.get(log.date) ?? []
    group.push(log)
    acc.set(log.date, group)
    return acc
  }, new Map())

  if (isLoading) {
    return <div className="text-muted text-sm text-center py-12">Loading…</div>
  }

  if (allLogs.length === 0) {
    return <div className="text-muted text-sm text-center py-12">No activity in the last 7 days.</div>
  }

  return (
    <div className="divide-y divide-border">
      {[...byDate.entries()].map(([date, logs]) => (
        <div key={date}>
          <p className="px-4 pt-3 pb-1 text-xs font-medium text-muted uppercase tracking-wide">
            {dateLabel(date)}
          </p>
          {logs.map(log => {
            const plant   = plantMap.get(log.plantId)
            const summary = feedSummary(log)
            const photoKeys = getPhotoKeys(log.data)
            return (
              <button
                key={log.logId}
                onClick={() => navigate(`/plants/${log.plantId}`)}
                className="w-full flex items-center gap-3 px-4 py-2.5 active:bg-raised text-left"
              >
                <div className="w-8 h-8 rounded-full overflow-hidden flex-shrink-0 bg-raised border border-border flex items-center justify-center text-sm">
                  {plant?.avatarKey
                    ? <MediaImage photoKey={plant.avatarKey} alt={plant.name} className="w-full h-full object-cover" />
                    : <span>🌱</span>
                  }
                </div>
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-1.5">
                    <span className="text-dim">{isFeedLog(log) ? <Zap size={14} /> : LOG_ICONS[log.logType] ?? null}</span>
                    <span className="text-xs font-medium text-primary">{plant?.name ?? log.plantId}</span>
                    <span className="text-xs text-muted capitalize">{logTypeLabel(log)}</span>
                  </div>
                  {summary && <p className="text-xs text-dim truncate">{summary}</p>}
                </div>
                {photoKeys[0] && (
                  <div className="w-9 h-9 rounded-lg overflow-hidden flex-shrink-0">
                    <MediaImage photoKey={photoKeys[0]} alt="photo" className="w-full h-full object-cover" />
                  </div>
                )}
              </button>
            )
          })}
        </div>
      ))}
    </div>
  )
}

// ── Page ───────────────────────────────────────────────────────────────────────

type Mode = 'feed' | 'sort'

export function OverviewPage() {
  const [mode, setMode] = useState<Mode>('sort')

  const { data: plants = [] } = useQuery({
    queryKey: ['plants'],
    queryFn:  api.plants.list,
  })

  const activePlants = plants.filter((p: Plant) => !['archived', 'dead'].includes(p.phase))

  return (
    <div className="flex flex-col">
      {/* Mode toggle */}
      <div className="flex border-b border-border px-4 pt-3 gap-4">
        {(['sort', 'feed'] as Mode[]).map(m => (
          <button
            key={m}
            onClick={() => setMode(m)}
            className={`pb-2.5 text-sm font-medium border-b-2 transition-colors capitalize ${
              mode === m
                ? 'border-fern text-fern'
                : 'border-transparent text-muted'
            }`}
          >
            {m === 'feed' ? 'Recent' : 'Sort'}
          </button>
        ))}
      </div>

      {mode === 'feed'
        ? <FeedView plants={activePlants} />
        : <SortView plants={activePlants} />
      }
    </div>
  )
}
