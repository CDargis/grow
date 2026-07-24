# grow — Todo

## Shipped
- [x] CDK infra — DynamoDB, Lambda, API Gateway, CloudFront, Route53
- [x] Go Lambda: full CRUD for plants, environments, logs
- [x] Plant detail: journal view (date strip, quick action tray, log entries)
- [x] Plant detail: phase timeline view
- [x] Plant detail: phase changes, environment assignment
- [x] Log types: watering, feeding, training, trimming, height, note, photo, transplant, sprout
- [x] Photo upload via pre-signed S3 URLs
- [x] Plant avatar/cover photo (selected from existing log photos)
- [x] Activity tab: cross-plant recent feed + sort by log type
- [x] DynamoDB GSI for log-type-date queries (user-logtype-date-index)
- [x] PWA manifest + splash screen for Android home screen install
- [x] Photo lightbox: swipe/chevron prev-next across all of a plant's photos (chronological, with date · phase · day overlay)
- [x] Unified water/feed/top-dress log entry (nutrients + TDS on WateringData; feeding type retired; migrate-feeding command)
- [x] Journal day-swipe: horizontal swipe in log area moves ±1 day; date strip follows when crossing a week boundary
- [x] Delete a log entry with photos → S3 media cleaned up too (photo/training/trimming log types)
- [x] Global settings (grow-settings table, GET/PUT /api/settings): shortcut tray order, sort-chip order, Plants layout mode
- [x] PlantDetail shortcut tray: long-press to reorder + choose 3–5 shown (drag-with-divider editor)
- [x] Activity/Sort chip strip: long-press to reorder
- [x] Plants page: auto-fit card grid (fills screen, scrolls once cards hit min size) with a fixed-size fallback toggle in Settings

## Not yet deployed
- [ ] `cdk deploy` needed before grow-settings table exists — settings endpoints will 500 until then (verified locally: no AWS creds in this sandbox, so this wasn't smoke-tested end-to-end)

## Backlog
- [ ] Height chart per plant
- [ ] Feeding/watering history charts
- [ ] Environment detail view (plants assigned, light schedule, assigned plants)
- [ ] Multi-user / auth — see plans/backlog/multi-user.md
