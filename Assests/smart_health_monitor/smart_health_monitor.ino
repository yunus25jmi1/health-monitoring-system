/*
 * ============================================================
 *   Smart Health Monitoring System v2.0
 *   Team: Md Yunus, Vishal Yadav, Ankur Sharma
 *   Hardware: ESP32 WROOM-32
 *   Sensors: MAX30102, DS18B20, AD8232, NIR Glucose Module
 * ============================================================
 *
 * LIBRARIES REQUIRED (install via Arduino IDE Library Manager):
 *   - MAX30105 by SparkFun Electronics
 *   - heartRate by SparkFun Electronics (bundled with MAX30105)
 *   - DallasTemperature by Miles Burton
 *   - OneWire by Paul Stoffregen
 *   - ArduinoJson by Benoit Blanchon
 *   - HTTPClient (built-in with ESP32 core)
 *   - WiFi (built-in with ESP32 core)
 */

// ============================================================
//  INCLUDES
// ============================================================
#include <Wire.h>
#include <WiFi.h>
#include <HTTPClient.h>
#include <ArduinoJson.h>
#include <Preferences.h>
#include "MAX30105.h"
#include "heartRate.h"
#include <OneWire.h>
#include <DallasTemperature.h>

// ============================================================
//  WiFi CREDENTIALS — change these
// ============================================================
const char* WIFI_SSID     = "Test";
const char* WIFI_PASSWORD = "25121964";

// ============================================================
//  BACKEND SERVER — change to your Golang server IP/URL
// ============================================================
const char* SERVER_URL = "https://health.yunus.eu.org/api/v1/readings";
const char* DEVICE_KEY = "Yunus@2512";
const int   PATIENT_ID = 2;

// ============================================================
//  GPIO PIN DEFINITIONS
// ============================================================

// DS18B20 Temperature Sensor
#define DS18B20_PIN     5       // GPIO5 — Data pin (with 4.7kΩ pull-up to 3.3V)

// AD8232 ECG Module
#define ECG_OUTPUT_PIN  34      // GPIO34 — Analog output (ADC)
#define ECG_LO_PLUS     32      // GPIO32 — Leads-off detection +
#define ECG_LO_MINUS    33      // GPIO33 — Leads-off detection -

// NIR Glucose Sensor (custom IR + photodiode)
#define NIR_SENSOR_PIN  35      // GPIO35 — Analog input from photodiode
#define NIR_LED_PIN     26      // GPIO26 — Controls 940nm IR LED (via transistor)

// Status LED (optional — shows system alive)
#define STATUS_LED_PIN  2       // GPIO2 — Built-in LED on most ESP32 boards

// ============================================================
//  SENSOR OBJECTS
// ============================================================
MAX30105 particleSensor;        // MAX30102 object (library uses MAX30105 name)
OneWire oneWire(DS18B20_PIN);
DallasTemperature tempSensor(&oneWire);

// ============================================================
//  HEART RATE CALCULATION VARIABLES (MAX30102)
// ============================================================
const byte  RATE_SIZE = 4;
byte        rates[RATE_SIZE];
byte        rateSpot = 0;
long        lastBeat = 0;
float       beatsPerMinute = 0;
int         beatAvg = 0;

// ============================================================
//  GLUCOSE CALIBRATION (adjust based on your sensor)
// ============================================================
// Raw ADC range: 0–4095 (12-bit)
// Calibrated glucose range: 40–400 mg/dL
// These are placeholder values — calibrate with a real glucometer
#define GLUCOSE_RAW_MIN   500
#define GLUCOSE_RAW_MAX   3500
#define GLUCOSE_MG_MIN    40
#define GLUCOSE_MG_MAX    400

// ============================================================
//  TIMING
// ============================================================
#define SEND_INTERVAL_MS  10000   // Send data to server every 10 seconds
unsigned long lastSendTime = 0;

#define HTTP_TIMEOUT_MS             8000
#define WIFI_RECONNECT_INTERVAL_MS  15000
#define RETRY_BASE_DELAY_MS         5000
#define RETRY_MAX_DELAY_MS          60000
#define MAX_BUFFERED_PACKETS        12

unsigned long lastWiFiReconnectAttempt = 0;
unsigned long nextRetryAt = 0;
unsigned long retryDelayMs = RETRY_BASE_DELAY_MS;

struct TelemetryPacket {
  int bpm;
  int spo2;
  float temp;
  int ecg;
  float glucose;
};

TelemetryPacket packetQueue[MAX_BUFFERED_PACKETS];
int queueHead = 0;
int queueTail = 0;
int queueCount = 0;
Preferences queuePrefs;

bool enqueuePacket(const TelemetryPacket& packet);
bool peekPacket(TelemetryPacket& packet);
void popPacket();
void flushQueuedPackets();
bool postPacket(const TelemetryPacket& packet);
void persistQueueState();
void loadQueueState();

// ============================================================
//  THRESHOLD FLAGS
// ============================================================
#define SPO2_LOW_THRESHOLD      94      // Below this = flag
#define BPM_HIGH_THRESHOLD      100     // Above this = flag
#define BPM_LOW_THRESHOLD       50      // Below this = flag
#define TEMP_HIGH_THRESHOLD     38.0    // Celsius
#define GLUCOSE_HIGH_THRESHOLD  180.0   // mg/dL — urgent flag
#define GLUCOSE_LOW_THRESHOLD   70.0    // mg/dL — hypoglycemia alert


// ============================================================
//  SETUP
// ============================================================
void setup() {
  Serial.begin(115200);
  delay(500);
  Serial.println("\n=== Smart Health Monitor Booting ===");

  // Status LED
  pinMode(STATUS_LED_PIN, OUTPUT);
  digitalWrite(STATUS_LED_PIN, LOW);

  // AD8232 leads-off detection pins
  pinMode(ECG_LO_PLUS,  INPUT);
  pinMode(ECG_LO_MINUS, INPUT);

  // NIR LED control pin
  pinMode(NIR_LED_PIN, OUTPUT);
  digitalWrite(NIR_LED_PIN, LOW);

  // ---- Connect to WiFi ----
  connectWiFi();

  // ---- Restore unsent telemetry queue ----
  loadQueueState();

  // ---- Init DS18B20 ----
  tempSensor.begin();
  Serial.println("[DS18B20] Temperature sensor initialized.");

  // ---- Init MAX30102 ----
  if (!particleSensor.begin(Wire, I2C_SPEED_FAST)) {
    Serial.println("[MAX30102] ERROR: Sensor not found. Check wiring!");
    // Non-fatal — loop will handle missing data
  } else {
    particleSensor.setup();
    particleSensor.setPulseAmplitudeRed(0x0A);   // Low power red LED
    particleSensor.setPulseAmplitudeGreen(0);     // Green LED off
    Serial.println("[MAX30102] Pulse oximeter initialized.");
  }

  Serial.println("=== Boot Complete. Monitoring started. ===\n");
  digitalWrite(STATUS_LED_PIN, HIGH);
}


// ============================================================
//  MAIN LOOP
// ============================================================
void loop() {

  // Keep WiFi healthy in long-running mode
  if (WiFi.status() != WL_CONNECTED && millis() - lastWiFiReconnectAttempt >= WIFI_RECONNECT_INTERVAL_MS) {
    lastWiFiReconnectAttempt = millis();
    connectWiFi();
  }

  // 1. Read heart rate from MAX30102
  readHeartRate();

  // 2. Every SEND_INTERVAL_MS, collect all sensors and POST to server
  if (millis() - lastSendTime >= SEND_INTERVAL_MS) {
    lastSendTime = millis();

    float bodyTemp   = readTemperature();
    int   spo2       = readSpO2();
    int   ecgValue   = readECG();
    float glucose    = readGlucose();
    bool  isUrgent   = checkThresholds(beatAvg, spo2, bodyTemp, glucose);

    printReadings(beatAvg, spo2, bodyTemp, ecgValue, glucose, isUrgent);
    sendToServer(beatAvg, spo2, bodyTemp, ecgValue, glucose, isUrgent);
  }

  // Try draining queued telemetry when network and retry window allow
  flushQueuedPackets();
}


// ============================================================
//  FUNCTION: Connect to WiFi
// ============================================================
void connectWiFi() {
  if (WiFi.status() == WL_CONNECTED) {
    return;
  }

  Serial.print("[WiFi] Connecting to ");
  Serial.print(WIFI_SSID);
  WiFi.begin(WIFI_SSID, WIFI_PASSWORD);
  int attempts = 0;
  while (WiFi.status() != WL_CONNECTED && attempts < 20) {
    delay(500);
    Serial.print(".");
    attempts++;
  }
  if (WiFi.status() == WL_CONNECTED) {
    Serial.println("\n[WiFi] Connected! IP: " + WiFi.localIP().toString());
  } else {
    Serial.println("\n[WiFi] FAILED. Running offline — data will not be sent.");
  }
}


// ============================================================
//  FUNCTION: Read Heart Rate from MAX30102
//  Call this in every loop iteration — it's interrupt-driven
// ============================================================
void readHeartRate() {
  long irValue = particleSensor.getIR();

  if (checkForBeat(irValue)) {
    long delta = millis() - lastBeat;
    lastBeat   = millis();

    beatsPerMinute = 60.0 / (delta / 1000.0);

    if (beatsPerMinute < 255 && beatsPerMinute > 20) {
      rates[rateSpot++] = (byte)beatsPerMinute;
      rateSpot %= RATE_SIZE;

      // Average the last RATE_SIZE readings
      beatAvg = 0;
      for (byte x = 0; x < RATE_SIZE; x++) beatAvg += rates[x];
      beatAvg /= RATE_SIZE;
    }
  }
}


// ============================================================
//  FUNCTION: Read SpO2 from MAX30102
//  Returns estimated SpO2 percentage (simplified calculation)
// ============================================================
int readSpO2() {
  long redValue = particleSensor.getRed();
  long irValue  = particleSensor.getIR();

  if (irValue < 50000) {
    // No finger detected
    return 0;
  }

  // Simplified R-value ratio method
  // For accurate SpO2, use SparkFun's spo2_algorithm library
  float ratio = (float)redValue / (float)irValue;
  int spo2 = (int)(110.0 - 25.0 * ratio);
  spo2 = constrain(spo2, 70, 100);
  return spo2;
}


// ============================================================
//  FUNCTION: Read Body Temperature from DS18B20
// ============================================================
float readTemperature() {
  tempSensor.requestTemperatures();
  float tempC = tempSensor.getTempCByIndex(0);
  if (tempC == DEVICE_DISCONNECTED_C) {
    Serial.println("[DS18B20] ERROR: Sensor disconnected!");
    return -1.0;
  }
  return tempC;
}


// ============================================================
//  FUNCTION: Read ECG analog value from AD8232
//  Returns raw ADC value (0–4095)
//  Leads-off detection: if LO+ or LO- are HIGH, electrodes are off
// ============================================================
int readECG() {
  if (digitalRead(ECG_LO_PLUS) == HIGH || digitalRead(ECG_LO_MINUS) == HIGH) {
    Serial.println("[AD8232] Leads off! Check electrode placement.");
    return -1;   // Sentinel value: leads disconnected
  }
  return analogRead(ECG_OUTPUT_PIN);
}


// ============================================================
//  FUNCTION: Read Glucose via NIR sensor
//  Returns estimated glucose in mg/dL
// ============================================================
float readGlucose() {
  // Turn IR LED on
  digitalWrite(NIR_LED_PIN, HIGH);
  delay(10);  // Stabilization time

  // Take multiple readings and average for noise reduction
  long rawSum = 0;
  for (int i = 0; i < 10; i++) {
    rawSum += analogRead(NIR_SENSOR_PIN);
    delay(5);
  }
  long rawAvg = rawSum / 10;

  // Turn IR LED off
  digitalWrite(NIR_LED_PIN, LOW);

  // Map raw ADC value to glucose range
  float glucose = map(rawAvg, GLUCOSE_RAW_MIN, GLUCOSE_RAW_MAX,
                      GLUCOSE_MG_MIN, GLUCOSE_MG_MAX);
  glucose = constrain(glucose, GLUCOSE_MG_MIN, GLUCOSE_MG_MAX);
  return glucose;
}


// ============================================================
//  FUNCTION: Check Thresholds — returns true if urgent
// ============================================================
bool checkThresholds(int bpm, int spo2, float temp, float glucose) {
  bool urgent = false;

  if (spo2 > 0 && spo2 < SPO2_LOW_THRESHOLD) {
    Serial.println("[ALERT] SpO2 critically low!");
    urgent = true;
  }
  if (bpm > BPM_HIGH_THRESHOLD) {
    Serial.println("[ALERT] Heart rate too high!");
    urgent = true;
  }
  if (bpm > 0 && bpm < BPM_LOW_THRESHOLD) {
    Serial.println("[ALERT] Heart rate too low!");
    urgent = true;
  }
  if (temp > TEMP_HIGH_THRESHOLD) {
    Serial.println("[ALERT] Fever detected!");
    urgent = true;
  }
  if (glucose > GLUCOSE_HIGH_THRESHOLD) {
    Serial.println("[ALERT] Blood sugar high — URGENT FLAG!");
    urgent = true;
  }
  if (glucose > 0 && glucose < GLUCOSE_LOW_THRESHOLD && bpm > BPM_HIGH_THRESHOLD) {
    Serial.println("[ALERT] Hypoglycemic event suspected!");
    urgent = true;
  }

  return urgent;
}


// ============================================================
//  FUNCTION: Print readings to Serial Monitor
// ============================================================
void printReadings(int bpm, int spo2, float temp, int ecg, float glucose, bool urgent) {
  Serial.println("\n--- Sensor Readings ---");
  Serial.print("Heart Rate (BPM): "); Serial.println(bpm);
  Serial.print("SpO2 (%):         "); Serial.println(spo2);
  Serial.print("Body Temp (C):    "); Serial.println(temp, 2);
  Serial.print("ECG (raw ADC):    "); Serial.println(ecg);
  Serial.print("Glucose (mg/dL):  "); Serial.println(glucose, 1);
  Serial.print("URGENT FLAG:      "); Serial.println(urgent ? "YES" : "NO");
  Serial.println("-----------------------\n");
}


// ============================================================
//  FUNCTION: Queue telemetry and trigger immediate flush attempt
// ============================================================
void sendToServer(int bpm, int spo2, float temp, int ecg, float glucose, bool urgent) {
  TelemetryPacket packet;
  packet.bpm = bpm;
  packet.spo2 = spo2;
  packet.temp = temp;
  packet.ecg = ecg;
  packet.glucose = glucose;

  if (!enqueuePacket(packet)) {
    Serial.println("[QUEUE] Full buffer. Dropping oldest packet.");
    popPacket();
    enqueuePacket(packet);
  }

  flushQueuedPackets();
}

bool enqueuePacket(const TelemetryPacket& packet) {
  if (queueCount >= MAX_BUFFERED_PACKETS) {
    return false;
  }

  packetQueue[queueTail] = packet;
  queueTail = (queueTail + 1) % MAX_BUFFERED_PACKETS;
  queueCount++;
  persistQueueState();
  return true;
}

bool peekPacket(TelemetryPacket& packet) {
  if (queueCount == 0) {
    return false;
  }

  packet = packetQueue[queueHead];
  return true;
}

void popPacket() {
  if (queueCount == 0) {
    return;
  }

  queueHead = (queueHead + 1) % MAX_BUFFERED_PACKETS;
  queueCount--;
  persistQueueState();
}

void flushQueuedPackets() {
  if (queueCount == 0) {
    return;
  }

  if (WiFi.status() != WL_CONNECTED) {
    return;
  }

  if (nextRetryAt > 0 && millis() < nextRetryAt) {
    return;
  }

  while (queueCount > 0) {
    TelemetryPacket packet;
    if (!peekPacket(packet)) {
      return;
    }

    bool sent = postPacket(packet);
    if (sent) {
      popPacket();
      retryDelayMs = RETRY_BASE_DELAY_MS;
      nextRetryAt = 0;
      continue;
    }

    nextRetryAt = millis() + retryDelayMs;
    retryDelayMs = min(retryDelayMs * 2, (unsigned long)RETRY_MAX_DELAY_MS);
    Serial.println("[HTTP] Send failed, backing off retry.");
    return;
  }
}

bool postPacket(const TelemetryPacket& packet) {
  if (WiFi.status() != WL_CONNECTED) {
    Serial.println("[HTTP] No WiFi — packet stays queued.");
    return false;
  }

  HTTPClient http;
  http.setTimeout(HTTP_TIMEOUT_MS);
  http.begin(SERVER_URL);
  http.addHeader("Content-Type", "application/json");
  http.addHeader("X-Device-Key", DEVICE_KEY);

  // Build JSON payload
  StaticJsonDocument<256> doc;
  doc["patient_id"]    = PATIENT_ID;
  doc["bpm"]           = packet.bpm;
  doc["spo2"]          = packet.spo2;
  doc["temp"]          = packet.temp;
  doc["ecg_raw"]       = packet.ecg;
  doc["glucose_level"] = packet.glucose;

  String payload;
  serializeJson(doc, payload);

  Serial.println("[HTTP] Sending: " + payload);

  int responseCode = http.POST(payload);
  if (responseCode >= 200 && responseCode < 300) {
    Serial.println("[HTTP] Response code: " + String(responseCode));
    Serial.println("[HTTP] Body: " + http.getString());
    http.end();
    return true;
  }

  if (responseCode >= 400 && responseCode < 500 && responseCode != 429) {
    // Permanent request errors should not be retried indefinitely.
    Serial.println("[HTTP] Permanent error, dropping packet. Code: " + String(responseCode));
    Serial.println("[HTTP] Body: " + http.getString());
    http.end();
    return true;
  }

  if (responseCode > 0) {
    Serial.println("[HTTP] Retryable response code: " + String(responseCode));
  } else {
    Serial.println("[HTTP] POST failed. Error: " + http.errorToString(responseCode));
  }

  http.end();
  return false;
}

void persistQueueState() {
  queuePrefs.begin("telemetry", false);
  queuePrefs.putInt("head", queueHead);
  queuePrefs.putInt("tail", queueTail);
  queuePrefs.putInt("count", queueCount);
  queuePrefs.putBytes("packets", packetQueue, sizeof(packetQueue));
  queuePrefs.end();
}

void loadQueueState() {
  queuePrefs.begin("telemetry", true);
  int savedCount = queuePrefs.getInt("count", 0);
  if (savedCount < 0 || savedCount > MAX_BUFFERED_PACKETS) {
    queuePrefs.end();
    queueHead = 0;
    queueTail = 0;
    queueCount = 0;
    return;
  }

  int savedHead = queuePrefs.getInt("head", 0);
  int savedTail = queuePrefs.getInt("tail", 0);
  size_t expected = sizeof(packetQueue);
  size_t copied = queuePrefs.getBytes("packets", packetQueue, expected);
  queuePrefs.end();

  if (copied != expected) {
    queueHead = 0;
    queueTail = 0;
    queueCount = 0;
    return;
  }

  if (savedHead < 0 || savedHead >= MAX_BUFFERED_PACKETS || savedTail < 0 || savedTail >= MAX_BUFFERED_PACKETS) {
    queueHead = 0;
    queueTail = 0;
    queueCount = 0;
    return;
  }

  queueHead = savedHead;
  queueTail = savedTail;
  queueCount = savedCount;
  Serial.println("[QUEUE] Restored buffered telemetry count: " + String(queueCount));
}
