# Cognito Auth (Single Real User) + Read-Only MCP Connector

## Status
Scoped, not started. Pulled forward from `context/plans/backlog/multi-user.md` Feature 1 — this
is a smaller slice of that (real login for Chris only, no public signup), taken on now because
Claude's custom connector needs real OAuth, and building a throwaway auth scheme just for MCP
would mean redoing this work later anyway.

## Goal
Claude (mobile app / claude.ai) can read grow's plants/logs via a custom connector, authenticated
as Chris specifically — not an open/public endpoint, not a one-off API key.

## Non-goals (for this slice)
- No public signup UI, no multi-tenant frontend changes
- No write access from Claude (read-only tools only)
- No read-only sharing links (Feature 2 in the backlog doc) — different auth model, unrelated

## Architecture

### 1. Cognito User Pool (real auth foundation)
- CDK: `UserPool` + `UserPoolClient`, self-signup disabled
- One user (Chris) admin-created directly (CLI/console), not via a signup flow
- API Gateway HTTP API JWT Authorizer attached to existing routes, pointed at the User Pool
  issuer — validates the token *before* it reaches Lambda, so most routes need no code change
- Backend: `app.userID` is currently a single field set once at startup from `USER_ID=default`
  (`backend/cmd/api/main.go`). Needs to become per-request, read from the JWT claims that
  API Gateway passes through (via `aws-lambda-go-api-proxy`'s request-context helpers). This
  touches every handler that currently reads `a.userID`.
- Frontend: Cognito Hosted UI login (OAuth Authorization Code + PKCE), store the token, attach
  `Authorization: Bearer <token>` to every `/api/*` call, handle 401 → redirect to login.

### 2. MCP server (read-only)
- Reuse the same Lambda + HTTP API rather than standing up new infra — add MCP routes
  (e.g. `/mcp`) guarded by the same JWT authorizer, reusing the existing `internal/store`
  packages directly (no new data-access code)
- Tools for v1: `list_plants`, `get_plant`, `list_logs_for_plant`, `get_recent_activity` —
  thin wrappers over existing store methods, read-only
- Protocol implementation: use the official MCP Go SDK rather than hand-rolling JSON-RPC/session
  framing — verify current package name/maturity at implementation time, ecosystem is young
- Expose at `grow.chrisdargis.com/mcp` (or similar) via the existing CloudFront `/api/*`-style
  behavior

## Spike result (2026-07-26) — RESOLVED, not a blocker
Built `spikes/mcp-auth/` — a throwaway Go server (official `modelcontextprotocol/go-sdk`) with
a minimal OAuth authorization server exposing metadata with **no `registration_endpoint`**, to
simulate exactly Cognito's no-DCR situation. Exposed via a `cloudflared` quick tunnel, added as
a real custom connector in Claude.ai.

Result: Claude's connector setup tried automatic (DCR) registration, failed as expected
("Automatic client registration isn't supported"), and **fell back to a UI field for manually
entering an OAuth Client ID**. Once a client ID was entered by hand, the full flow completed
for real — authorize redirect, PKCE-verified token exchange, and an authenticated `tools/call`
all confirmed in the server log, then a live tool call ("ping") round-tripped correctly through
Claude.

**Conclusion: Cognito's lack of DCR is not a blocker.** No shim needed, no swap to a different
IdP. One manual step when setting up the connector (entering the app client ID once) is the only
consequence — proceed with the original architecture (Cognito + API Gateway JWT authorizer)
as planned.

## Build order
1. ~~Spike~~ — done, see above. Spike server/tunnel torn down, connector removed from Claude.ai.
2. ~~Cognito User Pool + JWT authorizer wired to the existing API (all routes)~~ — written,
   not deployed. `infra/src/Grow/GrowStack.cs`: `UserPool` (self-signup disabled, no public
   users), `UserPoolClient` (public SPA client, no secret), Hosted UI domain
   `grow-chrisdargis`, `HttpJwtAuthorizer` attached to the `{proxy+}` route. One route,
   `GET /api/auth-config`, is registered separately *without* the authorizer (more specific
   path wins over the catch-all) — it returns the (non-secret) authority/client ID the
   frontend needs to start the login flow before it has a token.
3. ~~Backend: per-request userId from JWT claims, replace `USER_ID=default`~~ — done.
   `app.userID(r)` (`backend/cmd/api/main.go`) reads the Cognito `sub` claim via
   `aws-lambda-go-api-proxy/core.GetAPIGatewayV2ContextFromContext`, falling back to the
   `USER_ID` env var only when there's no API Gateway context at all (`LOCAL=1` dev server).
4. ~~Frontend: Hosted UI login flow, token storage, 401 handling~~ — done.
   `frontend/src/auth/` (`oidc-client-ts`), `AuthProvider` wraps the app and redirects
   straight to Cognito if there's no valid session (no login-page UI needed, one user).
   `api/client.ts` attaches the access token and redirects to login on 401.
5. **Not started**: MCP read-only routes + tools (reuse `spikes/mcp-auth`'s validated
   OAuth-gate + SDK approach, but there's nothing left to build there for auth — Cognito's
   JWT authorizer already covers `/mcp` once it's added as a route, same as every other
   endpoint. Just add the route + real read-only tools wrapping `internal/store`.)
6. **Not started**: register the connector for real (manual Client ID, per the spike
   finding), verify end-to-end from phone
7. `spikes/mcp-auth/` can be deleted any time now — superseded by the real thing being wired
   in directly, not a dependency of any later step

## Deploy checklist (not done yet — needs explicit go-ahead, this locks out the live app
until the last step)
1. `cdk deploy` — creates the User Pool, wires the JWT authorizer onto every route. **The
   moment this lands, the live app requires a login and there is no user yet** — the app is
   inaccessible between this step and the next.
2. Create the one real user (Chris) via AWS CLI/console — `admin-create-user` +
   `admin-set-user-password` (permanent), since self-signup is disabled by design.
3. Log in from the live app, confirm plants/logs/settings all still work end-to-end.

## Cost
For a single user, this should land at $0/month for the User Pool itself — Cognito's free tier
covers a large number of MAUs (historically tens of thousands), so 1 admin-created user is nowhere
near it. Avoid Cognito's "Advanced Security Features" tier (adaptive auth, compromised-credential
checks) — that's priced per-MAU and adds cost for a personal single-user pool that doesn't need it.
The one place a small charge could actually show up: a **custom domain** for the Cognito Hosted UI
(so the login page isn't on a raw `*.auth.*.amazoncognito.com` domain) requires provisioning a
CloudFront distribution behind it — worth skipping unless it matters cosmetically, since the
default Cognito-hosted domain works fine functionally. Worth double-checking current AWS pricing
before committing, since tier structure/pricing has shifted more than once.

## Effort estimate
Smaller than full multi-user (no signup UI, no multi-tenant frontend), but still real: roughly
1-2 sessions, assuming the Step 1 spike doesn't surface a blocker.
