import { useQuery } from '@tanstack/react-query'
import { useSearchParams } from 'react-router-dom'
import { api } from '@/api/client'

function getWeekDates(): Date[] {
  const today = new Date()
  const day = today.getDay()
  const monday = new Date(today)
  monday.setDate(today.getDate() - ((day + 6) % 7))
  return Array.from({ length: 7 }, (_, i) => {
    const d = new Date(monday)
    d.setDate(monday.getDate() + i)
    return d
  })
}

function toISO(d: Date): string {
  return d.toISOString().slice(0, 10)
}

const DAY_LABELS = ['M', 'T', 'W', 'T', 'F', 'S', 'S']

export function DateStrip() {
  const [params, setParams] = useSearchParams()
  const todayISO = toISO(new Date())
  const selectedDate = params.get('date') ?? todayISO
  const week = getWeekDates()

  const { data: logs } = useQuery({
    queryKey: ['logs', selectedDate],
    queryFn: () => api.logs.listForDate(selectedDate),
  })

  // Track which days in the week have any activity
  const activeDays = new Set<string>()
  if (logs) {
    logs.forEach(l => activeDays.add(l.date))
  }

  function selectDate(iso: string) {
    setParams(prev => {
      const next = new URLSearchParams(prev)
      if (iso === todayISO) {
        next.delete('date')
      } else {
        next.set('date', iso)
      }
      return next
    })
  }

  return (
    <div className="flex items-center justify-between px-4 pt-safe bg-surface border-b border-border">
      <span className="text-xs font-semibold tracking-widest text-fern uppercase select-none">
        grow
      </span>

      <div className="flex gap-1">
        {week.map((d, i) => {
          const iso = toISO(d)
          const isToday    = iso === todayISO
          const isSelected = iso === selectedDate
          const hasLogs    = activeDays.has(iso)

          return (
            <button
              key={iso}
              onClick={() => selectDate(iso)}
              className={`
                flex flex-col items-center gap-0.5 w-9 py-2 rounded-lg text-xs
                transition-colors
                ${isSelected
                  ? 'bg-pine text-primary'
                  : isToday
                  ? 'text-lime'
                  : 'text-dim hover:text-primary'}
              `}
            >
              <span className="font-medium">{DAY_LABELS[i]}</span>
              <span className={`font-semibold ${isToday && !isSelected ? 'text-lime' : ''}`}>
                {d.getDate()}
              </span>
              <span className={`h-1 w-1 rounded-full ${hasLogs ? 'bg-fern' : 'bg-transparent'}`} />
            </button>
          )
        })}
      </div>

      <div className="w-10" /> {/* spacer to center date row */}
    </div>
  )
}
