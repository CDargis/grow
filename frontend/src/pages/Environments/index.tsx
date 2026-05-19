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
            <div className="w-10 h-10 rounded-full bg-raised border border-border flex items-center justify-center flex-shrink-0 text-xl">
              {ENV_ICONS[env.type] ?? '📦'}
            </div>
            <div className="flex-1 min-w-0">
              <p className="font-medium truncate">{env.name}</p>
              <p className="text-xs text-dim capitalize">{env.type}</p>
            </div>
            {env.lightSchedule && (
              <span className="text-xs text-dim font-mono">{env.lightSchedule}</span>
            )}
          </button>
        ))}
      </div>

      <AddEnvironmentSheet
        open={addOpen}
        onClose={() => setAddOpen(false)}
      />
      <AddEnvironmentSheet
        open={!!editing}
        onClose={() => setEditing(null)}
        environment={editing ?? undefined}
      />
    </div>
  )
}
