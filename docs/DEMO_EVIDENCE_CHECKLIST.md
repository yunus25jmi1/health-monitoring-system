# Demo Evidence Checklist

Capture and store:
1. API request/response logs for auth, readings, reports, and PDF download.
2. DB snapshots for readings, reports, async_jobs.
3. AI provider fallback logs (NIM -> Gemini -> OpenRouter scenario).
4. Generated PDF artifacts (patient + clinical).
5. Firmware serial logs showing queueing, retry, and successful flush.
6. Final go test ./... output.
7. Concurrency test output for duplicate approval race.
