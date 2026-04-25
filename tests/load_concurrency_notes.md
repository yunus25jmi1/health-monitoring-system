# Concurrency and Load Test Notes

Planned scripts:
1. Burst telemetry load (100-500 requests/min) against /api/v1/readings with valid device key.
2. Parallel report approvals to validate exactly-one success.
3. AI provider outage simulation under parallel urgent events.
4. DB pool saturation check against max open/idle settings.

Record results in docs/DEMO_EVIDENCE_CHECKLIST.md.
