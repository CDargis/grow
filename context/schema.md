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

// ── Oils (Go: backend/internal/model/oil.go) ───────────────────────────────────
// Lots in, batches out. All potency values are mg/g (1% = 10 mg/g); mg/mL is
// always derived (mg/g × densityGPerMl), never stored. Batch profiles (the ROBP)
// are calculated on read by internal/oilcalc, never stored.

type BatchType = 'DIS' | 'ISO' | 'FS' | 'THC' | 'BL';
type Confidence = 'low' | 'medium' | 'high';

// Present key with 0 = tested, not detected. Absent key = not reported.
interface Profile {
  cannabinoids?: Partial<Record<CannabinoidKey, number>>;  // KnownCannabinoids
  terpenes?: Partial<Record<TerpeneKey, number>>;          // KnownTerpenes
  other?: Array<{ name: string; mgPerG: number }>;         // anything off-list
}
type CannabinoidKey = 'cbd' | 'cbda' | 'd9thc' | 'thca' | 'd8thc' | 'cbg' | 'cbga'
  | 'cbn' | 'cbc' | 'cbdv' | 'thcv';
type TerpeneKey = 'myrcene' | 'limonene' | 'beta_caryophyllene' | 'linalool'
  | 'alpha_pinene' | 'beta_pinene' | 'humulene' | 'terpinolene' | 'ocimene'
  | 'bisabolol' | 'nerolidol' | 'caryophyllene_oxide' | 'eucalyptol' | 'geraniol'
  | 'camphene' | 'terpineol';

// The supplier's lot. Unique per user on (supplier, supplierLotNumber),
// case-insensitive.
interface Lot {
  lotId: string;                  // ULID
  userId: string;
  supplier: string;               // "Red Mesa"
  product: string;                // isolate / distillate / …
  supplierLotNumber: string;
  gramsPurchased?: number;
  coaUrl?: string;
  coaS3Key?: string;              // archived copy in grow-media; absent = not archived
  profile: Profile;
  notes?: string;
  createdAt: string;
  updatedAt?: string;
}

type BatchInput =
  | { kind: 'lot'; lotId: string; grams: number }
  | { kind: 'batch'; batchId: string; grams: number }
  | { kind: 'flower'; grams: number;          // grams of flower, not oil
      strain: string; plantId?: string;
      profile: Profile;                        // label/COA numbers, acids as reported
      efficiency: number;                      // (0, 1]; decarb applied separately
      method: string; confidence: Confidence;
      profileSource?: 'coa' | 'label' | 'breeder' | 'guess';  // where the numbers came from;
                                               // informational, optional for older inputs
      coaUrl?: string;                         // the flower's own COA, often a JPG
      coaS3Key?: string }                      // archived copy; numbers stay estimates
  | { kind: 'additive'; grams: number; name: string; profile?: Profile; note?: string };

interface Batch {
  batchId: string;                // ULID
  userId: string;
  code: string;                   // [type]-<subType>-<discriminator>-[YYMMDD]-<batchDiscriminator>
                                  // built from the parts below; unique per user
  type: BatchType;
  subType?: string;               // letters/digits only, uppercased
  discriminator?: string;
  date?: string;                  // YYYY-MM-DD; absent → "000000" in code
  batchDiscriminator?: string;
  inputs: BatchInput[];
  carrier?: { name: string; grams: number };
  densityGPerMl: number;          // per batch; default 0.95, LouAna ≈ 0.94
  totalGrams: number;             // oil made
  labCoa?: { url: string; s3Key?: string; profile: Profile };  // replaces calculated profile
  notes?: string;
  createdAt: string;
  updatedAt?: string;
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

### grow-products
| Key | Type | Notes |
|-----|------|-------|
| PK: `productId` | STRING | ULID; nutrient products (see `backend/internal/model/product.go`) |
| GSI `user-index` | PK=userId, SK=productId | list all products for a user |

### grow-oil-lots
| Key | Type | Notes |
|-----|------|-------|
| PK: `lotId` | STRING | ULID |
| GSI `user-index` | PK=userId, SK=lotId | list all lots for a user; (supplier, lot number) uniqueness checked in Go |

### grow-oil-batches
| Key | Type | Notes |
|-----|------|-------|
| PK: `batchId` | STRING | ULID |
| GSI `user-index` | PK=userId, SK=batchId | list all batches for a user; code uniqueness and lookup-by-code done in Go |

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
  oils/lots/{lotId}/coa/{ulid}.{pdf|html|…}       # archived copy of a lot's COA link
  oils/batches/{batchId}/coa/{ulid}.{pdf|html|…}  # archived copy of a batch lab COA
  oils/batches/{batchId}/flower-coa/{ulid}.{jpg|pdf|…}  # archived copy of a flower input's COA
```
