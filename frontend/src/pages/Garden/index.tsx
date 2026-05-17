import { useQuery } from '@tanstack/react-query'
import { useSearchParams, Link } from 'react-router-dom'
import { api } from '@/api/client'
import type { Plant } from '@/types'

const PHASE_COLORS: Record<string, string> = {
  germination: 'text-yellow-400',
  seedling:    'text-lime',
  veg:         'text-fern',
  flower:      'text-purple-400',
  harvest:     'text-orange-400',
  drying:      'text-amber-500',
  curing:      'text-amber-400',
  archived:    'text-muted',
}

function PlantCard({ plant }: { plant: Plant }) {
  return (
    <Link
      to={`/plants/${plant.plantId}`}
      className="flex items-center gap-3 p-4 bg-surface rounded-xl border border-border active:scale-[0.98] transition-transform"
    >
      <div className="w-12 h-12 rounded-full bg-raised border border-border flex items-center justify-center text-2xl flex-shrink-0">
        {plant.avatarKey ? (
          <img src={plant.avatarKey} alt={plant.name} className="w-full h-full object-cover rounded-full" />
        ) : (
          '🌱'
        )}
      </div>
      <div className="flex-1 min-w-0">
        <p className="font-semibold text-primary truncate">{plant.name}</p>
        <p className="text-sm text-dim truncate">{plant.strain}</p>
      </div>
      <div className="flex flex-col items-end gap-1">
        <span className={`text-xs font-medium capitalize ${PHASE_COLORS[plant.phase] ?? 'text-muted'}`}>
          {plant.phase}
        </span>
      </div>
    </Link>
  )
}

export function GardenPage() {
  const [params] = useSearchParams()
  const date = params.get('date') ?? new Date().toISOString().slice(0, 10)

  const { data: plants, isLoading } = useQuery({
    queryKey: ['plants'],
    queryFn: api.plants.list,
  })

  const { data: logsToday } = useQuery({
    queryKey: ['logs', date],
    queryFn: () => api.logs.listForDate(date),
  })

  const activePlantIds = new Set(logsToday?.map(l => l.plantId) ?? [])

  const displayPlants = plants?.filter(p =>
    p.phase !== 'archived' && (params.get('date') ? activePlantIds.has(p.plantId) : true)
  ) ?? []

  if (isLoading) {
    return <div className="flex items-center justify-center h-full text-muted">Loading…</div>
  }

  return (
    <div className="p-4 space-y-3">
      {displayPlants.length === 0 ? (
        <div className="text-center text-muted py-16">
          {params.get('date') ? 'No activity on this day.' : 'No plants yet.'}
        </div>
      ) : (
        displayPlants.map(plant => <PlantCard key={plant.plantId} plant={plant} />)
      )}
    </div>
  )
}
