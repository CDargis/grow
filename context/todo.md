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

## Backlog
- [ ] Height chart per plant
- [ ] Feeding/watering history charts
- [ ] Environment detail view (plants assigned, light schedule, assigned plants)
- [ ] Multi-user / auth — see plans/backlog/multi-user.md
