import type { ReferenceDose, NPK } from '@/types'

const ML_PER_UNIT: Record<string, number> = { ml: 1, l: 1000, oz: 29.5735, gal: 3785.41 }

export function convertVolume(amount: number, from: string, to: string): number {
  if (from === to) return amount
  return amount * ML_PER_UNIT[from] / ML_PER_UNIT[to]
}

// Dose units (as opposed to batch/perVolume units above). "g" is mass, not
// volume -- converting it against ml/oz/tsp/tbsp needs the product's density,
// which we don't have, so that conversion is refused rather than guessed.
const DOSE_ML_PER_UNIT: Record<string, number> = { ml: 1, oz: 29.5735, tsp: 4.92892, tbsp: 14.7868 }

// Units a dose amount can be freely re-entered in, regardless of which unit
// the product's label happened to be entered in.
export const CONVERTIBLE_DOSE_UNITS = Object.keys(DOSE_ML_PER_UNIT)

export function isConvertibleDoseUnit(unit: string): boolean {
  return unit in DOSE_ML_PER_UNIT
}

export function convertDoseAmount(amount: number, from: string, to: string): number | undefined {
  if (from === to) return amount
  if (from === 'g' || to === 'g') return undefined
  if (!(from in DOSE_ML_PER_UNIT) || !(to in DOSE_ML_PER_UNIT)) return undefined
  return amount * DOSE_ML_PER_UNIT[from] / DOSE_ML_PER_UNIT[to]
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
// `unit` is the dose's own unit, which may differ from referenceDose.unit
// (e.g. logged in ml against a label given in oz) -- must be converted to
// the same unit as `full` before dividing, or the ratio is meaningless.
export function pctOfDose(amount: number, unit: string, referenceDose: ReferenceDose, batchAmount: number, batchUnit: string): number | undefined {
  const full = scaledFullDose(referenceDose, batchAmount, batchUnit)
  if (!full) return undefined
  const converted = convertDoseAmount(amount, unit, referenceDose.unit)
  if (converted === undefined) return undefined
  return (converted / full) * 100
}

// Delivered elemental grams -- amount is already the real applied dose (not
// a full-label reference), so this is a straight mass x concentration
// conversion, not scaled by pctOfDose again. Returns undefined when the
// dose's unit can't be converted to the product's own dose unit (e.g. a
// mass/volume mismatch with no density path -- see convertDoseAmount).
export function deliveredGrams(
  amount: number,
  unit: string,
  productDoseUnit: string,
  elementalNpk: NPK,
  density: number,
): { n: number; p: number; k: number } | undefined {
  const converted = convertDoseAmount(amount, unit, productDoseUnit)
  if (converted === undefined) return undefined
  const massGrams = converted * density
  return {
    n: massGrams * (elementalNpk.n / 100),
    p: massGrams * (elementalNpk.p / 100),
    k: massGrams * (elementalNpk.k / 100),
  }
}
