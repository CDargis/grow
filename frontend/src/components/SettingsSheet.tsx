import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { BottomSheet } from './BottomSheet'
import { api } from '@/api/client'
import type { PlantsLayoutMode } from '@/types'

interface SettingsSheetProps {
  open: boolean
  onClose: () => void
}

export function SettingsSheet({ open, onClose }: SettingsSheetProps) {
  const qc = useQueryClient()
  const { data: settings } = useQuery({
    queryKey: ['settings'],
    queryFn:  api.settings.get,
    enabled:  open,
  })
  const updateSettings = useMutation({
    mutationFn: (body: Parameters<typeof api.settings.update>[0]) => api.settings.update(body),
    onSuccess:  () => qc.invalidateQueries({ queryKey: ['settings'] }),
  })

  const layoutMode: PlantsLayoutMode = settings?.plantsLayoutMode ?? 'grid'

  return (
    <BottomSheet title="Settings" open={open} onClose={onClose}>
      <div className="flex flex-col gap-2">
        <p className="text-xs text-muted mb-1">Plants screen layout</p>
        <div className="flex flex-col gap-2">
          {([
            { mode: 'grid'  as PlantsLayoutMode, label: 'Grid',       desc: 'Cards fill the screen in columns and rows' },
            { mode: 'rows'  as PlantsLayoutMode, label: 'Rows',       desc: 'One column, rows fill the screen' },
            { mode: 'fixed' as PlantsLayoutMode, label: 'Fixed size', desc: 'Original list layout' },
          ]).map(({ mode, label, desc }) => (
            <button
              key={mode}
              onClick={() => updateSettings.mutate({ plantsLayoutMode: mode })}
              className={`flex items-start gap-0.5 flex-col px-3 py-2.5 rounded-xl border text-left transition-colors ${
                layoutMode === mode
                  ? 'bg-fern/20 text-fern border-fern/40'
                  : 'bg-raised text-muted border-border'
              }`}
            >
              <span className="text-sm font-medium">{label}</span>
              <span className="text-xs opacity-70">{desc}</span>
            </button>
          ))}
        </div>
      </div>
    </BottomSheet>
  )
}
