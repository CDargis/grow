import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Plus } from 'lucide-react'
import { api } from '@/api/client'
import { AddEnvironmentSheet } from '@/components/AddEnvironmentSheet'
import type { Environment } from '@/types'

const ENV_ICONS: Record<string, string> = {
  tent:       '⛺',
  outdoor:    '🌿',
  garage:     '🏠',
  basement:   '🪨',
  room:       '🚪',
  greenhouse: '🌱',
  other:      '📦',
}

function EnvPhoto({ photoKey, name }: { photoKey: string; name: string }) {
  const { data } = useQuery({
    queryKey: ['media-url', photoKey],
    queryFn: () => api.media.getDownloadUrl(photoKey),
    staleTime: 50 * 60 * 1000,
  })
  if (!data?.url) return <span className="text-xl">{ENV_ICONS['other']}</span>
  return <img src={data.url} alt={name} className="w-full h-full object-cover rounded-full" />
}

export function EnvironmentsPage() {
  const [editing, setEditing] = useState<Environment | null>(null)
  const [addOpen, setAddOpen] = useState(false)

  const { data: envs, isLoading } = useQuery({
    queryKey: ['environments'],
    queryFn: api.environments.list,
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

      {isLoading && (
        <div className="text-muted text-center py-16">Loading…</div>
      )}

      <div className="space-y-2">
        {envs?.map((env: Environment) => (
          <button
            key={env.environmentId}
            onClick={() => setEditing(env)}
            className="w-full flex items-center gap-3 p-4 bg-surface rounded-xl border border-border active:scale-[0.98] transition-transform text-left"
          >
            <div className="w-12 h-12 rounded-full bg-raised border border-border flex items-center justify-center flex-shrink-0 text-xl overflow-hidden">
              {env.photoKey
                ? <EnvPhoto photoKey={env.photoKey} name={env.name} />
                : <span>{ENV_ICONS[env.type] ?? '📦'}</span>}
            </div>
            <div className="flex-1 min-w-0">
              <p className="font-medium truncate text-primary">{env.name}</p>
              <p className="text-xs text-dim capitalize">{env.type}</p>
            </div>
            {env.lightSchedule && (
              <span className="text-xs text-dim font-mono flex-shrink-0">{env.lightSchedule}</span>
            )}
          </button>
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
