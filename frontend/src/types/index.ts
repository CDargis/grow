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

export interface WateringData  { amount?: number; unit: 'ml' | 'l' | 'oz' | 'gal'; ph?: number; runoff?: number; note?: string }
export interface FeedingData   { nutrients: Array<{ name: string; amount: number; unit: string }>; ph?: number; totalVol?: number }
export interface TrainingData  { method: string; notes?: string; photoKey?: string }
export interface TrimmingData  { method?: string; notes?: string; photoKey?: string }
export interface SproutData    { }
export interface TransplantData { potSize: string; medium?: string }
export interface HeightData    { height: number; unit: 'cm' | 'in' }
export interface NoteData      { text: string }
export interface PhotoData     { photoKey: string; caption?: string }
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

export interface CreateEnvironmentRequest {
  name: string
  type: EnvironmentType
  lightSchedule?: string
  targetTempF?: number
  targetHumidity?: number
  targetVpd?: number
  photoKey?: string
}
