# Runbook

## Backend Start
1. Copy .env.example to .env and fill secrets.
2. Start PostgreSQL and create health_monitor DB.
3. Run: go run main.go

## Test
1. Unit + integration: go test ./...
2. Validate health endpoint: GET /health

## AI Failover Drill
1. Disable NIM key or endpoint.
2. Validate fallback to Gemini.
3. Disable Gemini and validate fallback to OpenRouter.

## Report Workflow Drill
1. Create reviewed report.
2. Approve report.
3. Verify async PDF job and generated patient/clinical files.
4. Download via /api/v1/reports/:id/pdf?copy=patient|clinical.
