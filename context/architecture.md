# grow — Architecture

## Components

### Frontend
- React SPA, Vite build, Tailwind CSS (dark theme)
- Deployed to S3 (`grow-site` bucket), served via CloudFront
- Route 53 A record → CloudFront → S3 (OAC)
- ACM certificate for grow.chrisdargis.com

### Backend
- Single Go Lambda (`grow-api`), ARM64, provided.al2023
- API Gateway HTTP API v2 with `/{proxy+}` route
- CloudFront `/api/*` behavior → API Gateway (cache disabled)
- Pre-signed S3 URLs for photo uploads/reads (no CloudFront for media)

### Storage
- DynamoDB: `grow-plants`, `grow-environments`, `grow-logs`, `grow-settings` tables
- S3 `grow-media` bucket: plant/environment/log photos (private, pre-signed URL access)

### Infrastructure
- CDK (C#) in `infra/`
- Two stacks: `GrowPipelineStack` → `GrowStack` (via CDK Pipelines)
- Self-mutating CodePipeline sourced from GitHub (CDargis/grow, main)
- Docker bundling for Go Lambda (privileged CodeBuild)

## Data Flow

Photo upload:
1. Client requests pre-signed PUT URL from API (`POST /api/media/upload`)
2. Client uploads directly to S3 (`grow-media`)
3. Client sends log with `photoKey` referencing the S3 key

Photo read:
1. API response includes pre-signed GET URLs (1-hour TTL) for any `photoKey` fields

## CDK Asset Paths (relative to `infra/`)
- Backend: `../backend`
- Frontend: `../frontend/dist`

## Settings & Reorderable Log-Type Lists
- Global-only settings (`GET/PUT /api/settings`), one row per user in `grow-settings`, keyed by `userId` (not a single fixed row) — no per-plant override, kept intentionally simple
- `PUT /api/settings` is a partial merge (DynamoDB `UpdateItem` with only the provided fields in the `SET` expression), so the three independent editors below never clobber each other
- Three independent preferences share this one record:
  - `shortcutLogTypes` — PlantDetail quick-action tray (3–5 of 7 log types)
  - `sortChipOrder` — Activity/Sort chip strip order (all 6 chip types, no hide)
  - `plantsLayoutMode` — `'auto-fit' | 'fixed'` for the Plants page card grid
- Reordering UI: `frontend/src/components/LogTypeReorderSheet.tsx`, built on `@dnd-kit/core` + `@dnd-kit/sortable` with a `PointerSensor` activation delay (hold-to-drag, like rearranging home-screen icons). Two modes:
  - `divider` (shortcuts): single list of all catalog types with a visual divider; dragging an item across it toggles shown/hidden, clamped to min/max — one gesture picks both order and count
  - `plain` (sort chips): straightforward reorder, no hide/show
- Entry point is long-press on the tray/chip-strip itself (`frontend/src/lib/useLongPress.ts`), not a settings screen — editors open as a `BottomSheet`
- Plants page layout is pure CSS (`grid-template-columns: repeat(auto-fit, minmax(150px, 1fr))` + `gridAutoRows: 1fr` inside a `flex-1` container that fills the viewport): cards grow to fill the screen when there's room, and only wrap/scroll once they'd shrink below the minmax floor — no JS-computed sizing
