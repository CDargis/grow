import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { BottomSheet } from './BottomSheet'
import { api } from '@/api/client'
import type { PlantPhase, CreatePlantRequest } from '@/types'

const PHASES: PlantPhase[] = ['germination', 'seedling', 'veg', 'flower', 'harvest', 'drying', 'curing']

const inputCls = 'w-full bg-raised border border-border rounded-lg px-3 py-2.5 text-sm text-primary placeholder:text-muted focus:outline-none focus:border-fern'
const labelCls = 'block text-xs text-dim mb-1'

interface Props {
  open: boolean
  onClose: () => void
}

export function AddPlantSheet({ open, onClose }: Props) {
  const qc = useQueryClient()
  const [form, setForm] = useState<CreatePlantRequest>({
    name: '', strain: '', phase: 'seedling',
  })

  const { data: envs } = useQuery({
    queryKey: ['environments'],
    queryFn: api.environments.list,
    enabled: open,
  })

  const mutation = useMutation({
    mutationFn: api.plants.create,
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['plants'] })
      setForm({ name: '', strain: '', phase: 'seedling' })
      onClose()
    },
  })

  function submit(e: React.FormEvent) {
    e.preventDefault()
    if (!form.name.trim() || !form.strain.trim()) return
    mutation.mutate(form)
  }

  return (
    <BottomSheet title="New Plant" open={open} onClose={onClose}>
      <form onSubmit={submit} className="space-y-3 mt-2">
        <div>
          <label className={labelCls}>Name *</label>
          <input
            className={inputCls}
            placeholder="e.g. Blue Dream #1"
            value={form.name}
            onChange={e => setForm(f => ({ ...f, name: e.target.value }))}
            required
          />
        </div>

        <div>
          <label className={labelCls}>Strain *</label>
          <input
            className={inputCls}
            placeholder="e.g. Blue Dream"
            value={form.strain}
            onChange={e => setForm(f => ({ ...f, strain: e.target.value }))}
            required
          />
        </div>

        <div className="grid grid-cols-2 gap-3">
          <div>
            <label className={labelCls}>Genetics</label>
            <input
              className={inputCls}
              placeholder="e.g. Sativa-dominant"
              value={form.genetics ?? ''}
              onChange={e => setForm(f => ({ ...f, genetics: e.target.value || undefined }))}
            />
          </div>
          <div>
            <label className={labelCls}>Seed Bank</label>
            <input
              className={inputCls}
              placeholder="e.g. ILGM"
              value={form.seedBank ?? ''}
              onChange={e => setForm(f => ({ ...f, seedBank: e.target.value || undefined }))}
            />
          </div>
        </div>

        <div>
          <label className={labelCls}>Phase</label>
          <select
            className={inputCls}
            value={form.phase}
            onChange={e => setForm(f => ({ ...f, phase: e.target.value as PlantPhase }))}
          >
            {PHASES.map(p => (
              <option key={p} value={p} className="bg-raised capitalize">{p}</option>
            ))}
          </select>
        </div>

        {envs && envs.length > 0 && (
          <div>
            <label className={labelCls}>Environment</label>
            <select
              className={inputCls}
              value={form.environmentId ?? ''}
              onChange={e => setForm(f => ({ ...f, environmentId: e.target.value || undefined }))}
            >
              <option value="" className="bg-raised">None</option>
              {envs.map(env => (
                <option key={env.environmentId} value={env.environmentId} className="bg-raised">
                  {env.name}
                </option>
              ))}
            </select>
          </div>
        )}

        {mutation.isError && (
          <p className="text-red-400 text-sm">{(mutation.error as Error).message}</p>
        )}

        <button
          type="submit"
          disabled={mutation.isPending}
          className="w-full py-3 bg-fern text-base font-semibold rounded-xl active:opacity-80 disabled:opacity-50 mt-2"
        >
          {mutation.isPending ? 'Adding…' : 'Add Plant'}
        </button>
      </form>
    </BottomSheet>
  )
}
