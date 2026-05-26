export type PlantPhase =
  | 'germination' | 'seedling' | 'veg' | 'flower'
  | 'harvest' | 'drying' | 'curing' | 'archived' | 'dead'

export type PlantType = 'autoflower' | 'photoperiod' | 'unknown'

export type MilestoneType =
  | 'cotyledons_off' | 'leaf_set_1' | 'leaf_set_2' | 'leaf_set_3' | 'leaf_set_4'
  | 'early_veg' | 'full_veg' | 'pre_flower'
  | 'flip_to_flower' | 'peak_flower' | 'harvest' | 'dry_complete' | 'cure_ready'

export type MilestoneStatus = 'predicted' | 'confirmed' | 'skipped'

export type MilestoneConfidence = 'low' | 'medium' | 'high'

export interface Milestone {
  plantId: string
  milestoneType: MilestoneType
  predictedDate: string
  confidence: MilestoneConfidence
  confirmedDate?: string
  status: MilestoneStatus
  updatedAt: string
  notes?: string
  lastChangedAt?: string
  lastChangedFrom?: string
  lastChangeReason?: string
}

export type ObservationCategory = 'health' | 'growth' | 'pest' | 'nutrient' | 'general'

export interface PlantObservation {
  plantId: string
  observationId: string
  observedAt: string
  category: ObservationCategory
  text: string
  sourceLogIds?: string[]
}

export type EnvironmentType =
  | 'tent' | 'outdoor' | 'garage' | 'basement'
  | 'room' | 'greenhouse' | 'other'

export type LogType =
  | 'watering' | 'feeding' | 'training' | 'trimming'
  | 'transplant' | 'height' | 'note' | 'photo' | 'phase_change'
  | 'environment_change' | 'lighting_change'

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
  lastCalibratedAt?: string
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

export interface WateringData  { amount?: number; unit: 'ml' | 'l' | 'oz' | 'gal'; ph?: number; runoff?: number }
export interface FeedingData   { nutrients: Array<{ name: string; amount: number; unit: string }>; ph?: number; totalVol?: number }
export interface TrainingData  { method: string; notes?: string }
export interface TrimmingData  { notes?: string }
export interface TransplantData { potSize: string; medium?: string }
export interface HeightData    { height: number; unit: 'cm' | 'in' }
export interface NoteData      { text: string }
export interface PhotoData     { photoKey: string; caption?: string }
export interface PhaseChangeData       { fromPhase: PlantPhase; toPhase: PlantPhase }
export interface EnvironmentChangeData { fromEnvironmentId?: string; toEnvironmentId?: string }
export interface LightingChangeData    { fromSchedule?: string; toSchedule?: string }

export type LogData =
  | WateringData | FeedingData | TrainingData | TrimmingData
  | TransplantData | HeightData | NoteData | PhotoData | PhaseChangeData
  | EnvironmentChangeData | LightingChangeData

export interface MilestoneHistory {
  plantId: string
  recalSK: string
  milestoneType: MilestoneType
  predictedDate: string
  confidence: MilestoneConfidence
  recalibratedAt: string
  reason?: string
}

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

export interface CreateEnvironmentRequest {
  name: string
  type: EnvironmentType
  lightSchedule?: string
  targetTempF?: number
  targetHumidity?: number
  photoKey?: string
}
