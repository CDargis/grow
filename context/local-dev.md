# Local Development

## Prerequisites

- Go 1.24+
- Node 18+
- AWS credentials configured (`aws configure` or a named profile)

## Running locally

The backend detects `LOCAL=1` and starts a plain HTTP server on port 3000 instead of the Lambda entrypoint. It still hits real AWS (DynamoDB, S3) using your local credentials.

**Terminal 1 — backend**

```bash
cd backend

LOCAL=1 \
PLANTS_TABLE=grow-plants \
ENVIRONMENTS_TABLE=grow-environments \
LOGS_TABLE=grow-logs \
LOGS_DATE_GSI=user-date-index \
LOGS_LOGTYPE_DATE_GSI=user-logtype-date-index \
SETTINGS_TABLE=grow-settings \
MEDIA_BUCKET=grow-media \
USER_ID=default \
USER_POOL_ID=<from `cdk deploy` output "UserPoolId">      \
USER_POOL_CLIENT_ID=<from output "UserPoolClientId">      \
AWS_REGION=<your region> \
go run ./cmd/api
```

If using a named AWS profile, prefix with `AWS_PROFILE=yourprofile`.

**Terminal 2 — frontend**

```bash
cd frontend
npm run dev
```

Vite proxies `/api/*` → `http://localhost:3000`.

## Auth in local dev

`LOCAL=1` mode has no API Gateway in front of it, so the backend never checks the JWT locally —
`app.userID(r)` falls straight through to `USER_ID=default`, same as before Cognito existed.

The **frontend** doesn't know it's talking to a local backend, though: `AuthProvider` always
redirects to the real Cognito Hosted UI if there's no valid session, and it needs
`USER_POOL_ID`/`USER_POOL_CLIENT_ID` (above) to build that login URL via `/api/auth-config`. So
local frontend dev still requires logging in once, for real, against the deployed User Pool
(there's no way to mock Cognito locally) — the resulting token gets sent to your local backend,
which just ignores it. Log in once per browser session; the token persists in `localStorage`
until it expires.

## Notes

- `make run` in the Makefile has the same command but requires `make` (not available on Windows/MINGW64 by default)
- No local DynamoDB emulator needed — all reads/writes go to the real AWS tables
- The `user-logtype-date-index` GSI (added 2026-07-04) only indexes logs written after deployment; older logs won't appear in the Activity sort view
