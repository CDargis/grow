# Multi-User, Sharing, and Social Feed

## Status
Parked. Grow with Jane already has the social feed concept. Worth revisiting if the app evolves beyond personal use or a differentiated angle emerges.

## Background
The data model already has `userId` on every entity (plants, logs, environments). The only thing keeping this single-user is `USER_ID=default` hardcoded in the Lambda env. No schema migration needed to add auth.

---

## Feature 1 — Full Multi-User (prerequisite for everything else)

**What:** Each user has their own plants, environments, and logs. Login/signup UI.

**Approach:**
- AWS Cognito User Pool — fits the existing CDK/Lambda/DynamoDB stack cleanly
- Cognito JWT passed in `Authorization` header; Lambda validates and extracts `userId` from claims
- Replace `USER_ID=default` env var with the authenticated userId on every request
- All existing DynamoDB access patterns already filter by userId via GSIs

**CDK changes:** Cognito User Pool + App Client, Lambda authorizer or API Gateway JWT authorizer

**Frontend changes:** Login/signup screens, token storage, attach JWT to every API request, handle 401s

**Effort:** Largest single chunk of work — probably 2-3 sessions

---

## Feature 2 — Read-Only Sharing

**What:** Shareable link that lets someone view your garden without editing. No login required for the viewer.

**Approach:**
- A `shares` table: `{ shareId (PK), userId, scope (plant|garden), plantId?, createdAt, expiresAt? }`
- `GET /share/:shareId` returns read-only plant/log data for that share token
- Frontend route `/share/:shareId` renders a stripped-down read-only view (no edit/delete controls)
- Share link is opaque (e.g. `/share/01JXABC123`) — no brute-force guessing

**Effort:** Small — maybe one session after multi-user is in

---

## Feature 3 — Social Feed

**What:** A global feed of log entries across all users, shown as a growing community journal.

**Notes:**
- Grow with Jane already does this — not a differentiator as-is
- Could still build it for fun or with a twist (strain-specific feeds, environment-type feeds, etc.)
- Privacy model needs defining: are grows public by default or opt-in?
- A "follows" model vs. global feed changes the data model significantly

**Approach if built:**
- New `feed` GSI or table: log entries marked as public, queryable by timestamp across users
- Frontend: new feed tab, infinite scroll, tap to view that grower's plant (read-only share view)
- Opt-in public flag on plants or per log entry

**Effort:** Medium-large — needs multi-user + sharing first, plus privacy design decisions

---

## Build Order
1. Multi-user (Cognito auth)
2. Read-only sharing
3. Social feed (revisit if there's a differentiated angle vs. GWJ)
