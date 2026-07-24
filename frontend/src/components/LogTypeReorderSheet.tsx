import { useEffect, useState } from 'react'
import {
  DndContext, closestCenter, PointerSensor, useSensor, useSensors,
  type DragEndEvent,
} from '@dnd-kit/core'
import {
  SortableContext, useSortable, verticalListSortingStrategy, arrayMove,
} from '@dnd-kit/sortable'
import { CSS } from '@dnd-kit/utilities'
import { GripVertical } from 'lucide-react'
import { BottomSheet } from './BottomSheet'
import type { LogType } from '@/types'

export interface LogTypeCatalogItem {
  type: LogType
  label: string
  icon: React.ReactNode
}

interface LogTypeReorderSheetProps {
  open: boolean
  onClose: () => void
  title: string
  catalog: LogTypeCatalogItem[]
  value: LogType[]
  mode: 'divider' | 'plain'
  min?: number
  max?: number
  onSave: (order: LogType[]) => void
}

function buildInitialItems(catalog: LogTypeCatalogItem[], value: LogType[]): LogType[] {
  const rest = catalog.map(c => c.type).filter(t => !value.includes(t))
  return [...value, ...rest]
}

function SortableRow({ id, children }: { id: string; children: React.ReactNode }) {
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({ id })
  const style: React.CSSProperties = {
    transform: CSS.Transform.toString(transform),
    transition,
    opacity: isDragging ? 0.5 : 1,
    touchAction: 'none',
  }
  return (
    <div ref={setNodeRef} style={style} {...attributes} {...listeners} className="flex items-center gap-3 px-3 py-2.5 bg-raised rounded-xl active:opacity-80">
      <GripVertical size={16} className="text-muted flex-shrink-0" />
      {children}
    </div>
  )
}

export function LogTypeReorderSheet({ open, onClose, title, catalog, value, mode, min = 1, max = catalog.length, onSave }: LogTypeReorderSheetProps) {
  const [items, setItems]           = useState<LogType[]>(() => buildInitialItems(catalog, value))
  const [activeCount, setActiveCount] = useState(value.length)

  useEffect(() => {
    if (!open) return
    setItems(buildInitialItems(catalog, value))
    setActiveCount(value.length)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open])

  const sensors = useSensors(useSensor(PointerSensor, { activationConstraint: { delay: 200, tolerance: 8 } }))
  const catalogMap = new Map(catalog.map(c => [c.type, c]))

  function handleDragEnd(e: DragEndEvent) {
    const { active, over } = e
    if (!over || active.id === over.id) return
    const oldIndex = items.indexOf(active.id as LogType)
    const newIndex = items.indexOf(over.id as LogType)
    if (oldIndex < 0 || newIndex < 0) return

    if (mode === 'plain') {
      setItems(arrayMove(items, oldIndex, newIndex))
      return
    }

    const wasActive = oldIndex < activeCount
    let nextActiveCount = activeCount
    if (wasActive && newIndex >= activeCount) {
      if (activeCount - 1 < min) return
      nextActiveCount = activeCount - 1
    } else if (!wasActive && newIndex < activeCount) {
      if (activeCount + 1 > max) return
      nextActiveCount = activeCount + 1
    }
    setItems(arrayMove(items, oldIndex, newIndex))
    setActiveCount(nextActiveCount)
  }

  function handleSave() {
    onSave(mode === 'divider' ? items.slice(0, activeCount) : items)
    onClose()
  }

  return (
    <BottomSheet title={title} open={open} onClose={onClose}>
      <p className="text-xs text-muted mb-3">
        {mode === 'divider'
          ? `Hold and drag to reorder. Drag above or below the line to change which ${min}–${max} are shown.`
          : 'Hold and drag to reorder.'}
      </p>
      <DndContext sensors={sensors} collisionDetection={closestCenter} onDragEnd={handleDragEnd}>
        <SortableContext items={items} strategy={verticalListSortingStrategy}>
          <div className="flex flex-col gap-2">
            {items.map((type, idx) => {
              const item = catalogMap.get(type)
              if (!item) return null
              return (
                <div key={type}>
                  {mode === 'divider' && idx === activeCount && (
                    <div className="flex items-center gap-2 my-1 px-1">
                      <div className="flex-1 border-t border-dashed border-border" />
                      <span className="text-[10px] text-muted uppercase tracking-wide">Hidden</span>
                      <div className="flex-1 border-t border-dashed border-border" />
                    </div>
                  )}
                  <SortableRow id={type}>
                    <span className={idx < activeCount || mode === 'plain' ? 'text-dim' : 'text-muted/50'}>{item.icon}</span>
                    <span className={`text-sm ${idx < activeCount || mode === 'plain' ? 'text-primary' : 'text-muted'}`}>{item.label}</span>
                  </SortableRow>
                </div>
              )
            })}
          </div>
        </SortableContext>
      </DndContext>
      <button
        onClick={handleSave}
        className="w-full mt-4 py-3 rounded-xl bg-fern text-white font-medium text-sm active:opacity-80"
      >
        Save
      </button>
    </BottomSheet>
  )
}
