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
- [x] Plants page layout config: Grid (fills screen in columns+rows), Rows (single column, fills screen vertically), or Fixed size — toggle in Settings
- [x] Camera-direct photo option (Take Photo alongside Library) for photo logs, Training, Trimming, Environment photos
- [x] Cognito auth (single real user) — deployed, live, confirmed working end-to-end. Required
      migrating all existing data's `userId` from the old hardcoded `"default"` to the real
      Cognito `sub` after first deploy — see incident note in the plan doc.

## In progress
- [ ] Read-only MCP connector — see `context/plans/active/cognito-auth-mcp-connector.md`.
      `backend/internal/mcpserver/` written (tools: list_plants, get_plant, list_logs_for_plant,
      get_recent_activity), CDK routes + new CloudFront `/.well-known/*` behavior written,
      **not deployed yet**. Left to do: deploy, then actually add the connector in Claude.ai
      (manual Client ID, per the spike finding) and verify a real tool call works.

## Backlog
- [ ] Height chart per plant
- [ ] Feeding/watering history charts
- [ ] Environment detail view (plants assigned, light schedule, assigned plants)
