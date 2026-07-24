# grow — Decisions

## Go for Lambda
Go compiles to a single static binary with fast cold starts. ARM64 (Graviton) for cost/performance. Single Lambda handles all API routes via an HTTP mux — no per-route Lambda proliferation.

## Pre-signed S3 URLs for media
Photos upload directly from browser to S3 via pre-signed PUT URLs. Lambda generates short-lived (1h) GET URLs inline in API responses. Avoids proxying binary data through Lambda and keeps the media bucket private without needing CloudFront signed cookies.

## Single-table per entity (not single-table design)
Three separate DynamoDB tables (plants, environments, logs) rather than a single-table design. Access patterns are simple and distinct enough that the complexity of single-table isn't warranted. Separate tables also make IAM scoping cleaner.

## ULID for IDs
ULIDs are time-sortable and URL-safe. The logs table uses logId as SK, so time-sortability gives chronological ordering for free.

## userId is a fixed constant initially
Single-user app to start. userId = "default". Auth can be layered in later (Cognito or similar) without schema changes since userId is already in the data model.

## logTypeDate composite attribute on logs
A `logTypeDate` attribute (`logType#date`, e.g. `watering#2026-07-04`) is written on every log create/update. Powers the `user-logtype-date-index` GSI, enabling efficient "last occurrence of log type X per plant" queries for the Activity sort view — without this, the query would require a full scan filtered by logType.

## date field on logs (denormalized)
The `date` (YYYY-MM-DD) field on Log is denormalized from `loggedAt` to enable the GSI for date-strip queries. Without it, date-range queries on the GSI would require timestamp comparisons across ISO strings.

## Delete log's S3 media on log delete (2026-07-23)
`LogStore.Delete` previously only removed the DynamoDB item, leaving photo objects in S3 (`grow-media`) orphaned forever. It now uses `ReturnValues: ALL_OLD` to get the deleted item back without an extra read, and the handler best-effort deletes any `photoKeys`/`photoKey` it referenced via `S3.DeleteObjects`. Cleanup failure is logged but doesn't fail the request — the log record is already gone either way, and a dangling S3 object is preferable to a delete that reports failure when it actually succeeded.

## Global-only settings, keyed by userId (2026-07-24)
Considered per-plant overrides for the shortcut tray but dropped them — not worth the UX complexity for a use case that would rarely come up. One `Settings` item per user in `grow-settings`, partition-keyed on `userId` (not a single fixed row), so when real auth eventually replaces the hardcoded `USER_ID=default`, settings are already correctly scoped per user with no migration. `PUT /api/settings` merges only the fields provided (DynamoDB `UpdateItem`, not a full overwrite) so the three independent preference editors can't clobber each other.

## No dedicated settings screen for reorderable lists (2026-07-24)
Shortcut tray order and Activity sort-chip order are edited via long-press-to-enter-edit-mode directly on the tray/chip-strip (same convention as rearranging home-screen icons), not a navigable settings page — the editor is a `BottomSheet`, not a route. Only the Plants-page layout toggle (`grid` / `rows` / `fixed`) got a real settings screen (gear icon, `SettingsSheet`), since a mode toggle doesn't have a natural drag gesture. Icon is a gear, not a profile/account icon — the sheet holds display preferences, not identity/account data, and a profile icon would set the wrong expectation. If real multi-user accounts are added later, the intended path is consolidating under a single avatar icon whose menu includes "Settings" as one item, not stacking a second icon.

## CSS-only auto-fit layout for Plants page cards (2026-07-24)
`grid-template-columns: repeat(auto-fit, minmax(150px, 1fr))` + `gridAutoRows: 1fr` inside a `flex-1` container, rather than JS-computed rows/columns. `auto-fit` collapses unused column tracks to zero width, so a single plant's card still fills the full width/height; cards stop shrinking at the `minmax` floor (protects the name's font size) and wrap to scroll once there are too many to fit one screen. No manual layout math needed. Added a `rows` variant (2026-07-24, same day) — same mechanism with `gridTemplateColumns: '1fr'` fixed to one column — after finding out "auto-fit" read as "grid of columns" to the person who asked for it, when a single auto-sized column was also a mode worth having.

## Feeding collapsed into watering (2026-07-19)
Real-world feeds are waterings with nutrients mixed in, and dry top-dresses get watered in too — the split forced double-logging (July 18 sessions produced watering+feeding pairs for single events). `WateringData` now carries optional `nutrients[]`, `tds`, `runoffTds`; the stored logType is always `watering` and UI labels derive from data shape (nutrients → "feeding", nutrients without amount → "top dress"). Chose a one-time migration (`backend/cmd/migrate-feeding`, dry-run by default) over read-time shimming: only 11 feeding rows existed, and shimming would leave every consumer branching on two types forever, with old and new feeds under different GSI keys. The migration also merges same-event watering+feeding twins (same plant/date, matching volume, <1h apart) and deletes the duplicate row.
