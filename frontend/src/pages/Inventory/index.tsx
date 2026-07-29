import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Plus, Pencil, Trash2, Droplet, Wheat } from 'lucide-react'
import { api } from '@/api/client'
import { AddProductSheet } from '@/components/AddProductSheet'
import type { Product } from '@/types'

function doseText(product: Product) {
  const { min, max, unit, perVolume, perVolumeUnit } = product.referenceDose
  const amount = min === max ? `${min}` : `${min}-${max}`
  const basis = product.form === 'dry' ? 'pot' : perVolumeUnit
  return `${amount} ${unit} / ${perVolume} ${basis}`
}

function ProductCard({ product, onEdit, onDelete }: { product: Product; onEdit: (p: Product) => void; onDelete: (p: Product) => void }) {
  const { n, p, k } = product.npk
  return (
    <div className="flex items-center gap-3 p-4 bg-surface rounded-xl border border-border">
      <div className="w-10 h-10 rounded-full bg-raised border border-border flex items-center justify-center flex-shrink-0 text-muted">
        {product.form === 'dry' ? <Wheat size={18} /> : <Droplet size={18} />}
      </div>
      <div className="flex-1 min-w-0">
        <p className="font-semibold text-primary truncate">{product.name}</p>
        <p className="text-xs text-dim truncate">
          {product.brand ? `${product.brand} · ` : ''}NPK {n}-{p}-{k}
        </p>
        <p className="text-xs text-muted truncate">{doseText(product)}</p>
        {(product.stockQty ?? 0) > 0 && (
          <p className="text-xs text-fern mt-0.5">{product.stockQty} {product.stockUnit} on hand</p>
        )}
      </div>
      <div className="flex flex-col gap-2 flex-shrink-0">
        <button onClick={() => onEdit(product)} className="w-7 h-7 flex items-center justify-center text-muted active:opacity-60">
          <Pencil size={14} />
        </button>
        <button onClick={() => onDelete(product)} className="w-7 h-7 flex items-center justify-center text-red-400 active:opacity-60">
          <Trash2 size={14} />
        </button>
      </div>
    </div>
  )
}

export function InventoryPage() {
  const qc = useQueryClient()
  const [editing, setEditing] = useState<Product | null>(null)
  const [addOpen, setAddOpen] = useState(false)

  const { data: products, isLoading } = useQuery({
    queryKey: ['products'],
    queryFn: api.products.list,
  })

  const deleteMutation = useMutation({
    mutationFn: (id: string) => api.products.delete(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['products'] }),
  })

  function onDelete(product: Product) {
    if (confirm(`Delete ${product.name}?`)) deleteMutation.mutate(product.productId)
  }

  return (
    <div className="p-4">
      <div className="flex items-center justify-between mb-4">
        <h1 className="text-lg font-semibold">Inventory</h1>
        <button
          onClick={() => setAddOpen(true)}
          className="flex items-center gap-1 text-sm text-fern active:opacity-70"
        >
          <Plus size={18} />
          Add
        </button>
      </div>

      {isLoading && <div className="text-muted text-center py-16">Loading…</div>}

      {!isLoading && (products?.length ?? 0) === 0 && (
        <div className="text-center text-muted py-16">No products yet.</div>
      )}

      <div className="space-y-3">
        {products?.map(product => (
          <ProductCard key={product.productId} product={product} onEdit={setEditing} onDelete={onDelete} />
        ))}
      </div>

      <AddProductSheet open={addOpen} onClose={() => setAddOpen(false)} />
      <AddProductSheet open={!!editing} onClose={() => setEditing(null)} product={editing ?? undefined} />
    </div>
  )
}
