# Requirement Decisions

1. Canonical PRD source is Assests/Backend_PRD_SmartHealth.md.
2. AI provider failover order is fixed and deterministic:
   - Primary: NVIDIA NIM
   - Secondary: Gemini official API
   - Tertiary: OpenRouter
3. Firmware sends only sensor payload and device key; urgency is computed by backend threshold engine.
4. Report approval requires reviewed status and is race-safe at DB update time.
5. Async AI and PDF tasks use durable DB-backed job records for retries and auditability.
6. Glucose readings are treated as prototype estimates, not clinical-grade diagnostics.
