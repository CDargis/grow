import { useState, useEffect } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { BottomSheet } from './BottomSheet'
import { api } from '@/api/client'
import type { Environment, EnvironmentType, CreateEnvironmentRequest } from '@/types'

const TYPES: EnvironmentType[] = ['tent', 'outdoor', 'garage', 'basement', 'room', 'greenhouse', 'other']

const inputCls = 'w-full bg-raised border border-border rounded-lg px-3 py-2.5 text-sm text-primary placeholder:text-muted focus:outline-none focus:border-fern'
const labelCls = 'block text-xs text-dim mb-1'

const empty: CreateEnvironmentRequest = { name: '', type: 'tent' }

interface Props {
  open: boolean
  onClose: () => void
  environment?: Environment  // present = edit mode
}

export function AddEnvironmentSheet({ open, onClose, environment }: Props) {
  const qc = useQueryClient()
  const isEdit = !!environment
  const [form, setForm] = useState<CreateEnvironmentRequest>(empty)

  useEffect(() => {
    if (open) {
      setForm(environment
        ? {
            name:           environment.name,
            type:           environment.type,
            lightSchedule:  environment.lightSchedule,
            targetTempF:    environment.targetTempF || undefined,
            targetHumidity: environment.targetHumidity || undefined,
          }
        : empty)
    }
  }, [open, environment])

  const mutation = useMutation({
    mutationFn: (body: CreateEnvironmentRequest) =>
      isEdit
        ? api.environments.update(environment!.environmentId, body)
        : api.environments.create(body),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['environments'] })
      onClose()
    },
  })

  function submit(e: React.FormEvent) {
    e.preventDefault()
    if (!form.name.trim()) return
    mutation.mutate(form)
  }

  return (
    <BottomSheet title={isEdit ? 'Edit Environment' : 'New Environment'} open={open} onClose={onClose}>
      <form onSubmit={submit} className="space-y-3 mt-2">
        <div>
          <label className={labelCls}>Name *</label>
          <input
            className={inputCls}
            placeholder="e.g. 4x4 Tent"
            value={form.name}
            onChange={e => setForm(f => ({ ...f, name: e.target.value }))}
            required
          />
        </div>

        <div>
          <label className={labelCls}>Type</label>
          <select
            className={inputCls}
            value={form.type}
            onChange={e => setForm(f => ({ ...f, type: e.target.value as EnvironmentType }))}
          >
            {TYPES.map(t => (
              <option key={t} value={t} className="bg-raised capitalize">{t}</option>
            ))}
          </select>
        </div>

        <div>
          <label className={labelCls}>Light Schedule</label>
          <input
            className={inputCls}
            placeholder="e.g. 18/6"
            value={form.lightSchedule ?? ''}
            onChange={e => setForm(f => ({ ...f, lightSchedule: e.target.value || undefined }))}
          />
        </div>

        <div className="grid grid-cols-2 gap-3">
          <div>
            <label className={labelCls}>Target Temp (°F)</label>
            <input
              type="number"
              className={inputCls}
              placeholder="75"
              value={form.targetTempF ?? ''}
              onChange={e => setForm(f => ({ ...f, targetTempF: e.target.value ? Number(e.target.value) : undefined }))}
            />
          </div>
          <div>
            <label className={labelCls}>Target Humidity (%)</label>
            <input
              type="number"
              className={inputCls}
              placeholder="55"
              value={form.targetHumidity ?? ''}
              onChange={e => setForm(f => ({ ...f, targetHumidity: e.target.value ? Number(e.target.value) : undefined }))}
            />
          </div>
        </div>

        {mutation.isError && (
          <p className="text-red-400 text-sm">{(mutation.error as Error).message}</p>
        )}

        <button
          type="submit"
          disabled={mutation.isPending}
          className="w-full py-3 bg-fern text-base font-semibold rounded-xl active:opacity-80 disabled:opacity-50 mt-2"
        >
          {mutation.isPending ? 'Saving…' : isEdit ? 'Save Changes' : 'Add Environment'}
        </button>
      </form>
    </BottomSheet>
  )
}
