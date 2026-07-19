import type { Log, WateringData } from '@/types'

export type WateringKind = 'water' | 'feed' | 'topdress'

// A watering with nutrients is a feed; nutrients with no water volume is a
// top-dress. The stored logType is 'watering' for all three — labels derive.
export function wateringKind(data: WateringData): WateringKind {
  const hasNutrients = (data.nutrients?.length ?? 0) > 0
  if (!hasNutrients) return 'water'
  return data.amount != null ? 'feed' : 'topdress'
}

export function isFeedLog(log: Log): boolean {
  return log.logType === 'watering' && wateringKind(log.data as WateringData) !== 'water'
}

export function logTypeLabel(log: Log): string {
  if (log.logType === 'watering') {
    const kind = wateringKind(log.data as WateringData)
    return kind === 'water' ? 'watering' : kind === 'feed' ? 'feeding' : 'top dress'
  }
  return log.logType.replace(/_/g, ' ')
}
