# End-to-End Scenario Matrix

| Scenario | Steps | Expected |
|---|---|---|
| Normal reading | Device POST valid non-urgent payload | Reading stored, is_urgent=false |
| Urgent threshold | Device POST payload breaching threshold | Reading stored, async ai_draft job enqueued |
| Doctor review | PATCH report status to reviewed | Status updated with transition guard |
| Doctor approve | POST approve on reviewed report | Status approved, async pdf job enqueued |
| PDF download | GET /reports/:id/pdf | Streams selected copy if file exists |
| Failover | NIM down then Gemini down | OpenRouter serves draft |
| Retry | AI/PDF job fails transiently | Async job attempts increment and next_run_at backoff |
