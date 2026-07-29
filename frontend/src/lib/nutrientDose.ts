import type { ReferenceDose } from '@/types'

const ML_PER_UNIT: Record<string, number> = { ml: 1, l: 1000, oz: 29.5735, gal: 3785.41 }

export function convertVolume(amount: number, from: string, to: string): number {
  if (from === to) return amount
  return amount * ML_PER_UNIT[from] / ML_PER_UNIT[to]
}

// "Full strength" is always the reference dose's max (== its only value for a
// fixed, non-range dose), scaled from the label's perVolume basis to the
// batch amount actually being mixed.
export function scaledFullDose(referenceDose: ReferenceDose, batchAmount: number, batchUnit: string): number {
  if (!batchAmount || !referenceDose.perVolume) return 0
  const batchInProductUnits = convertVolume(batchAmount, batchUnit, referenceDose.perVolumeUnit)
  const scale = batchInProductUnits / referenceDose.perVolume
  return referenceDose.max * scale
}

// Uncapped -- can exceed 100% (pushing past label rate) or fall under it
// (e.g. dosing below a range's own labeled minimum), both real use cases.
export function pctOfDose(amount: number, referenceDose: ReferenceDose, batchAmount: number, batchUnit: string): number | undefined {
  const full = scaledFullDose(referenceDose, batchAmount, batchUnit)
  if (!full) return undefined
  return (amount / full) * 100
}
