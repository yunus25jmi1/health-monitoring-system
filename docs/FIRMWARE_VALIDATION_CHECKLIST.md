# Firmware Validation Checklist

## Bring-up Steps
1. Boot with USB power and verify serial startup logs.
2. Validate WiFi connect/disconnect/reconnect logs.
3. Validate queue restore count after reboot.

## Sensor Fault Injection
1. MAX30102 no finger -> spo2=0 behavior checked.
2. DS18B20 unplugged -> temp=-1.0 logged.
3. AD8232 leads off -> ecg_raw=-1 logged.
4. NIR ambient noise -> verify smoothing and bounded output.

## Transport Reliability
1. Disconnect WiFi and generate >5 telemetry events; ensure queue grows.
2. Reconnect WiFi and verify queued packets flush in order.
3. Force 5xx response from backend and verify exponential backoff.
4. Force 401/400 and verify packet drop behavior for permanent client errors.

## Contract
1. Confirm X-Device-Key header present.
2. Confirm payload excludes urgent field.
3. Confirm patient_id and device key match configured values.
