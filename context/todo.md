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
- [x] Read-only MCP connector — **live and working in Claude.ai** (tools: list_plants,
      get_plant, list_logs_for_plant, get_logs_by_date_range). Took three fixes: Claude's
      callback URL on the App Client, a dedicated confidential `grow-mcp` client, and finally
      hosting the RFC 8414 discovery metadata ourselves (Cognito's path-bearing issuer breaks
      MCP client discovery — the actual root cause). Full postmortem in
      `context/plans/active/cognito-auth-mcp-connector.md`.

## Follow-ups from the auth/MCP work
- [ ] Web app forces a full re-login whenever the access token expires (~1h after the tab
      closes) — `AuthProvider` should try `signinSilent()` (refresh token) before redirecting;
      also consider raising `RefreshTokenValidity` for fewer password prompts
- [ ] Cognito Hosted UI branding (logo/colors) — cosmetic, cheap, parked
- [ ] Delete `spikes/mcp-auth/` — superseded by the real implementation

## Backlog
- [ ] Height chart per plant
- [ ] Feeding/watering history charts
- [ ] Environment detail view (plants assigned, light schedule, assigned plants)
