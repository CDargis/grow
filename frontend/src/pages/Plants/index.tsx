import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { Plus } from 'lucide-react'
import { api } from '@/api/client'

export function PlantsPage() {
  const { data: plants, isLoading } = useQuery({
    queryKey: ['plants'],
    queryFn: api.plants.list,
  })

  return (
    <div className="p-4">
      <div className="flex items-center justify-between mb-4">
        <h1 className="text-lg font-semibold">Plants</h1>
        <button className="flex items-center gap-1 text-sm text-fern active:opacity-70">
          <Plus size={18} />
          Add
        </button>
      </div>

      {isLoading && (
        <div className="text-muted text-center py-16">Loading…</div>
      )}

      <div className="space-y-2">
        {plants?.map(plant => (
          <Link
            key={plant.plantId}
            to={`/plants/${plant.plantId}`}
            className="flex items-center gap-3 p-4 bg-surface rounded-xl border border-border"
          >
            <div className="w-10 h-10 rounded-full bg-raised border border-border flex items-center justify-center flex-shrink-0">
              🌱
            </div>
            <div className="flex-1 min-w-0">
              <p className="font-medium truncate">{plant.name}</p>
              <p className="text-xs text-dim truncate">{plant.strain}</p>
            </div>
            <span className="text-xs text-dim capitalize">{plant.phase}</span>
          </Link>
        ))}
      </div>
    </div>
  )
}
