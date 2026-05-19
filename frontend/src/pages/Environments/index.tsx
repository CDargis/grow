import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { Plus, Pencil, Leaf, ChevronDown } from 'lucide-react'
import { api } from '@/api/client'
import { AddEnvironmentSheet } from '@/components/AddEnvironmentSheet'
import { MediaImage } from '@/components/MediaImage'
import type { Environment, Plant, PlantPhase } from '@/types'

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

function EnvironmentCard({
  env,
  plants,
  onEdit,
}: {
  env: Environment
  plants: Plant[]
  onEdit: (env: Environment) => void
}) {
  const [expanded, setExpanded] = useState(false)

  const { data: photoData } = useQuery({
    queryKey: ['media-url', env.photoKey],
    queryFn: () => api.media.getDownloadUrl(env.photoKey!),
    enabled: !!env.photoKey,
    staleTime: 50 * 60 * 1000,
  })

  const assigned = plants.filter(
    p => p.environmentId === env.environmentId && p.phase !== 'archived' && p.phase !== 'dead'
  )

  return (
    <div className="rounded-2xl border border-border overflow-hidden">
      {/* Cover */}
      <button
        onClick={() => setExpanded(e => !e)}
        className="relative w-full h-40 block text-left"
      >
        {photoData?.url
          ? <img src={photoData.url} alt={env.name} className="w-full h-full object-cover" />
          : <div className="w-full h-full bg-gradient-to-br from-forest/50 to-raised" />
        }
        <div className="absolute inset-0 bg-gradient-to-t from-black/75 via-black/10 to-transparent" />

        {/* Plant count */}
        {assigned.length > 0 && (
          <div className="absolute top-3 left-3 flex items-center gap-1 bg-black/50 backdrop-blur-sm rounded-full px-2 py-1">
            <Leaf size={10} className="text-lime" />
            <span className="text-xs text-white font-medium">{assigned.length}</span>
          </div>
        )}

        {/* Edit */}
        <button
          onClick={e => { e.stopPropagation(); onEdit(env) }}
          className="absolute top-3 right-3 w-7 h-7 bg-black/50 backdrop-blur-sm rounded-full flex items-center justify-center active:opacity-60"
        >
          <Pencil size={12} className="text-white" />
        </button>

        {/* Name + meta */}
        <div className="absolute bottom-0 left-0 right-0 px-4 pb-3">
          <p className="font-semibold text-white text-base leading-tight">{env.name}</p>
          <div className="flex items-center gap-2 mt-0.5 text-xs text-white/60">
            <span className="capitalize">{env.type}</span>
            {env.lightSchedule && <><span>·</span><span className="font-mono">{env.lightSchedule}</span></>}
            {env.targetTempF   && <><span>·</span><span>{env.targetTempF}°F</span></>}
          </div>
        </div>

        {/* Expand chevron */}
        <ChevronDown
          size={16}
          className={`absolute bottom-3 right-4 text-white/50 transition-transform duration-200 ${expanded ? 'rotate-180' : ''}`}
        />
      </button>

      {/* Expanded plant list */}
      {expanded && (
        <div className="bg-surface border-t border-border">
          {assigned.length === 0 ? (
            <p className="text-xs text-muted text-center py-4">No active plants in this environment.</p>
          ) : (
            assigned.map(plant => (
              <Link
                key={plant.plantId}
                to={`/plants/${plant.plantId}`}
                className="flex items-center gap-3 px-4 py-3 border-b border-border last:border-0 active:bg-raised transition-colors"
              >
                <div className="w-8 h-8 rounded-full bg-raised border border-border flex items-center justify-center text-base flex-shrink-0 overflow-hidden">
                  {plant.avatarKey
                    ? <MediaImage photoKey={plant.avatarKey} alt={plant.name} className="w-full h-full object-cover" fallback={<span>🌱</span>} />
                    : '🌱'}
                </div>
                <div className="flex-1 min-w-0">
                  <p className="text-sm font-medium text-primary truncate">{plant.name}</p>
                  <p className="text-xs text-dim truncate">{plant.strain}</p>
                </div>
                <span className={`text-xs font-medium capitalize flex-shrink-0 ${PHASE_COLORS[plant.phase]}`}>
                  {plant.phase}
                </span>
              </Link>
            ))
          )}
        </div>
      )}
    </div>
  )
}

export function EnvironmentsPage() {
  const [editing, setEditing] = useState<Environment | null>(null)
  const [addOpen, setAddOpen] = useState(false)

  const { data: envs, isLoading: envsLoading } = useQuery({
    queryKey: ['environments'],
    queryFn: api.environments.list,
  })

  const { data: plants = [] } = useQuery({
    queryKey: ['plants'],
    queryFn: api.plants.list,
  })

  return (
    <div className="p-4">
      <div className="flex items-center justify-between mb-4">
        <h1 className="text-lg font-semibold">Environments</h1>
        <button
          onClick={() => setAddOpen(true)}
          className="flex items-center gap-1 text-sm text-fern active:opacity-70"
        >
          <Plus size={18} />
          Add
        </button>
      </div>

      {envsLoading && <div className="text-muted text-center py-16">Loading…</div>}

      <div className="space-y-3">
        {envs?.map((env: Environment) => (
          <EnvironmentCard
            key={env.environmentId}
            env={env}
            plants={plants as Plant[]}
            onEdit={setEditing}
          />
        ))}
      </div>

      <AddEnvironmentSheet open={addOpen} onClose={() => setAddOpen(false)} />
      <AddEnvironmentSheet
        open={!!editing}
        onClose={() => setEditing(null)}
        environment={editing ?? undefined}
      />
    </div>
  )
}
