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

## date field on logs (denormalized)
The `date` (YYYY-MM-DD) field on Log is denormalized from `loggedAt` to enable the GSI for date-strip queries. Without it, date-range queries on the GSI would require timestamp comparisons across ISO strings.
