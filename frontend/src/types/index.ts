export type PlantPhase =
  | 'germination' | 'seedling' | 'veg' | 'flower'
  | 'harvest' | 'drying' | 'curing' | 'archived' | 'dead'

export type PlantType = 'autoflower' | 'photoperiod' | 'unknown'

export type EnvironmentType =
  | 'tent' | 'outdoor' | 'garage' | 'basement'
  | 'room' | 'greenhouse' | 'other'

export type LogType =
  | 'watering' | 'feeding' | 'training' | 'trimming' | 'sprout'
  | 'transplant' | 'height' | 'note' | 'photo' | 'phase_change'
  | 'environment_change' | 'lighting_change' | 'vpd_change'

export interface Plant {
  plantId: string
  userId: string
  name: string
  strain: string
  genetics?: string
  seedBank?: string
  plantType?: PlantType
  phase: PlantPhase
  phaseStartDate: string
  avatarKey?: string
  environmentId?: string
  archivedAt?: string
  createdAt: string
}

export interface Environment {
  environmentId: string
  userId: string
  name: string
  type: EnvironmentType
  lightSchedule?: string
  targetTempF?: number
  targetHumidity?: number
  targetVpd?: number
  photoKey?: string
  createdAt: string
}

export interface Log {
  plantId: string
  logId: string
  userId: string
  logType: LogType
  date: string
  loggedAt: string
  data: LogData
}

export interface Nutrient {
  name: string
  amount: number
  unit: string
  // Snapshotted at log-creation time from the linked Product -- not
  // recomputed later, so editing/deleting the product doesn't rewrite history.
  productId?: string
  npk?: NPK
  pctOfDose?: number
}

// Unified water/feed/top-dress entry. Nutrients present = feed; nutrients with
// no amount = dry top-dress. See lib/logDisplay.ts for the derived labels.
export interface WateringData {
  amount?: number
  unit: 'ml' | 'l' | 'oz' | 'gal'
  ph?: number
  runoff?: number
  tds?: number
  runoffTds?: number
  nutrients?: Nutrient[]
  note?: string
}

// Legacy — replaced by WateringData.nutrients; kept so pre-migration 'feeding'
// rows still render. No new logs of this shape are written.
export interface FeedingData   { nutrients: Nutrient[]; ph?: number; totalVol?: number }
export interface TrainingData  { method: string; notes?: string; photoKey?: string }
export interface TrimmingData  { method?: string; notes?: string; photoKey?: string }
export interface SproutData    { }
export interface TransplantData { potSize: string; medium?: string }
export interface HeightData    { height: number; unit: 'cm' | 'in' }
export interface NoteData      { text: string }
export interface PhotoData     { photoKeys: string[]; caption?: string }
export interface PhaseChangeData       { fromPhase: PlantPhase; toPhase: PlantPhase }
export interface EnvironmentChangeData { fromEnvironmentId?: string; toEnvironmentId?: string }
export interface LightingChangeData    { fromSchedule?: string; toSchedule?: string }
export interface VpdChangeData         { fromVpd?: number; toVpd?: number }

export type LogData =
  | WateringData | FeedingData | TrainingData | TrimmingData | SproutData
  | TransplantData | HeightData | NoteData | PhotoData | PhaseChangeData
  | EnvironmentChangeData | LightingChangeData

export interface UpdatePlantDetailsRequest {
  name: string
  strain: string
  genetics?: string
  seedBank?: string
  plantType?: PlantType
}

export interface CreatePlantRequest {
  name: string
  strain: string
  genetics?: string
  seedBank?: string
  plantType?: PlantType
  phase: PlantPhase
  environmentId?: string
}

export interface CreateLogRequest {
  logType: LogType
  date?: string
  loggedAt?: string
  data: LogData
}

export interface LastActivity {
  plantId: string
  logType: LogType
  date: string
  loggedAt: string
}

export type PlantsLayoutMode = 'grid' | 'rows' | 'fixed'
export type NutrientEntryMode = 'wizard' | 'manual'

export interface Settings {
  userId: string
  shortcutLogTypes?: LogType[]
  sortChipOrder?: LogType[]
  plantsLayoutMode?: PlantsLayoutMode
  nutrientEntryMode?: NutrientEntryMode
}

export interface UpdateSettingsRequest {
  shortcutLogTypes?: LogType[]
  sortChipOrder?: LogType[]
  plantsLayoutMode?: PlantsLayoutMode
  nutrientEntryMode?: NutrientEntryMode
}

// ── Inventory / Products ──────────────────────────────────────────────────────

export type ProductForm = 'liquid' | 'dry'

export interface NPK { n: number; p: number; k: number }

// Label dosing instruction: Amount (min-max) of the product per perVolume of
// water (liquid) or container/pot size (dry). min === max for a single
// labeled dose rather than a range.
export interface ReferenceDose {
  min: number
  max: number
  unit: 'ml' | 'oz' | 'g' | 'tsp' | 'tbsp'
  perVolume: number
  perVolumeUnit: 'gal' | 'l'
}

export interface Product {
  productId: string
  userId: string
  name: string
  brand?: string
  form: ProductForm
  npk: NPK
  referenceDose: ReferenceDose
  stockQty?: number
  stockUnit?: string
  notes?: string
  createdAt: string
}

export interface CreateProductRequest {
  name: string
  brand?: string
  form: ProductForm
  npk: NPK
  referenceDose: ReferenceDose
  stockQty?: number
  stockUnit?: string
  notes?: string
}

export interface CreateEnvironmentRequest {
  name: string
  type: EnvironmentType
  lightSchedule?: string
  targetTempF?: number
  targetHumidity?: number
  targetVpd?: number
  photoKey?: string
}
