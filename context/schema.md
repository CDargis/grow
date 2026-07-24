# grow — Schema

## TypeScript Interfaces

```typescript
// ── Enums ────────────────────────────────────────────────────────────────────

type PlantPhase =
  | 'germination' | 'seedling' | 'veg' | 'flower'
  | 'harvest' | 'drying' | 'curing' | 'archived';

type EnvironmentType =
  | 'tent' | 'outdoor' | 'garage' | 'basement'
  | 'room' | 'greenhouse' | 'other';

type LogType =
  | 'watering' | 'training' | 'trimming'
  | 'transplant' | 'height' | 'note' | 'photo' | 'phase_change';
// 'feeding' retired 2026-07-19 — collapsed into WateringData.nutrients
// (one-time migration: backend/cmd/migrate-feeding)

// ── Plant ─────────────────────────────────────────────────────────────────────

interface Plant {
  plantId: string;         // ULID
  userId: string;
  name: string;
  strain: string;
  genetics?: string;
  seedBank?: string;
  phase: PlantPhase;
  phaseStartDate: string;  // ISO date YYYY-MM-DD
  avatarKey?: string;      // S3 key in grow-media
  environmentId?: string;
  archivedAt?: string;     // ISO datetime
  createdAt: string;       // ISO datetime
}

// ── Environment ───────────────────────────────────────────────────────────────

interface Environment {
  environmentId: string;   // ULID
  userId: string;
  name: string;
  type: EnvironmentType;
  lightSchedule?: string;  // e.g. "18/6", "12/12", "auto"
  targetTempF?: number;
  targetHumidity?: number; // percentage
  photoKey?: string;       // S3 key in grow-media
  createdAt: string;
}

// ── Log ───────────────────────────────────────────────────────────────────────

interface Log {
  plantId: string;
  logId: string;           // ULID (time-sortable)
  userId: string;
  logType: LogType;
  date: string;            // YYYY-MM-DD — used for GSI date queries
  loggedAt: string;        // ISO datetime
  data: LogData;
}

// Log data variants by logType

// Unified water/feed/top-dress entry. Labels are derived (frontend
// lib/logDisplay.ts): nutrients present = "feeding"; nutrients but no
// amount = "top dress"; otherwise "watering".
interface WateringData {
  amount?: number;               // water volume; absent for a dry top-dress
  unit: 'ml' | 'l' | 'oz' | 'gal';
  ph?: number;
  runoff?: number;               // runoff pH
  tds?: number;                  // ppm
  runoffTds?: number;            // ppm
  nutrients?: Array<{ name: string; amount: number; unit: string }>;
  note?: string;
}
interface TrainingData  { method: string; notes?: string; }  // LST, topping, defoliation, etc.
interface TrimmingData  { notes?: string; }
interface TransplantData { potSize: string; medium?: string; }
interface HeightData    { height: number; unit: 'cm' | 'in'; }
interface NoteData      { text: string; }
interface PhotoData     { photoKeys: string[]; caption?: string; }  // one photo log entry can hold many photos
interface PhaseChangeData { fromPhase: PlantPhase; toPhase: PlantPhase; }

type LogData =
  | WateringData | TrainingData | TrimmingData
  | TransplantData | HeightData | NoteData | PhotoData | PhaseChangeData;

// ── Settings (global, one row per user) ────────────────────────────────────────

interface Settings {
  userId: string;
  shortcutLogTypes?: LogType[];   // PlantDetail quick-action tray, 3-5 items; unset = default 4
  sortChipOrder?: LogType[];      // Activity/Sort chip strip order; unset = catalog default order
  plantsLayoutMode?: 'grid' | 'rows' | 'fixed';  // Plants page card layout; unset = 'grid'
}
```

## DynamoDB Tables

### grow-plants
| Key | Type | Notes |
|-----|------|-------|
| PK: `plantId` | STRING | ULID |
| GSI `user-index` | PK=userId, SK=plantId | list all plants for a user |

### grow-environments
| Key | Type | Notes |
|-----|------|-------|
| PK: `environmentId` | STRING | ULID |
| GSI `user-index` | PK=userId, SK=environmentId | list all envs for a user |

### grow-logs
| Key | Type | Notes |
|-----|------|-------|
| PK: `plantId` | STRING | |
| SK: `logId` | STRING | ULID — enables range queries for a plant's logs |
| GSI `user-date-index` | PK=userId, SK=date | powers the date strip (all logs across plants for a day) |

### grow-settings
| Key | Type | Notes |
|-----|------|-------|
| PK: `userId` | STRING | one item per user; keyed by userId (not a single fixed row) so it's already multi-user-ready once real auth replaces the hardcoded `USER_ID=default` |

## S3 Media Keys

```
grow-media/
  plants/{plantId}/avatar/{filename}
  plants/{plantId}/logs/{logId}/{filename}
  environments/{environmentId}/photo/{filename}
```
