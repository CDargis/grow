import { useState, useRef, useEffect } from 'react'
import { useQuery } from '@tanstack/react-query'
import { api } from '@/api/client'
import type { Product } from '@/types'

interface Props {
  value: string
  onChange: (name: string) => void
  onSelectProduct: (product: Product | null) => void
  placeholder?: string
  inputClassName: string
  wrapperClassName?: string
}

// Search/dropdown over Inventory products, shared by both manual nutrient
// entry and the feed wizard. Picking a suggestion links the row to a
// product (unlocks NPK/% dose); free-typing keeps the row a plain string,
// same as before this feature existed.
export function NutrientAutocomplete({
  value, onChange, onSelectProduct, placeholder, inputClassName, wrapperClassName = 'relative flex-1',
}: Props) {
  const { data: products = [] } = useQuery({ queryKey: ['products'], queryFn: api.products.list })
  const [open, setOpen] = useState(false)
  const containerRef = useRef<HTMLDivElement>(null)

  const query = value.trim().toLowerCase()
  const matches = (query ? products.filter(p => p.name.toLowerCase().includes(query)) : products).slice(0, 6)

  useEffect(() => {
    function onDocClick(e: MouseEvent) {
      if (containerRef.current && !containerRef.current.contains(e.target as Node)) setOpen(false)
    }
    document.addEventListener('mousedown', onDocClick)
    return () => document.removeEventListener('mousedown', onDocClick)
  }, [])

  return (
    <div ref={containerRef} className={wrapperClassName}>
      <input
        className={inputClassName}
        placeholder={placeholder}
        value={value}
        onFocus={() => setOpen(true)}
        onChange={e => { onChange(e.target.value); onSelectProduct(null); setOpen(true) }}
      />
      {open && matches.length > 0 && (
        <div className="absolute z-10 top-full left-0 right-0 mt-1 bg-raised border border-border rounded-lg overflow-hidden shadow-lg max-h-48 overflow-y-auto">
          {matches.map(p => (
            <button
              key={p.productId}
              type="button"
              onClick={() => { onChange(p.name); onSelectProduct(p); setOpen(false) }}
              className="w-full text-left px-3 py-2 text-sm text-primary active:bg-surface hover:bg-surface truncate"
            >
              {p.name}
              <span className="text-xs text-muted ml-2">NPK {p.npk.n}-{p.npk.p}-{p.npk.k}</span>
            </button>
          ))}
        </div>
      )}
    </div>
  )
}
