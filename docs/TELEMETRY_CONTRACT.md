# Telemetry Contract

Endpoint: POST /api/v1/readings

Required headers:
- Content-Type: application/json
- X-Device-Key: <device secret>

Request JSON:
- patient_id: uint
- bpm: int
- spo2: int
- temp: float
- ecg_raw: int
- glucose_level: float

Sensor sentinel policy:
- ECG leads off: ecg_raw = -1
- DS18B20 disconnected: temp = -1.0 (firmware), backend should reject out-of-range values in production mode
- No finger for SpO2: spo2 = 0

Backend ownership:
- is_urgent is not accepted from device payload.
- is_urgent is computed server-side using threshold rules.

Retry policy (firmware):
- Queue unsent packets in RAM and persist queue metadata/data in ESP32 Preferences.
- Retry with exponential backoff for network/5xx/429 failures.
- Drop only permanently invalid 4xx packets (except 429).
