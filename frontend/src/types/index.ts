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
  // Used to scale dry-amendment doses in the feed wizard -- independent
  // fields (not a derived conversion), since pot shape makes diameter <->
  // volume unreliable and product labels use whichever the maker chose.
  potSizeGal?: number
  potSizeDiameterIn?: number
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
  // Delivered elemental grams, snapshotted at log-creation time (amount x
  // product density x elemental%/100 -- amount is already the real applied
  // dose, so no separate partial-dose multiply).
  deliveredN?: number
  deliveredP?: number
  deliveredK?: number
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
  potSizeGal?: number
  potSizeDiameterIn?: number
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
  // 'in' = pot diameter, for dry amendments labeled that way (e.g. Dr
  // Earth's dosing tiers by pot diameter rather than by volume).
  perVolumeUnit: 'gal' | 'l' | 'in'
}

export interface Product {
  productId: string
  userId: string
  name: string
  brand?: string
  form: ProductForm
  npk: NPK
  // Derived from npk server-side (P as P2O5, K as K2O on the label -- these
  // are the actual elemental percentages). Never edited directly.
  elementalNpk: NPK
  referenceDose: ReferenceDose
  // Grams per one referenceDose.unit. Defaulted server-side by unit when
  // omitted; densityCalibrated is false until the user has actually
  // weighed a dry (tbsp/tsp) product -- ml/oz/g get a trustworthy default.
  density?: number
  densityCalibrated: boolean
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
  density?: number
  densityCalibrated?: boolean
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

// ── Oils (see context/schema.md; Go: backend/internal/model/oil.go) ────────────
// All potency values are mg/g. mg/mL is calculated by the backend.

export type BatchType = 'DIS' | 'ISO' | 'FS' | 'THC' | 'BL'
export type Confidence = 'low' | 'medium' | 'high'
export type ProvenanceSource = 'lab' | Confidence

export interface OilProfile {
  cannabinoids?: Record<string, number>
  terpenes?: Record<string, number>
  other?: Array<{ name: string; mgPerG: number }>
}

export interface Lot {
  lotId: string
  userId: string
  supplier: string
  product: string
  supplierLotNumber: string
  gramsPurchased?: number
  coaUrl?: string
  coaS3Key?: string
  profile: OilProfile
  notes?: string
  createdAt: string
  updatedAt?: string
}

export interface BatchInput {
  kind: 'lot' | 'batch' | 'flower' | 'additive'
  grams: number
  lotId?: string
  batchId?: string
  strain?: string
  plantId?: string
  profile?: OilProfile
  efficiency?: number
  method?: string
  confidence?: Confidence
  profileSource?: 'coa' | 'label' | 'breeder' | 'guess'  // flower only; informational
  coaUrl?: string    // flower only: the flower's own COA (often an image)
  coaS3Key?: string  // archived copy; absent = not archived
  name?: string
  note?: string
}

export interface Batch {
  batchId: string
  userId: string
  code: string
  type: BatchType
  subType?: string
  discriminator?: string
  date?: string
  batchDiscriminator?: string
  inputs: BatchInput[]
  carrier?: { name: string; grams: number }
  densityGPerMl: number
  totalGrams: number
  labCoa?: { url: string; s3Key?: string; profile: OilProfile }
  notes?: string
  createdAt: string
  updatedAt?: string
}

export interface Compound {
  group: 'cannabinoid' | 'terpene' | 'other'
  key: string
  name: string
  mgPerG: number
  mgPerMl: number
  shares: Partial<Record<ProvenanceSource, number>>  // fractions, sum to 1
}

export interface BatchProfile {
  batchId: string
  code: string
  densityGPerMl: number
  fromLabCoa: boolean
  compounds: Compound[]
  calculated?: Compound[]
  totalCbd: Compound
  totalThc: Compound
}

export interface BatchView {
  batch: Batch
  profile?: BatchProfile
  profileError?: string
}
