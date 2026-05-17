export type PlantPhase =
  | 'germination' | 'seedling' | 'veg' | 'flower'
  | 'harvest' | 'drying' | 'curing' | 'archived'

export type EnvironmentType =
  | 'tent' | 'outdoor' | 'garage' | 'basement'
  | 'room' | 'greenhouse' | 'other'

export type LogType =
  | 'watering' | 'feeding' | 'training' | 'trimming'
  | 'transplant' | 'height' | 'note' | 'photo' | 'phase_change'

export interface Plant {
  plantId: string
  userId: string
  name: string
  strain: string
  genetics?: string
  seedBank?: string
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

export interface WateringData  { amount: number; unit: 'ml' | 'l' | 'oz' | 'gal'; ph?: number; runoff?: number }
export interface FeedingData   { nutrients: Array<{ name: string; amount: number; unit: string }>; ph?: number; totalVol?: number }
export interface TrainingData  { method: string; notes?: string }
export interface TrimmingData  { notes?: string }
export interface TransplantData { potSize: string; medium?: string }
export interface HeightData    { height: number; unit: 'cm' | 'in' }
export interface NoteData      { text: string }
export interface PhotoData     { photoKey: string; caption?: string }
export interface PhaseChangeData { fromPhase: PlantPhase; toPhase: PlantPhase }

export type LogData =
  | WateringData | FeedingData | TrainingData | TrimmingData
  | TransplantData | HeightData | NoteData | PhotoData | PhaseChangeData

export interface CreatePlantRequest {
  name: string
  strain: string
  genetics?: string
  seedBank?: string
  phase: PlantPhase
  environmentId?: string
}

export interface CreateLogRequest {
  logType: LogType
  date?: string
  data: LogData
}

export interface CreateEnvironmentRequest {
  name: string
  type: EnvironmentType
  lightSchedule?: string
  targetTempF?: number
  targetHumidity?: number
}
