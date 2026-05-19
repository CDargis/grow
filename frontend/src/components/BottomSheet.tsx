import { useEffect, useRef } from 'react'
import { X, ArrowLeft } from 'lucide-react'

interface BottomSheetProps {
  title: string
  open: boolean
  onClose: () => void
  onBack?: () => void
  children: React.ReactNode
}

export function BottomSheet({ title, open, onClose, onBack, children }: BottomSheetProps) {
  const ref = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (!open) return
    function onKey(e: KeyboardEvent) { if (e.key === 'Escape') onClose() }
    document.addEventListener('keydown', onKey)
    return () => document.removeEventListener('keydown', onKey)
  }, [open, onClose])

  if (!open) return null

  return (
    <div className="fixed inset-0 z-50 flex flex-col justify-end">
      <div className="absolute inset-0 bg-black/60" onClick={onClose} />
      <div
        ref={ref}
        className="relative bg-surface rounded-t-2xl w-full max-h-[90dvh] overflow-y-auto"
      >
        <div className="flex items-center justify-between px-4 pt-4 pb-2">
          <div className="flex items-center gap-2">
            {onBack && (
              <button onClick={onBack} className="p-1 text-muted active:opacity-70 -ml-1">
                <ArrowLeft size={20} />
              </button>
            )}
            <h2 className="text-base font-semibold">{title}</h2>
          </div>
          <button onClick={onClose} className="p-1 text-muted active:opacity-70">
            <X size={20} />
          </button>
        </div>
        <div className="px-4 pb-8">
          {children}
        </div>
      </div>
    </div>
  )
}
