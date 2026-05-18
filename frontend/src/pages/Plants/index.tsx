import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { Plus } from 'lucide-react'
import { api } from '@/api/client'
import { AddPlantSheet } from '@/components/AddPlantSheet'
import type { Plant, PlantPhase } from '@/types'

const PHASE_COLORS: Record<PlantPhase, string> = {
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
        {plant.avatarKey
          ? <img src={plant.avatarKey} alt={plant.name} className="w-full h-full object-cover rounded-full" />
          : '🌱'}
      </div>
      <div className="flex-1 min-w-0">
        <p className="font-semibold text-primary truncate">{plant.name}</p>
        <p className="text-sm text-dim truncate">{plant.strain}</p>
      </div>
      <span className={`text-xs font-medium capitalize flex-shrink-0 ${PHASE_COLORS[plant.phase] ?? 'text-muted'}`}>
        {plant.phase}
      </span>
    </Link>
  )
}

export function PlantsPage() {
  const [addOpen, setAddOpen] = useState(false)

  const { data: plants, isLoading } = useQuery({
    queryKey: ['plants'],
    queryFn: api.plants.list,
  })

  const active   = plants?.filter(p => p.phase !== 'archived') ?? []
  const archived = plants?.filter(p => p.phase === 'archived') ?? []

  return (
    <div className="p-4">
      <div className="flex items-center justify-between mb-4">
        <h1 className="text-lg font-semibold">Plants</h1>
        <button
          onClick={() => setAddOpen(true)}
          className="flex items-center gap-1 text-sm text-fern active:opacity-70"
        >
          <Plus size={18} />
          Add
        </button>
      </div>

      {isLoading && (
        <div className="text-muted text-center py-16">Loading…</div>
      )}

      {!isLoading && active.length === 0 && (
        <div className="text-muted text-center py-16 text-sm">No plants yet.</div>
      )}

      <div className="space-y-2">
        {active.map(plant => <PlantCard key={plant.plantId} plant={plant} />)}
      </div>

      {archived.length > 0 && (
        <div className="mt-6">
          <p className="text-xs text-muted uppercase tracking-wide font-medium mb-2">Archived</p>
          <div className="space-y-2">
            {archived.map(plant => <PlantCard key={plant.plantId} plant={plant} />)}
          </div>
        </div>
      )}

      <AddPlantSheet open={addOpen} onClose={() => setAddOpen(false)} />
    </div>
  )
}
