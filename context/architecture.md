# grow — Architecture

## Components

### Frontend
- React SPA, Vite build, Tailwind CSS (dark theme)
- Deployed to S3 (`grow-site` bucket), served via CloudFront
- Route 53 A record → CloudFront → S3 (OAC)
- ACM certificate for grow.chrisdargis.com

### Backend
- Single Go Lambda (`grow-api`), ARM64, provided.al2023
- API Gateway HTTP API v2 with `/{proxy+}` route
- CloudFront `/api/*` behavior → API Gateway (cache disabled)
- Pre-signed S3 URLs for photo uploads/reads (no CloudFront for media)

### Storage
- DynamoDB: `grow-plants`, `grow-environments`, `grow-logs` tables
- S3 `grow-media` bucket: plant/environment/log photos (private, pre-signed URL access)

### Infrastructure
- CDK (C#) in `infra/`
- Two stacks: `GrowPipelineStack` → `GrowStack` (via CDK Pipelines)
- Self-mutating CodePipeline sourced from GitHub (CDargis/grow, main)
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
