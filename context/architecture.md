# grow — Architecture

## Components

### Frontend
- React SPA, Vite build, Tailwind CSS (dark theme)
- Deployed to S3 (`grow-site` bucket), served via CloudFront
- Route 53 A record → CloudFront → S3 (OAC)
- ACM certificate for grow.chrisdargis.com

### Backend
- Single Go Lambda (`grow-api`), ARM64, provided.al2023
- API Gateway HTTP API v2 with `/{proxy+}` route, guarded by a Cognito JWT authorizer
  (`GET /api/auth-config` is the one route excluded from it — see Auth below)
- CloudFront `/api/*` behavior → API Gateway (cache disabled)
- Pre-signed S3 URLs for photo uploads/reads (no CloudFront for media)

### Auth
- Cognito User Pool (`grow-users`), self-signup disabled — one admin-created user (Chris), not a
  public app. Real login, not a placeholder, but "real multi-user" is still just the data-model
  readiness (`userId` on every entity) plus this pool, not open signup.
- API Gateway HTTP API JWT Authorizer validates the token before Lambda is invoked, on every
  route except `/api/auth-config`. Backend never validates JWTs itself.
- `app.userID(r)` (`backend/cmd/api/main.go`) reads the Cognito `sub` claim that API Gateway
  passes through in the request context (via `aws-lambda-go-api-proxy/core`), replacing the old
  fixed `USER_ID=default` field. Falls back to `USER_ID` env var only when running `LOCAL=1`
  (no API Gateway in front locally, so no claims to read) — see local-dev.md for what that means
  for local testing now that Cognito exists.
- Frontend: `frontend/src/auth/` — `oidc-client-ts`'s `UserManager`, Authorization Code + PKCE
  against Cognito's Hosted UI. `AuthProvider` redirects to login immediately if there's no valid
  session (no "please log in" landing page — personal app, one user); if the stored access token
  is just expired (not absent), it tries `signinSilent()` (refresh token, no user interaction)
  before falling back to a full redirect — `automaticSilentRenew` alone only fires while the tab
  stays open continuously, so without this a session idle for over an hour (access token TTL)
  forced a full re-login every time. `RefreshTokenValidity` is 365 days on both App Clients.
  `GET /api/auth-config` supplies the User Pool authority/client ID at runtime rather than baking
  them into the build, since those IDs don't exist until after the CDK stack that creates them
  is deployed.
- API calls attach the access token as `Authorization: Bearer` (`frontend/src/api/client.ts`);
  a 401 response triggers `signinRedirect()` rather than surfacing an error.

### MCP (read-only Claude custom connector)
- `backend/internal/mcpserver/` — official `modelcontextprotocol/go-sdk`, Streamable HTTP
  transport, stateless. Tools: `list_plants`, `get_plant`, `list_logs_for_plant`,
  `get_logs_by_date_range` — thin wrappers over `internal/store`, plus an explicit
  `plant.UserID == callerUserID` ownership check at the tool layer (the REST API doesn't have
  this check either — pre-existing gap, not fixed everywhere, but cheap to add for this new
  surface).
- `GET /api/mcp` is excluded from the API Gateway JWT authorizer and validates its own bearer
  token in Go (`lestrrat-go/jwx/v3` against Cognito's JWKS) — API Gateway's built-in authorizer
  returns a generic 401 with no `WWW-Authenticate` header, but MCP clients need that header's
  `resource_metadata` URL to discover where to authenticate. Cognito is still the only OAuth
  implementation; this only validates tokens Cognito issued, it doesn't reimplement /authorize
  or /token.
- Two unauthenticated discovery documents at the domain root (own CloudFront `/.well-known/*`
  behavior, since they can't live under `/api/*`):
  - `GET /.well-known/oauth-protected-resource` — points MCP clients at **our origin** as the
    authorization server
  - `GET /.well-known/oauth-authorization-server` — RFC 8414 document we host ourselves,
    with endpoints pointing at Cognito's real hosted-UI `/oauth2/authorize` and `/oauth2/token`.
    Hosted by us because Cognito's path-bearing issuer URL serves discovery at only 1 of the 4
    standard URL forms, which breaks MCP-client metadata discovery entirely (see decisions.md).
    Not a proxy — the actual OAuth flow goes straight to Cognito.
- The connector uses the dedicated confidential `grow-mcp` App Client (ID + secret entered
  manually in Claude's connector settings — Cognito has no Dynamic Client Registration, and
  Claude falls back to manual credentials, validated in `spikes/mcp-auth/`).
- Confirmed working end-to-end with the real Claude.ai connector (2026-07-26).

### Storage
- DynamoDB: `grow-plants`, `grow-environments`, `grow-logs`, `grow-settings` tables
- S3 `grow-media` bucket: plant/environment/log photos (private, pre-signed URL access)

### Infrastructure
- CDK (C#) in `infra/`
- Two stacks: `GrowPipelineStack` → `GrowStack` (via CDK Pipelines)
- Self-mutating CodePipeline sourced from GitHub (CDargis/grow, main)
- **Pipeline does not auto-trigger on push** (cause unknown — repo owner/name casing in
  PipelineStack.cs matches the GitHub remote, which is the usual culprit and was ruled out).
  Start deploys manually: `aws codepipeline start-pipeline-execution --name GrowPipeline`
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
  - `plantsLayoutMode` — `'grid' | 'rows' | 'fixed'` for the Plants page card layout
- Reordering UI: `frontend/src/components/LogTypeReorderSheet.tsx`, built on `@dnd-kit/core` + `@dnd-kit/sortable` with a `PointerSensor` activation delay (hold-to-drag, like rearranging home-screen icons). Two modes:
  - `divider` (shortcuts): single list of all catalog types with a visual divider; dragging an item across it toggles shown/hidden, clamped to min/max — one gesture picks both order and count
  - `plain` (sort chips): straightforward reorder, no hide/show
- Entry point is long-press on the tray/chip-strip itself (`frontend/src/lib/useLongPress.ts`), not a settings screen — editors open as a `BottomSheet`
- Plants page layout is pure CSS, no JS-computed sizing, inside a `flex-1` container that fills the viewport:
  - `grid` — `grid-template-columns: repeat(auto-fit, minmax(150px, 1fr))` + `gridAutoRows: 1fr`: cards fill the screen in columns and rows, wrapping/scrolling once they'd shrink below the minmax floor
  - `rows` — same idea but `gridTemplateColumns: '1fr'` (single column): rows fill the screen vertically instead
  - `fixed` — original fixed-height list, no auto-sizing
