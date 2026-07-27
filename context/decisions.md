# grow — Decisions

## Multi-photo batches keep selection/upload order, not capture time (2026-07-27)
Went back and forth on this. Tried sorting by `File.lastModified`, then by real EXIF
`DateTimeOriginal` (via `exifr`) when that proved to be only a filesystem-attribute proxy for
capture time. User then explicitly reversed course: whatever order the photos were
selected/uploaded in is what should be shown, including when uploading an older batch later --
that upload should appear last, not interleaved back into the timeline by capture time. Reverted
to plain selection order, no sorting; `exifr` removed as a dependency again. The other half of
the original bug -- two *different* log entries tying on same-minute `loggedAt` -- is unrelated
to this reversal and stays fixed (`logId` tiebreak in `photoItems`, `PlantDetail/index.tsx`).

## Denormalize plantName onto MCP log entries (2026-07-26)
Live use surfaced a real failure: asked about a log entry, Claude quoted back the tool's raw
JSON but misattributed it to the wrong plant -- two plants' ids differ by only a couple of
characters (`...9KK9...` vs `...9KM9...`), and matching an opaque id back to a name had been left
entirely to the model's in-context recall of an earlier `list_plants` call. Verified the tool and
data were correct (direct DB scan confirmed the entry's real plantId) -- this was a model
transcription error, not a server bug, but it's a completely avoidable failure mode: `logWithPlantName`
now wraps every log entry from `list_logs_for_plant`/`get_logs_by_date_range` with the plant's
name directly, so there's no id-matching step left for a model to get wrong.

## Host OAuth authorization-server metadata ourselves, not Cognito's (2026-07-26)
Cognito's issuer URL has a path component (`/{poolId}`) and serves its discovery document at
only the OIDC suffix form — the RFC 8414 path-insertion forms and both
`oauth-authorization-server` forms return 400. Claude's MCP connector discovery never finds it,
killing the OAuth flow before any request reaches the login page, Cognito, or the Lambda (no
logs anywhere — the defining symptom). So `grow.chrisdargis.com/.well-known/oauth-authorization-server`
serves a static RFC 8414 document (issuer = our origin, endpoints = Cognito's real hosted-UI
authorize/token), and the protected-resource metadata lists our origin as the authorization
server. Not a proxy: authorize/token still go straight to Cognito; token validation still checks
Cognito's real issuer. This reproduces the exact topology the spike validated (metadata at a
path-free origin root). This was the third and final fix — the callback-URL fix and the
confidential client were each necessary but not sufficient.

## Separate Cognito App Client for the MCP connector (2026-07-26)
Cognito's OIDC discovery document (`token_endpoint_auth_methods_supported`) only ever lists
`client_secret_basic`/`client_secret_post` -- never `"none"`, even for genuinely public/PKCE-only
clients. Confirmed by manually replaying the full login+token exchange against the real Cognito
endpoints with `grow-web` (public, no secret): it works fine directly, but a standards-compliant
OAuth client (Claude) reading that discovery doc has no way to know "none" is actually supported,
so it likely tries secret-based auth it doesn't have. Rather than compromise the browser
frontend's client (which can't safely hold a secret) to work around this, added a second App
Client, `grow-mcp`, confidential (has a real secret), used only by Claude's connector. Same user
pool, same one user -- just two different client "shapes" for two different kinds of caller.

## /api/mcp validates its own JWT instead of using the API Gateway authorizer (2026-07-26)
Every other route uses API Gateway's built-in JWT authorizer, which validates the token before
Lambda is even invoked. `/api/mcp` can't use it: on missing/invalid auth, MCP clients discover
where to log in via a 401's `WWW-Authenticate: Bearer resource_metadata="..."` header, and API
Gateway's authorizer returns a generic 401 with no custom header. So `/api/mcp` is registered
without the authorizer (like `/api/auth-config`) and validates the bearer token itself in Go
(`backend/internal/mcpserver`, `lestrrat-go/jwx/v3` against Cognito's JWKS, cached 1h). This
doesn't reimplement OAuth -- Cognito is still the only issuer/authorization server -- it just
means this one route checks the token in application code instead of at the gateway.

## Ownership check added at the MCP tool layer, not fixed API-wide (2026-07-26)
`get_plant` and `list_logs_for_plant` verify `plant.UserID == callerUserID` before returning
data; the underlying REST handlers (`getPlant` etc.) don't do this anywhere in the existing API
-- they just fetch by id with no ownership check, a gap that predates this work and is harmless
today (single real user). Added the check at the MCP layer specifically because an LLM-facing
tool is a different trust surface worth hardening cheaply, without taking on an API-wide
authorization refactor that isn't otherwise in scope.

## Bumped Go to 1.25, backend and Lambda build image together (2026-07-26)
Added `lestrrat-go/jwx/v3` for MCP's JWT verification; jwx/v3 itself requires go 1.25 in its own
go.mod. Rather than fight that (pin to jwx/v2, deprecated; or a different library), bumped
`backend/go.mod`'s `go` directive and the CDK Lambda build's pinned Docker image
(`golang:1.24-alpine` -> `golang:1.25-alpine`) together, keeping local builds and the actual
Lambda build environment in sync.

## Cognito confirmed viable for MCP custom connector auth (2026-07-26)
Spiked (`spikes/mcp-auth/`) whether Claude's custom-connector OAuth flow requires Dynamic Client
Registration (RFC 7591), which Cognito doesn't support. It doesn't — Claude falls back to a
manual "enter an OAuth Client ID" field when a server's metadata has no `registration_endpoint`,
confirmed with a real Claude.ai connector completing the full authorize/token/tool-call flow
against a throwaway server mimicking Cognito's no-DCR shape. Proceeding with Cognito + API
Gateway JWT authorizer as planned in `context/plans/complete/cognito-auth-mcp-connector.md` —
no DCR shim, no swap to a different identity provider.

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
