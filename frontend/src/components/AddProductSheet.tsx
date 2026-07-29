import { useState, useEffect } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { BottomSheet } from './BottomSheet'
import { api } from '@/api/client'
import type { Product, ProductForm, CreateProductRequest } from '@/types'

const inputCls = 'w-full bg-raised border border-border rounded-lg px-3 py-2.5 text-sm text-primary placeholder:text-muted focus:outline-none focus:border-fern'
const labelCls = 'block text-xs text-dim mb-1'

const DOSE_UNITS = ['ml', 'oz', 'g', 'tsp', 'tbsp'] as const
const LIQUID_VOLUME_UNITS = ['gal', 'l'] as const
const DRY_VOLUME_UNITS = ['gal', 'l', 'in'] as const

const emptyForm: CreateProductRequest = {
  name: '',
  form: 'liquid',
  npk: { n: 0, p: 0, k: 0 },
  referenceDose: { min: 0, max: 0, unit: 'ml', perVolume: 1, perVolumeUnit: 'gal' },
}

interface Props {
  open: boolean
  onClose: () => void
  product?: Product
}

export function AddProductSheet({ open, onClose, product }: Props) {
  const qc = useQueryClient()
  const isEdit = !!product

  const [form, setForm] = useState<CreateProductRequest>(emptyForm)
  const [isRange, setIsRange] = useState(false)

  useEffect(() => {
    if (open) {
      if (product) {
        setForm({
          name:          product.name,
          brand:         product.brand,
          form:          product.form,
          npk:           product.npk,
          referenceDose: product.referenceDose,
          stockQty:      product.stockQty,
          stockUnit:     product.stockUnit,
          notes:         product.notes,
        })
        setIsRange(product.referenceDose.min !== product.referenceDose.max)
      } else {
        setForm(emptyForm)
        setIsRange(false)
      }
    }
  }, [open, product])

  const mutation = useMutation({
    mutationFn: (body: CreateProductRequest) =>
      isEdit ? api.products.update(product!.productId, body) : api.products.create(body),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['products'] })
      onClose()
    },
  })

  function submit(e: React.FormEvent) {
    e.preventDefault()
    if (!form.name.trim()) return
    const referenceDose = isRange
      ? form.referenceDose
      : { ...form.referenceDose, max: form.referenceDose.min }
    mutation.mutate({ ...form, referenceDose })
  }

  const volumeLabel = form.form === 'dry' ? 'pot/container size' : 'water'

  return (
    <BottomSheet title={isEdit ? 'Edit Product' : 'New Product'} open={open} onClose={onClose}>
      <form onSubmit={submit} className="space-y-3 mt-2">
        <div>
          <label className={labelCls}>Name *</label>
          <input
            className={inputCls}
            placeholder="e.g. GH Flora Bloom"
            value={form.name}
            onChange={e => setForm(f => ({ ...f, name: e.target.value }))}
            required
          />
        </div>

        <div>
          <label className={labelCls}>Brand</label>
          <input
            className={inputCls}
            placeholder="e.g. General Hydroponics"
            value={form.brand ?? ''}
            onChange={e => setForm(f => ({ ...f, brand: e.target.value || undefined }))}
          />
        </div>

        <div>
          <label className={labelCls}>Form</label>
          <select
            className={inputCls}
            value={form.form}
            onChange={e => {
              const nextForm = e.target.value as ProductForm
              setForm(f => ({
                ...f,
                form: nextForm,
                referenceDose: nextForm === 'liquid' && f.referenceDose.perVolumeUnit === 'in'
                  ? { ...f.referenceDose, perVolumeUnit: 'gal' }
                  : f.referenceDose,
              }))
            }}
          >
            <option value="liquid" className="bg-raised">Liquid</option>
            <option value="dry"    className="bg-raised">Dry / powder</option>
          </select>
        </div>

        <div>
          <label className={labelCls}>NPK</label>
          <div className="grid grid-cols-3 gap-2">
            {(['n', 'p', 'k'] as const).map(key => (
              <input
                key={key}
                type="number"
                step="0.1"
                className={inputCls}
                placeholder={key.toUpperCase()}
                value={form.npk[key] || ''}
                onChange={e => setForm(f => ({ ...f, npk: { ...f.npk, [key]: Number(e.target.value) || 0 } }))}
              />
            ))}
          </div>
        </div>

        <div>
          <div className="flex items-center justify-between mb-1">
            <label className={labelCls}>Labeled dose</label>
            <button
              type="button"
              onClick={() => setIsRange(r => !r)}
              className="text-xs text-fern active:opacity-70"
            >
              {isRange ? 'Use single value' : 'Use a range'}
            </button>
          </div>
          <div className="grid grid-cols-2 gap-2 items-end">
            {isRange ? (
              <>
                <div>
                  <label className="block text-[11px] text-muted mb-1">Min</label>
                  <input
                    type="number" step="0.1" className={inputCls} placeholder="1"
                    value={form.referenceDose.min || ''}
                    onChange={e => setForm(f => ({ ...f, referenceDose: { ...f.referenceDose, min: Number(e.target.value) || 0 } }))}
                  />
                </div>
                <div>
                  <label className="block text-[11px] text-muted mb-1">Max</label>
                  <input
                    type="number" step="0.1" className={inputCls} placeholder="2"
                    value={form.referenceDose.max || ''}
                    onChange={e => setForm(f => ({ ...f, referenceDose: { ...f.referenceDose, max: Number(e.target.value) || 0 } }))}
                  />
                </div>
              </>
            ) : (
              <div className="col-span-2">
                <label className="block text-[11px] text-muted mb-1">Amount</label>
                <input
                  type="number" step="0.1" className={inputCls} placeholder="5"
                  value={form.referenceDose.min || ''}
                  onChange={e => setForm(f => ({ ...f, referenceDose: { ...f.referenceDose, min: Number(e.target.value) || 0 } }))}
                />
              </div>
            )}
          </div>
          <div className="grid grid-cols-3 gap-2 mt-2">
            <select
              className={inputCls}
              value={form.referenceDose.unit}
              onChange={e => setForm(f => ({ ...f, referenceDose: { ...f.referenceDose, unit: e.target.value as typeof form.referenceDose.unit } }))}
            >
              {DOSE_UNITS.map(u => <option key={u} value={u} className="bg-raised">{u}</option>)}
            </select>
            <span className="text-xs text-muted self-center text-center">per</span>
            <div className="flex gap-1">
              <input
                type="number" step="0.1" className={inputCls} placeholder="1"
                value={form.referenceDose.perVolume || ''}
                onChange={e => setForm(f => ({ ...f, referenceDose: { ...f.referenceDose, perVolume: Number(e.target.value) || 0 } }))}
              />
              <select
                className={inputCls}
                value={form.referenceDose.perVolumeUnit}
                onChange={e => setForm(f => ({ ...f, referenceDose: { ...f.referenceDose, perVolumeUnit: e.target.value as typeof form.referenceDose.perVolumeUnit } }))}
              >
                {(form.form === 'dry' ? DRY_VOLUME_UNITS : LIQUID_VOLUME_UNITS).map(u => (
                  <option key={u} value={u} className="bg-raised">{u === 'in' ? 'in (pot diameter)' : u}</option>
                ))}
              </select>
            </div>
          </div>
          <p className="text-[11px] text-muted mt-1">
            {form.referenceDose.perVolumeUnit === 'in'
              ? <>e.g. {form.referenceDose.min}{isRange ? `-${form.referenceDose.max}` : ''} {form.referenceDose.unit} per {form.referenceDose.perVolume}in pot diameter</>
              : <>e.g. {form.referenceDose.min}{isRange ? `-${form.referenceDose.max}` : ''} {form.referenceDose.unit} per {form.referenceDose.perVolume} {form.referenceDose.perVolumeUnit} of {volumeLabel}</>}
          </p>
        </div>

        <div className="grid grid-cols-2 gap-3">
          <div>
            <label className={labelCls}>Stock on hand</label>
            <input
              type="number" step="0.1" className={inputCls} placeholder="500"
              value={form.stockQty ?? ''}
              onChange={e => setForm(f => ({ ...f, stockQty: e.target.value ? Number(e.target.value) : undefined }))}
            />
          </div>
          <div>
            <label className={labelCls}>Stock unit</label>
            <input
              className={inputCls} placeholder="ml, bottles, g..."
              value={form.stockUnit ?? ''}
              onChange={e => setForm(f => ({ ...f, stockUnit: e.target.value || undefined }))}
            />
          </div>
        </div>

        <div>
          <label className={labelCls}>Notes</label>
          <textarea
            className={inputCls}
            rows={2}
            placeholder="Optional"
            value={form.notes ?? ''}
            onChange={e => setForm(f => ({ ...f, notes: e.target.value || undefined }))}
          />
        </div>

        {mutation.isError && (
          <p className="text-red-400 text-sm">{(mutation.error as Error).message}</p>
        )}

        <button
          type="submit"
          disabled={mutation.isPending}
          className="w-full py-3 bg-fern text-base font-semibold rounded-xl active:opacity-80 disabled:opacity-50 mt-2"
        >
          {mutation.isPending ? 'Saving…' : isEdit ? 'Save Changes' : 'Add Product'}
        </button>
      </form>
    </BottomSheet>
  )
}
