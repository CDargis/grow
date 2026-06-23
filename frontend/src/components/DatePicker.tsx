import { useState, useRef } from 'react'
import { ChevronLeft, ChevronRight, CalendarDays, ChevronsUpDown } from 'lucide-react'

const MONTH_NAMES = [
  'January','February','March','April','May','June',
  'July','August','September','October','November','December',
]
const DAY_LABELS = ['M','T','W','T','F','S','S']

function toYMD(d: Date) {
  return d.toISOString().slice(0, 10)
}

function addDays(d: Date, n: number): Date {
  const r = new Date(d)
  r.setDate(r.getDate() + n)
  return r
}

// Returns the Monday of the week containing d
function weekStart(d: Date): Date {
  const r = new Date(d)
  r.setHours(0, 0, 0, 0)
  const dow = r.getDay() // 0=Sun
  r.setDate(r.getDate() - (dow === 0 ? 6 : dow - 1))
  return r
}

interface DatePickerProps {
  activeDates: Set<string>
  selected: string | null
  onSelect: (date: string | null) => void
  weekRef?: Date
  onWeekRefChange?: (d: Date) => void
}

export function DatePicker({ activeDates, selected, onSelect, weekRef: externalWeekRef, onWeekRefChange }: DatePickerProps) {
  const [expanded, setExpanded] = useState(false)
  const todayStr = toYMD(new Date())

  const [internalRef, setInternalRef] = useState(new Date())
  const ref = externalWeekRef ?? internalRef
  const touchStartX = useRef<number | null>(null)

  function setRef(updater: Date | ((prev: Date) => Date)) {
    const next = typeof updater === 'function' ? updater(ref) : updater
    if (onWeekRefChange) {
      onWeekRefChange(next)
    } else {
      setInternalRef(next)
    }
  }

  function toggle(dateStr: string) {
    onSelect(selected === dateStr ? null : dateStr)
  }

  function handleTouchStart(e: React.TouchEvent) {
    touchStartX.current = e.touches[0].clientX
  }

  function handleTouchEnd(e: React.TouchEvent) {
    if (touchStartX.current === null) return
    const dx = e.changedTouches[0].clientX - touchStartX.current
    touchStartX.current = null
    if (Math.abs(dx) < 40) return
    setRef(d => addDays(d, dx < 0 ? 7 : -7))
  }

  if (!expanded) {
    const monday = weekStart(ref)
    const week = Array.from({ length: 7 }, (_, i) => addDays(monday, i))

    return (
      <div className="bg-surface border-b border-border">
        <div
          className="flex items-center px-2 py-2"
          onTouchStart={handleTouchStart}
          onTouchEnd={handleTouchEnd}
        >
          <button
            onClick={() => setRef(d => addDays(d, -7))}
            className="p-1.5 text-muted active:opacity-60"
          >
            <ChevronLeft size={16} />
          </button>
          <div className="flex flex-1 justify-around">
            {week.map((day, i) => {
              const str = toYMD(day)
              const isSelected = str === selected
              const isToday = str === todayStr
              const hasActivity = activeDates.has(str)
              return (
                <button key={str} onClick={() => toggle(str)} className="flex flex-col items-center gap-0.5 py-0.5 px-0.5">
                  <span className="text-[10px] text-muted leading-none">{DAY_LABELS[i]}</span>
                  <span className={`w-7 h-7 flex items-center justify-center rounded-full text-sm font-medium ${
                    isSelected ? 'bg-fern text-base font-semibold' :
                    isToday    ? 'text-lime' : 'text-primary'
                  }`}>
                    {day.getDate()}
                  </span>
                  <span className={`w-1.5 h-1.5 rounded-full ${
                    hasActivity ? (isSelected ? 'bg-base' : 'bg-fern') : 'bg-transparent'
                  }`} />
                </button>
              )
            })}
          </div>
          <button
            onClick={() => setRef(d => addDays(d, 7))}
            className="p-1.5 text-muted active:opacity-60"
          >
            <ChevronRight size={16} />
          </button>
          <button
            onClick={() => setExpanded(true)}
            className="p-1.5 text-muted active:opacity-60 ml-0.5"
            title="Expand calendar"
          >
            <CalendarDays size={15} />
          </button>
        </div>
      </div>
    )
  }

  // ── Month view ──────────────────────────────────────────────────────────────

  const year  = ref.getFullYear()
  const month = ref.getMonth()

  const firstOfMonth = new Date(year, month, 1)
  const lastOfMonth  = new Date(year, month + 1, 0)
  const gridStart    = weekStart(firstOfMonth)
  const lastDow      = lastOfMonth.getDay()
  const gridEnd      = addDays(lastOfMonth, lastDow === 0 ? 0 : 7 - lastDow)

  const calDays: Date[] = []
  let cur = new Date(gridStart)
  while (cur <= gridEnd) {
    calDays.push(new Date(cur))
    cur = addDays(cur, 1)
  }

  return (
    <div className="bg-surface border-b border-border px-3 pt-2 pb-3">
      <div className="flex items-center justify-between mb-2">
        <button
          onClick={() => setRef(new Date(year, month - 1, 1))}
          className="p-1 text-muted active:opacity-60"
        >
          <ChevronLeft size={16} />
        </button>
        <span className="text-sm font-medium">{MONTH_NAMES[month]} {year}</span>
        <button
          onClick={() => setRef(new Date(year, month + 1, 1))}
          className="p-1 text-muted active:opacity-60"
        >
          <ChevronRight size={16} />
        </button>
        <button
          onClick={() => setExpanded(false)}
          className="p-1 text-muted active:opacity-60 ml-2"
          title="Collapse"
        >
          <ChevronsUpDown size={15} />
        </button>
      </div>

      <div className="grid grid-cols-7 mb-1">
        {DAY_LABELS.map((d, i) => (
          <div key={i} className="text-center text-[10px] text-muted py-0.5">{d}</div>
        ))}
      </div>

      <div className="grid grid-cols-7">
        {calDays.map(day => {
          const str = toYMD(day)
          const inMonth    = day.getMonth() === month
          const isSelected = str === selected
          const isToday    = str === todayStr
          const hasActivity = activeDates.has(str) && inMonth
          return (
            <button
              key={str}
              onClick={() => toggle(str)}
              className="flex flex-col items-center py-0.5"
            >
              <span className={`w-7 h-7 flex items-center justify-center rounded-full text-xs font-medium ${
                isSelected ? 'bg-fern text-base font-semibold' :
                isToday    ? 'text-lime' :
                inMonth    ? 'text-primary' : 'text-muted/30'
              }`}>
                {day.getDate()}
              </span>
              <span className={`w-1.5 h-1.5 rounded-full ${
                hasActivity ? (isSelected ? 'bg-base' : 'bg-fern') : 'bg-transparent'
              }`} />
            </button>
          )
        })}
      </div>
    </div>
  )
}
