import { useState, useEffect } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { BottomSheet } from './BottomSheet'
import { api } from '@/api/client'
import type { Plant, PlantType } from '@/types'

const inputCls = 'w-full bg-raised border border-border rounded-lg px-3 py-2.5 text-sm text-primary placeholder:text-muted focus:outline-none focus:border-fern'
const labelCls = 'block text-xs text-dim mb-1'

const PLANT_TYPE_LABELS: Record<PlantType, string> = {
  autoflower:  'Autoflower',
  photoperiod: 'Photoperiod',
  unknown:     'Unknown / TBD',
}

interface Props {
  open: boolean
  onClose: () => void
  plant: Plant
}

export function EditPlantSheet({ open, onClose, plant }: Props) {
  const qc = useQueryClient()
  const [name,      setName]      = useState(plant.name)
  const [strain,    setStrain]    = useState(plant.strain)
  const [genetics,  setGenetics]  = useState(plant.genetics ?? '')
  const [seedBank,  setSeedBank]  = useState(plant.seedBank ?? '')
  const [plantType, setPlantType] = useState<PlantType | ''>(plant.plantType ?? '')
  const [potSizeGal,        setPotSizeGal]        = useState(plant.potSizeGal?.toString() ?? '')
  const [potSizeDiameterIn, setPotSizeDiameterIn] = useState(plant.potSizeDiameterIn?.toString() ?? '')

  useEffect(() => {
    if (open) {
      setName(plant.name)
      setStrain(plant.strain)
      setGenetics(plant.genetics ?? '')
      setSeedBank(plant.seedBank ?? '')
      setPlantType(plant.plantType ?? '')
      setPotSizeGal(plant.potSizeGal?.toString() ?? '')
      setPotSizeDiameterIn(plant.potSizeDiameterIn?.toString() ?? '')
    }
  }, [open, plant])

  const mutation = useMutation({
    mutationFn: () => api.plants.update(plant.plantId, {
      name:      name.trim(),
      strain:    strain.trim(),
      genetics:  genetics.trim() || undefined,
      seedBank:  seedBank.trim() || undefined,
      plantType: plantType || undefined,
      potSizeGal:        potSizeGal ? Number(potSizeGal) : undefined,
      potSizeDiameterIn: potSizeDiameterIn ? Number(potSizeDiameterIn) : undefined,
    }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['plant', plant.plantId] })
      qc.invalidateQueries({ queryKey: ['plants'] })
      onClose()
    },
  })

  function submit(e: React.FormEvent) {
    e.preventDefault()
    if (!name.trim() || !strain.trim()) return
    mutation.mutate()
  }

  return (
    <BottomSheet title="Edit Plant" open={open} onClose={onClose}>
      <form onSubmit={submit} className="space-y-3 mt-2">
        <div>
          <label className={labelCls}>Name *</label>
          <input
            className={inputCls}
            value={name}
            onChange={e => setName(e.target.value)}
            required
          />
        </div>

        <div>
          <label className={labelCls}>Strain *</label>
          <input
            className={inputCls}
            value={strain}
            onChange={e => setStrain(e.target.value)}
            required
          />
        </div>

        <div className="grid grid-cols-2 gap-3">
          <div>
            <label className={labelCls}>Genetics</label>
            <input
              className={inputCls}
              placeholder="e.g. Sativa-dominant"
              value={genetics}
              onChange={e => setGenetics(e.target.value)}
            />
          </div>
          <div>
            <label className={labelCls}>Seed Bank</label>
            <input
              className={inputCls}
              placeholder="e.g. ILGM"
              value={seedBank}
              onChange={e => setSeedBank(e.target.value)}
            />
          </div>
        </div>

        <div>
          <label className={labelCls}>Plant Type</label>
          <select
            className={inputCls}
            value={plantType}
            onChange={e => setPlantType(e.target.value as PlantType | '')}
          >
            <option value="" className="bg-raised">Select type…</option>
            {(Object.keys(PLANT_TYPE_LABELS) as PlantType[]).map(t => (
              <option key={t} value={t} className="bg-raised">{PLANT_TYPE_LABELS[t]}</option>
            ))}
          </select>
        </div>

        <div>
          <label className={labelCls}>Pot size</label>
          <div className="grid grid-cols-2 gap-3">
            <div className="flex gap-2 items-center">
              <input
                type="number" step="0.1" className={inputCls} placeholder="5"
                value={potSizeGal}
                onChange={e => setPotSizeGal(e.target.value)}
              />
              <span className="text-xs text-muted flex-shrink-0">gal</span>
            </div>
            <div className="flex gap-2 items-center">
              <input
                type="number" step="0.1" className={inputCls} placeholder="12"
                value={potSizeDiameterIn}
                onChange={e => setPotSizeDiameterIn(e.target.value)}
              />
              <span className="text-xs text-muted flex-shrink-0">in diameter</span>
            </div>
          </div>
          <p className="text-[11px] text-muted mt-1">
            Either or both — used to scale dry amendment doses in the feed wizard, since those are labeled per pot size, not per water amount.
          </p>
        </div>

        {mutation.isError && (
          <p className="text-red-400 text-sm">{(mutation.error as Error).message}</p>
        )}

        <button
          type="submit"
          disabled={mutation.isPending}
          className="w-full py-3 bg-fern text-base font-semibold rounded-xl active:opacity-80 disabled:opacity-50 mt-2"
        >
          {mutation.isPending ? 'Saving…' : 'Save'}
        </button>
      </form>
    </BottomSheet>
  )
}
