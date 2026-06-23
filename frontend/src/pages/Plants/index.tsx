import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { Plus, ChevronDown, ChevronUp } from 'lucide-react'
import { api } from '@/api/client'
import { AddPlantSheet } from '@/components/AddPlantSheet'
import { MediaImage } from '@/components/MediaImage'
import type { Plant, PlantPhase } from '@/types'

const PAST_PHASES: PlantPhase[] = ['archived', 'dead']

const PHASE_COLORS: Record<PlantPhase, string> = {
  germination: 'text-yellow-400',
  seedling:    'text-lime',
  veg:         'text-fern',
  flower:      'text-purple-400',
  harvest:     'text-orange-400',
  drying:      'text-amber-500',
  curing:      'text-amber-400',
  archived:    'text-muted',
  dead:        'text-red-400',
}

function PlantCard({ plant, past = false }: { plant: Plant; past?: boolean }) {
  const badge = plant.phase === 'dead' ? '💀' : plant.phase === 'archived' ? '✓' : null
  return (
    <Link
      to={`/plants/${plant.plantId}`}
      className={`block bg-surface rounded-xl border border-border overflow-hidden active:scale-[0.99] transition-transform ${past ? 'opacity-50' : ''}`}
    >
      <div className="relative h-28 bg-raised flex items-center justify-center text-5xl overflow-hidden">
        {plant.avatarKey
          ? <MediaImage photoKey={plant.avatarKey} alt={plant.name} className="w-full h-full object-cover" fallback={<span>🌱</span>} />
          : '🌱'}
        {badge && (
          <span className="absolute top-2 right-2 w-6 h-6 rounded-full bg-black/50 backdrop-blur-sm flex items-center justify-center text-xs leading-none">
            {badge}
          </span>
        )}
        <div className="absolute bottom-0 left-0 right-0 bg-gradient-to-t from-black/60 to-transparent px-3 pt-6 pb-2.5">
          <p className="font-semibold text-white truncate">{plant.name}</p>
          <div className="flex items-center justify-between gap-2">
            <p className="text-xs text-white/60 truncate">{plant.strain}</p>
            <span className={`text-xs font-medium capitalize flex-shrink-0 ${PHASE_COLORS[plant.phase] ?? 'text-muted'}`}>
              {plant.phase}
            </span>
          </div>
        </div>
      </div>
    </Link>
  )
}

export function PlantsPage() {
  const [addOpen, setAddOpen]   = useState(false)
  const [showPast, setShowPast] = useState(false)

  const { data: plants, isLoading } = useQuery({
    queryKey: ['plants'],
    queryFn: api.plants.list,
  })

  const active   = plants?.filter(p => !PAST_PHASES.includes(p.phase)) ?? []
  const past     = plants?.filter(p =>  PAST_PHASES.includes(p.phase)) ?? []
  const nArchived = past.filter(p => p.phase === 'archived').length
  const nDead     = past.filter(p => p.phase === 'dead').length

  const pastLabel = [
    nArchived > 0 && `${nArchived} archived`,
    nDead     > 0 && `${nDead} dead`,
  ].filter(Boolean).join(' · ')

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

      {!isLoading && active.length === 0 && past.length === 0 && (
        <div className="text-muted text-center py-16 text-sm">No plants yet.</div>
      )}

      <div className="space-y-2">
        {active.map(plant => <PlantCard key={plant.plantId} plant={plant} />)}
      </div>

      {past.length > 0 && (
        <div className="mt-6">
          <button
            onClick={() => setShowPast(p => !p)}
            className="flex items-center justify-between w-full mb-2 group"
          >
            <span className="text-xs text-muted uppercase tracking-wide font-medium">Past Grows</span>
            <span className="flex items-center gap-1 text-xs text-muted">
              {pastLabel}
              {showPast ? <ChevronUp size={14} /> : <ChevronDown size={14} />}
            </span>
          </button>
          {showPast && (
            <div className="space-y-2">
              {past.map(plant => <PlantCard key={plant.plantId} plant={plant} past />)}
            </div>
          )}
        </div>
      )}

      <AddPlantSheet open={addOpen} onClose={() => setAddOpen(false)} />
    </div>
  )
}
