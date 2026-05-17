# grow — Overview

A personal cannabis grow journal web app. Track plants through their full lifecycle, log daily activity (watering, feeding, training, etc.), and manage multiple grow environments.

## Status
Scaffolding in progress. Infrastructure and frontend/backend shells being built.

## Stack
- **Frontend**: React + TypeScript + Vite, deployed to S3/CloudFront
- **Backend**: Go on AWS Lambda (provided.al2023, ARM64), API Gateway HTTP API v2
- **Database**: DynamoDB (PAY_PER_REQUEST)
- **Infrastructure**: AWS CDK (C#), self-mutating CodePipeline
- **Domain**: grow.chrisdargis.com

## Repo
github.com/CDargis/grow
