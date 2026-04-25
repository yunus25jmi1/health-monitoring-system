# Traceability Matrix

| PRD Requirement | Implementation Evidence | Test Evidence | Status |
|---|---|---|---|
| POST /readings with device auth | handlers/reading_handler.go, middleware/device_auth.go, routes/router.go | routes/router_integration_test.go, middleware/device_auth_test.go | Done |
| Clinical threshold checks | services/reading_service.go | services/reading_service_test.go | Done |
| JWT auth and role-based access | middleware/jwt_auth.go, handlers/auth_handler.go | middleware/jwt_auth_test.go, routes/router_integration_test.go | Done |
| Report lifecycle pending->reviewed->approved | services/report_lifecycle.go, handlers/report_handler.go | services/report_lifecycle_test.go, handlers/report_handler_test.go | Done |
| AI draft with provider fallback | services/ai_service.go | services/ai_service_test.go | Done |
| Dual-copy PDF generation | services/pdf_service.go, handlers/report_handler.go | services/pdf_service_test.go, handlers/report_handler_test.go | Done |
| Async durable retry for AI/PDF | models/async_job.go, services/async_job_service.go, main.go | services/async_job_service_test.go | Done |
| Firmware retry/backoff + queue | Assests/smart_health_monitor.ino | Manual hardware validation checklist | In Progress |
| End-to-end validation evidence capture | docs/DEMO_EVIDENCE_CHECKLIST.md | Pending run artifacts | Pending |
