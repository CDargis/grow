import { useEffect, useRef, useState } from 'react'
import { X, ChevronLeft, ChevronRight } from 'lucide-react'
import { MediaImage } from './MediaImage'

export interface PhotoLightboxItem {
  photoKey: string
  title: string
  subtitle?: string
  accent?: string
}

interface Props {
  items: PhotoLightboxItem[]
  index: number | null
  onClose: () => void
  onNavigate: (index: number) => void
}

function Slide({ item }: { item: PhotoLightboxItem | null }) {
  return (
    <div className="w-screen h-full flex items-center justify-center flex-shrink-0">
      {item && (
        <MediaImage photoKey={item.photoKey} alt="photo" className="max-w-full max-h-full object-contain" />
      )}
    </div>
  )
}

export function PhotoLightbox({ items, index, onClose, onNavigate }: Props) {
  const [dragX, setDragX]       = useState(0)
  const [dragging, setDragging] = useState(false)
  // Direction the strip is animating toward after a completed swipe; 0 = at rest
  const [settle, setSettle]     = useState<-1 | 0 | 1>(0)
  const touchRef = useRef<{ x: number; y: number; horizontal: boolean | null } | null>(null)
  const movedRef = useRef(false)

  const open = index !== null && index >= 0 && index < items.length

  useEffect(() => {
    if (!open) return
    function onKey(e: KeyboardEvent) {
      if (e.key === 'Escape') onClose()
      if (e.key === 'ArrowLeft'  && index! > 0)                onNavigate(index! - 1)
      if (e.key === 'ArrowRight' && index! < items.length - 1) onNavigate(index! + 1)
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [open, index, items.length, onClose, onNavigate])

  if (!open) return null

  const i    = index!
  const item = items[i]
  const prev = i > 0 ? items[i - 1] : null
  const next = i < items.length - 1 ? items[i + 1] : null

  function handleTouchStart(e: React.TouchEvent) {
    if (settle !== 0) return
    touchRef.current = { x: e.touches[0].clientX, y: e.touches[0].clientY, horizontal: null }
    movedRef.current = false
    setDragX(0)
    setDragging(true)
  }

  function handleTouchMove(e: React.TouchEvent) {
    const t = touchRef.current
    if (!t) return
    const dx = e.touches[0].clientX - t.x
    const dy = e.touches[0].clientY - t.y
    if (t.horizontal === null && (Math.abs(dx) > 8 || Math.abs(dy) > 8)) {
      t.horizontal = Math.abs(dx) > Math.abs(dy)
    }
    if (t.horizontal) {
      movedRef.current = true
      // Rubber-band when swiping past either end of the set
      const limited = (dx > 0 && !prev) || (dx < 0 && !next) ? dx / 3 : dx
      setDragX(limited)
    }
  }

  function handleTouchEnd() {
    const t = touchRef.current
    touchRef.current = null
    setDragging(false)
    if (t?.horizontal) {
      if (dragX < -60 && next)      { setSettle(1);  return }
      else if (dragX > 60 && prev)  { setSettle(-1); return }
    }
    setDragX(0)
  }

  function handleTransitionEnd() {
    if (settle !== 0) {
      onNavigate(i + settle)
      setSettle(0)
      setDragX(0)
    }
  }

  function handleBackdropClick() {
    if (movedRef.current) { movedRef.current = false; return }
    onClose()
  }

  const transform = settle === 1
    ? 'translateX(-200vw)'
    : settle === -1
      ? 'translateX(0vw)'
      : `translateX(calc(-100vw + ${dragX}px))`

  return (
    <div
      className="fixed inset-0 z-50 bg-black overflow-hidden"
      style={{ touchAction: 'none' }}
      onClick={handleBackdropClick}
      onTouchStart={handleTouchStart}
      onTouchMove={handleTouchMove}
      onTouchEnd={handleTouchEnd}
    >
      {/* Sliding strip: prev / current / next. Keyed by index so it remounts
          centered (no transition) after a settle animation commits the move. */}
      <div
        key={i}
        className="flex h-full"
        style={{
          width: '300vw',
          transform,
          transition: dragging ? 'none' : 'transform 0.25s ease-out',
        }}
        onTransitionEnd={handleTransitionEnd}
      >
        <Slide item={prev} />
        <Slide item={item} />
        <Slide item={next} />
      </div>

      <span className="absolute top-6 left-4 text-xs text-white/70 font-medium pointer-events-none">
        {i + 1} / {items.length}
      </span>

      <button
        onClick={e => { e.stopPropagation(); onClose() }}
        className="absolute top-4 right-4 w-9 h-9 rounded-full bg-white/10 flex items-center justify-center text-white active:opacity-70"
      >
        <X size={20} />
      </button>

      {prev && (
        <button
          onClick={e => { e.stopPropagation(); if (settle === 0) setSettle(-1) }}
          className="absolute left-2 top-1/2 -translate-y-1/2 w-9 h-9 rounded-full bg-white/10 flex items-center justify-center text-white/80 active:opacity-70"
        >
          <ChevronLeft size={20} />
        </button>
      )}
      {next && (
        <button
          onClick={e => { e.stopPropagation(); if (settle === 0) setSettle(1) }}
          className="absolute right-2 top-1/2 -translate-y-1/2 w-9 h-9 rounded-full bg-white/10 flex items-center justify-center text-white/80 active:opacity-70"
        >
          <ChevronRight size={20} />
        </button>
      )}

      <div className="absolute bottom-0 left-0 right-0 pointer-events-none bg-gradient-to-t from-black/80 to-transparent px-4 pb-7 pt-12">
        <p className="text-white text-sm font-medium">{item.title}</p>
        {item.subtitle && (
          <p className="text-xs capitalize mt-0.5" style={{ color: item.accent ?? 'rgba(255,255,255,0.7)' }}>
            {item.subtitle}
          </p>
        )}
      </div>
    </div>
  )
}
