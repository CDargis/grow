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

const MAX_SCALE = 4

function Slide({ item }: { item: PhotoLightboxItem | null }) {
  return (
    <div className="w-screen h-full flex items-center justify-center flex-shrink-0">
      {item && (
        <MediaImage photoKey={item.photoKey} alt="photo" className="max-w-full max-h-full object-contain" />
      )}
    </div>
  )
}

function touchDist(e: React.TouchEvent): number {
  const dx = e.touches[0].clientX - e.touches[1].clientX
  const dy = e.touches[0].clientY - e.touches[1].clientY
  return Math.hypot(dx, dy)
}

function touchMid(e: React.TouchEvent): { x: number; y: number } {
  return {
    x: (e.touches[0].clientX + e.touches[1].clientX) / 2,
    y: (e.touches[0].clientY + e.touches[1].clientY) / 2,
  }
}

export function PhotoLightbox({ items, index, onClose, onNavigate }: Props) {
  const [dragX, setDragX]       = useState(0)
  const [dragging, setDragging] = useState(false)
  // Direction the strip is animating toward after a completed swipe; 0 = at rest
  const [settle, setSettle]     = useState<-1 | 0 | 1>(0)
  // Pinch-zoom state for the current photo; swipe nav only engages at scale 1
  const [zoom, setZoom]         = useState({ scale: 1, tx: 0, ty: 0 })
  const [zooming, setZooming]   = useState(false)

  const touchRef = useRef<{ x: number; y: number; horizontal: boolean | null } | null>(null)
  const pinchRef = useRef<{ dist: number; scale: number; tx: number; ty: number; midX: number; midY: number } | null>(null)
  const panRef   = useRef<{ x: number; y: number; tx: number; ty: number } | null>(null)
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
  const zoomed = zoom.scale > 1

  function resetZoom() {
    setZoom({ scale: 1, tx: 0, ty: 0 })
  }

  function handleTouchStart(e: React.TouchEvent) {
    if (settle !== 0) return
    if (e.touches.length === 2) {
      // Entering a pinch cancels any in-progress swipe
      touchRef.current = null
      setDragX(0)
      setDragging(false)
      movedRef.current = true
      setZooming(true)
      const mid = touchMid(e)
      pinchRef.current = { dist: touchDist(e), scale: zoom.scale, tx: zoom.tx, ty: zoom.ty, midX: mid.x, midY: mid.y }
      return
    }
    if (zoomed) {
      panRef.current = { x: e.touches[0].clientX, y: e.touches[0].clientY, tx: zoom.tx, ty: zoom.ty }
      setZooming(true)
      return
    }
    touchRef.current = { x: e.touches[0].clientX, y: e.touches[0].clientY, horizontal: null }
    movedRef.current = false
    setDragX(0)
    setDragging(true)
  }

  function handleTouchMove(e: React.TouchEvent) {
    const pinch = pinchRef.current
    if (pinch && e.touches.length === 2) {
      const cx = window.innerWidth / 2
      const cy = window.innerHeight / 2
      const mid = touchMid(e)
      const scale = Math.min(MAX_SCALE, Math.max(1, pinch.scale * (touchDist(e) / pinch.dist)))
      // Keep the content point under the pinch midpoint anchored while scaling
      const tx = (mid.x - cx) - (scale / pinch.scale) * (pinch.midX - cx - pinch.tx)
      const ty = (mid.y - cy) - (scale / pinch.scale) * (pinch.midY - cy - pinch.ty)
      setZoom({ scale, tx, ty })
      return
    }
    const pan = panRef.current
    if (pan && zoomed) {
      movedRef.current = true
      setZoom(z => ({ ...z, tx: pan.tx + (e.touches[0].clientX - pan.x), ty: pan.ty + (e.touches[0].clientY - pan.y) }))
      return
    }
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

  function handleTouchEnd(e: React.TouchEvent) {
    if (pinchRef.current) {
      if (e.touches.length < 2) {
        pinchRef.current = null
        setZooming(false)
        // Snap back to fit if the pinch ended near or below 1x
        if (zoom.scale <= 1.05) resetZoom()
      }
      return
    }
    if (panRef.current) {
      panRef.current = null
      setZooming(false)
      return
    }
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
      resetZoom()
    }
  }

  function handleBackdropClick() {
    if (movedRef.current) { movedRef.current = false; return }
    if (zoomed) { resetZoom(); return }
    onClose()
  }

  function navigateTo(dir: -1 | 1) {
    if (settle !== 0) return
    resetZoom()
    setSettle(dir)
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
        <div
          className="w-screen h-full flex-shrink-0"
          style={{
            transform: `translate(${zoom.tx}px, ${zoom.ty}px) scale(${zoom.scale})`,
            transition: zooming ? 'none' : 'transform 0.2s ease-out',
          }}
        >
          <Slide item={item} />
        </div>
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

      {!zoomed && prev && (
        <button
          onClick={e => { e.stopPropagation(); navigateTo(-1) }}
          className="absolute left-2 top-1/2 -translate-y-1/2 w-9 h-9 rounded-full bg-white/10 flex items-center justify-center text-white/80 active:opacity-70"
        >
          <ChevronLeft size={20} />
        </button>
      )}
      {!zoomed && next && (
        <button
          onClick={e => { e.stopPropagation(); navigateTo(1) }}
          className="absolute right-2 top-1/2 -translate-y-1/2 w-9 h-9 rounded-full bg-white/10 flex items-center justify-center text-white/80 active:opacity-70"
        >
          <ChevronRight size={20} />
        </button>
      )}

      {!zoomed && (
        <div className="absolute bottom-0 left-0 right-0 pointer-events-none bg-gradient-to-t from-black/80 to-transparent px-4 pb-7 pt-12">
          <p className="text-white text-sm font-medium">{item.title}</p>
          {item.subtitle && (
            <p className="text-xs capitalize mt-0.5" style={{ color: item.accent ?? 'rgba(255,255,255,0.7)' }}>
              {item.subtitle}
            </p>
          )}
        </div>
      )}
    </div>
  )
}
