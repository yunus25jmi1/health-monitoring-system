s

**PRODUCT REQUIREMENTS DOCUMENT**

**Golang Backend Architecture**

Smart Health Monitoring System v2.0

  ----------------------------------- -----------------------------------
  Project                             Smart Health Monitor

  Version                             1.0 (Draft)

  Date                                April 2026

  Backend Stack                       Golang + Gin + PostgreSQL

  Team                                Md Yunus, Vishal Yadav, Ankur
                                      Sharma

  Institution                         GNDU Amritsar
  ----------------------------------- -----------------------------------

# **1. Overview**

## **1.1 Purpose**

This document defines the complete product requirements for the Golang
backend of the Smart Health Monitoring System. It covers the REST API
design, data models, business logic, Doctor-in-the-Loop (DIL) workflow,
AI agent integration, and deployment expectations for the prototype
demonstration.

## **1.2 Background**

The system collects real-time vitals from an ESP32 hardware node ---
including heart rate (BPM), blood oxygen (SpO2), body temperature, ECG
signals, and estimated glucose --- and transmits them via HTTP POST to a
Golang backend. The backend validates the data, applies clinical
thresholds, flags anomalies, and invokes an LLM-based AI agent to draft
medical reports that are reviewed and approved by a doctor before
reaching the patient.

## **1.3 Goals**

-   Receive, validate, and persist real-time sensor telemetry from ESP32
    > devices.

```{=html}
<!-- -->
```
-   Implement clinical threshold checks and auto-flag urgent patient
    > readings.

-   Integrate an LLM API to generate structured Markdown health report
    > drafts.

-   Expose a Doctor Portal API for reviewing, editing, and approving
    > AI-generated reports.

-   Generate dual-copy PDFs upon doctor approval --- one for the
    > patient, one for records.

-   Deliver all APIs within a 3-day prototype build timeline.

## **1.4 Non-Goals**

-   Real-time ECG waveform streaming (deferred to future scope).

-   Mobile app frontend (out of scope for v1.0 prototype).

-   HIPAA/clinical compliance certification (prototype only).

-   Multi-hospital or multi-tenant deployment.

# **2. System Architecture**

## **2.1 High-Level Layers**

The backend is structured into four distinct layers, each with a clearly
separated responsibility:

  ---------------------------------------------------------------------------
  **Layer**      **Technology**     **Responsibility**   **Components**
  -------------- ------------------ -------------------- --------------------
  Transport      HTTP / JSON        Receive telemetry    Gin Router,
                                    from ESP32 over WiFi Middleware

  Business Logic Golang             Threshold checks,    Service layer,
                                    anomaly detection,   threshold engine
                                    report lifecycle     

  Intelligence   LLM API (Claude /  Markdown report      AI agent client,
                 GPT)               drafting from sensor prompt builder
                                    summary              

  Persistence    PostgreSQL         Store readings,      GORM ORM, migrations
                                    reports, patients,   
                                    doctors              
  ---------------------------------------------------------------------------

## **2.2 Tech Stack**

  -----------------------------------------------------------------------
  **Component**     **Choice**        **Reason**
  ----------------- ----------------- -----------------------------------
  Language          Go 1.22+          High concurrency, low latency,
                                      strong stdlib

  HTTP Router       Gin v1.9          Minimal overhead, fast routing,
                                      middleware support

  ORM               GORM v2           Clean schema migration, auto-create
                                      tables

  Database          PostgreSQL 15     Time-series vitals, JSONB for
                                      flexible report notes

  PDF               go-pdf / fpdf     Server-side dual-copy PDF
                                      generation

  AI Agent          HTTP client to    Async report drafting via REST
                    LLM API           

  Config            godotenv / env    12-factor app config management
                    vars              

  Auth              JWT (golang-jwt)  Doctor/Patient role-based access
                                      control
  -----------------------------------------------------------------------

## **2.3 Directory Structure**

The project follows standard Go project layout conventions:

> /smart-health-backend
>
> ├── main.go \# Entry point, server bootstrap
>
> ├── .env \# Config (DB URL, LLM API key, port)
>
> ├── /config \# Env loading, DB connection
>
> ├── /routes \# Gin router registration
>
> ├── /handlers \# HTTP request handlers (thin layer)
>
> ├── /services \# Business logic (threshold, AI, PDF)
>
> ├── /models \# GORM model structs
>
> ├── /middleware \# Auth JWT, CORS, logging
>
> └── /migrations \# DB schema SQL files

# **3. Data Models**

## **3.1 readings Table**

Stores every telemetry payload sent by the ESP32. Records are
append-only --- no updates after insert.

  ----------------------------------------------------------------------------
  **Column**      **Type**       **Constraints**   **Description**
  --------------- -------------- ----------------- ---------------------------
  id              SERIAL         PRIMARY KEY       Auto-increment row ID

  patient_id      INT            NOT NULL, FK →    Owning patient
                                 patients          

  bpm             INT            NOT NULL          Heart rate in beats/min

  spo2            INT            NOT NULL          Blood oxygen saturation %

  temp            DECIMAL(5,2)   NOT NULL          Body temperature in Celsius

  ecg_raw         INT            NULLABLE          Raw ADC value from AD8232;
                                                   -1 = leads off

  glucose_level   DECIMAL(6,2)   NULLABLE          Estimated blood glucose
                                                   mg/dL

  is_urgent       BOOLEAN        DEFAULT false     Set true if any threshold
                                                   breached

  recorded_at     TIMESTAMPTZ    DEFAULT NOW()     Timestamp of reading
  ----------------------------------------------------------------------------

## **3.2 reports Table**

Stores AI-generated draft reports and the doctor\'s finalized notes.
Status progresses through a defined lifecycle: pending → reviewed →
approved.

  -------------------------------------------------------------------------
  **Column**    **Type**      **Constraints**   **Description**
  ------------- ------------- ----------------- ---------------------------
  id            SERIAL        PRIMARY KEY       Auto-increment ID

  patient_id    INT           NOT NULL, FK →    Patient the report belongs
                              patients          to

  doctor_id     INT           NULLABLE, FK →    Assigned reviewing doctor
                              doctors           

  ai_draft      TEXT          NOT NULL          Markdown report generated
                                                by LLM

  final_notes   TEXT          NULLABLE          Doctor edits/annotations

  status        VARCHAR(20)   DEFAULT           pending \| reviewed \|
                              \'pending\'       approved

  pdf_path      TEXT          NULLABLE          File path of generated PDF

  created_at    TIMESTAMPTZ   DEFAULT NOW()     Report creation timestamp

  approved_at   TIMESTAMPTZ   NULLABLE          Timestamp of doctor
                                                approval
  -------------------------------------------------------------------------

## 

## **3.3 patients & doctors Tables**

Standard identity tables --- kept minimal for prototype scope.

  ------------------------------------------------------------------------
  **Column**    **Type**       **Notes**
  ------------- -------------- -------------------------------------------
  id            SERIAL         PRIMARY KEY

  name          VARCHAR(100)   Full name

  email         VARCHAR(150)   UNIQUE --- used as login identifier

  password      TEXT           Bcrypt hashed --- never store plaintext

  role          VARCHAR(20)    patient \| doctor --- drives RBAC
                               middleware

  created_at    TIMESTAMPTZ    DEFAULT NOW()
  ------------------------------------------------------------------------

# **4. API Specification**

All endpoints use JSON. Base path: /api/v1. Authorization header: Bearer
\<JWT token\> for protected routes.

## **4.1 Telemetry Endpoints**

  -----------------------------------------------------------------------------------------------
  **Method**   **Route**                             **Auth**      **Description**
  ------------ ------------------------------------- ------------- ------------------------------
  POST         /api/v1/readings                      Device key    Receive sensor payload from
                                                                   ESP32; run threshold check;
                                                                   trigger AI draft if urgent

  GET          /api/v1/readings/:patient_id          Doctor/JWT    Fetch all readings for a
                                                                   patient, newest first

  GET          /api/v1/readings/latest/:patient_id   Doctor/JWT    Fetch most recent single
                                                                   reading for live dashboard
  -----------------------------------------------------------------------------------------------

### **POST /api/v1/readings --- Request Body**

> { \"patient_id\": 1, \"bpm\": 92, \"spo2\": 97, \"temp\": 36.8,
>
> \"ecg_raw\": 2048, \"glucose_level\": 145.5, \"urgent\": false }

### **POST /api/v1/readings --- Response (201 Created)**

> { \"id\": 204, \"is_urgent\": false, \"recorded_at\":
> \"2026-04-20T10:32:00Z\",
>
> \"message\": \"Reading stored successfully\" }

## **4.2 Report Endpoints (Doctor-in-the-Loop)**

  -------------------------------------------------------------------------------------------------
  **Method**   **Route**                             **Auth**         **Description**
  ------------ ------------------------------------- ---------------- -----------------------------
  GET          /api/v1/reports/pending               Doctor           List all reports with
                                                                      status=pending, sorted by
                                                                      urgency

  GET          /api/v1/reports/:id                   Doctor           Fetch single report with AI
                                                                      draft, readings summary, and
                                                                      charts data

  PATCH        /api/v1/reports/:id                   Doctor           Update final_notes and/or
                                                                      status (reviewed \| approved)

  POST         /api/v1/reports/:id/approve           Doctor           Approve report: set
                                                                      status=approved, trigger PDF
                                                                      generation, timestamp

  GET          /api/v1/reports/:id/pdf               Doctor/Patient   Stream the generated
                                                                      dual-copy PDF for download

  GET          /api/v1/reports/patient/:patient_id   Doctor           All reports for a specific
                                                                      patient
  -------------------------------------------------------------------------------------------------

### **PATCH /api/v1/reports/:id --- Request Body**

> { \"final_notes\": \"Patient shows signs of controlled
> hyperglycemia.\",
>
> \"status\": \"reviewed\" }

## **4.3 Auth Endpoints**

  --------------------------------------------------------------------------------
  **Method**   **Route**               **Auth**   **Description**
  ------------ ----------------------- ---------- --------------------------------
  POST         /api/v1/auth/login      Public     Email + password login; returns
                                                  JWT with role claim

  POST         /api/v1/auth/register   Public     Create patient/doctor account
                                                  (restricted in production)

  POST         /api/v1/auth/refresh    JWT        Refresh access token before
                                                  expiry
  --------------------------------------------------------------------------------

# **5. Business Logic**

## **5.1 Threshold Engine**

Every incoming reading is evaluated against the following clinical
thresholds immediately after persistence. If any condition is true,
is_urgent is set to true on the reading record.

  -------------------------------------------------------------------------
  **Metric**       **Low Threshold**  **High Threshold** **Condition**
  ---------------- ------------------ ------------------ ------------------
  Heart Rate (BPM) \< 50 BPM          \> 100 BPM         Either triggers
                                                         urgent flag

  SpO2             \< 94%             N/A                Critical oxygen
                                                         --- immediate flag

  Body Temperature N/A                \> 38.0°C          Fever detection

  Glucose Level    \< 70 mg/dL        \> 180 mg/dL       Hypoglycemia or
                                                         hyperglycemia flag

  Glucose + HR     Glucose \< 70 AND  ---                Hypoglycemic event
  combo            HR \> 100                             suspected ---
                                                         highest priority
  -------------------------------------------------------------------------

## **5.2 AI Report Drafting**

When is_urgent = true, or automatically every 24 hours for each patient,
the backend triggers an AI report draft via the LLM API. This is done
asynchronously --- a goroutine is spawned so the POST /readings response
is not delayed.

### **Prompt Construction**

The service builds a structured prompt including:

-   Patient ID, age, and any known conditions (from patients table).

-   Aggregated 24-hour readings: min/max/avg for each vital sign.

-   Count of urgent flags and specific breached thresholds.

-   Instruction to output a Markdown report with sections: Summary,
    > Vitals Analysis, Glucose Trend, Clinical Observations, and
    > Recommended Actions.

### **AI Integration Flow**

  -----------------------------------------------------------------------------
  **Step**   **Action**           **Detail**
  ---------- -------------------- ---------------------------------------------
  1          Aggregate data       Query last 24h readings for patient from
                                  PostgreSQL

  2          Build prompt         Inject structured vitals summary + clinical
                                  instructions into system prompt

  3          Call LLM API         POST to /v1/messages with max_tokens=1000,
                                  model=claude-sonnet

  4          Parse response       Extract Markdown text from content\[0\].text

  5          Store draft          INSERT into reports with status=pending,
                                  ai_draft=\<markdown\>

  6          Notify               Log to console; future: send email/push
                                  notification to assigned doctor
  -----------------------------------------------------------------------------

## **5.3 PDF Generation**

On POST /api/v1/reports/:id/approve, the service generates two PDF
copies:

-   Patient Copy --- contains vitals summary, AI draft + doctor notes,
    > approval timestamp, doctor name.

-   Clinical Record Copy --- identical content with a CONFIDENTIAL
    > CLINICAL RECORD watermark.

PDFs are saved to /storage/reports/{report_id}\_patient.pdf and
/storage/reports/{report_id}\_clinical.pdf. The pdf_path field in the
reports table stores the base path.

## **5.4 Report Status Lifecycle**

Reports follow a strict one-way status progression:

  -----------------------------------------------------------------------
  **From Status**   **To Status**     **Trigger**
  ----------------- ----------------- -----------------------------------
  (none)            pending           AI draft created after threshold
                                      breach or scheduled 24h cycle

  pending           reviewed          Doctor views report and makes edits
                                      via PATCH /reports/:id

  reviewed          approved          Doctor clicks Approve via POST
                                      /reports/:id/approve

  approved          (terminal)        No further state changes --- PDF
                                      generated and locked
  -----------------------------------------------------------------------

# **6. Middleware & Security**

  ------------------------------------------------------------------------
  **Middleware**   **Purpose**                 **Implementation Notes**
  ---------------- --------------------------- ---------------------------
  JWT Auth         Verify Bearer token on      golang-jwt/jwt v5; roles:
                   protected routes            doctor \| patient \| device

  CORS             Allow ESP32 + frontend      gin-contrib/cors; restrict
                   origins                     to known origins in
                                               production

  Request Logger   Log method, path, latency,  Gin default logger or
                   status code                 zerolog for structured logs

  Rate Limiter     Prevent sensor flooding     golang.org/x/time/rate or
                   (\>100 req/min)             custom goroutine bucket

  Panic Recovery   Recover from handler        gin.Recovery() middleware
                   panics, return 500          --- always enabled

  Device Auth      Validate ESP32 device       Static key in header
                   secret key on /readings     X-Device-Key; checked
                                               before JWT
  ------------------------------------------------------------------------

## **6.1 Security Requirements**

-   All doctor and patient passwords must be hashed with bcrypt (cost
    > factor 12) before storage.

-   JWT tokens expire after 8 hours. Refresh tokens valid for 7 days.

-   Device API key must be set via environment variable --- never
    > hardcoded.

-   Database credentials, LLM API key, and JWT secret must be loaded
    > from .env --- not committed to Git.

-   SQL queries must use GORM\'s parameterized queries only --- no raw
    > string interpolation.

# **7. Concurrency Model**

Golang\'s goroutine model is used to keep the telemetry POST endpoint
fast. The AI report drafting and PDF generation are the two expensive
operations and must never block the response path.

  ---------------------------------------------------------------------------
  **Operation**           **Mode**         **Notes**
  ----------------------- ---------------- ----------------------------------
  Persist reading to DB   Synchronous      Must complete before responding
                                           201 to ESP32

  Threshold check         Synchronous      Fast in-memory compare --- no DB
                                           call needed

  AI report draft (LLM    Async goroutine  go
  call)                                    aiService.DraftReport(patientID)
                                           --- fire and forget

  PDF generation on       Async goroutine  go pdfService.Generate(reportID)
  approval                                 --- triggered after PATCH

  Email/notification      Async goroutine  Deferred for future implementation
  dispatch                                 
  ---------------------------------------------------------------------------

Database connection pool is managed via GORM\'s built-in pool. For the
prototype, set SetMaxOpenConns(10) and SetMaxIdleConns(5) to prevent
resource exhaustion on the deployment machine.

# **8. 3-Day Implementation Plan**

## **Day 1 --- Foundation (Hardware + Pipe)**

Goal: ESP32 is posting live JSON to a running Golang server and raw data
is being stored in PostgreSQL.

  -----------------------------------------------------------------------
  **Time**   **Task**                            **Deliverable**
  ---------- ----------------------------------- ------------------------
  AM         Project setup: Go module, Gin,      Server boots, connects
             GORM, .env, DB connection           to PostgreSQL

  AM         Create models: Patient, Reading,    GORM AutoMigrate creates
             Report, Doctor                      all tables

  PM         Implement POST /api/v1/readings     ESP32 can POST and get
             handler + device key middleware     201 response

  PM         Implement GET /readings/:patient_id Data visible in DB + via
             for basic data retrieval            API

  PM         Wire all 4 sensors on ESP32;        Hardware + software
             confirm Serial Monitor readings     pipeline working
                                                 end-to-end
  -----------------------------------------------------------------------

## **Day 2 --- Logic & AI**

Goal: Threshold engine running, urgent flags set correctly, AI-drafted
reports appearing in the reports table.

  -----------------------------------------------------------------------
  **Time**   **Task**                            **Deliverable**
  ---------- ----------------------------------- ------------------------
  AM         Implement threshold service with    is_urgent set correctly
             all 5 clinical rules                on test readings

  AM         Implement JWT auth + /auth/login    Doctor can log in and
             endpoint                            receive JWT token

  PM         Implement AI service: prompt        AI draft stored in
             builder + LLM API HTTP client       reports table on urgent
                                                 flag

  PM         Implement GET /reports/pending +    Doctor can fetch pending
             GET /reports/:id                    reports with AI draft

  PM         Implement PATCH /reports/:id for    Doctor can edit and mark
             doctor notes + status update        report as reviewed
  -----------------------------------------------------------------------

## **Day 3 --- UI & Reports**

Goal: Complete Doctor Portal API, PDF generation functional, full
end-to-end demo ready.

  -----------------------------------------------------------------------
  **Time**   **Task**                            **Deliverable**
  ---------- ----------------------------------- ------------------------
  AM         Implement POST                      Dual-copy PDFs generated
             /reports/:id/approve + PDF          on approval
             generation service                  

  AM         Implement GET /reports/:id/pdf to   Doctor can download
             stream PDF download                 approved report PDF

  PM         Wire basic HTML Doctor Portal       Doctor can view, edit,
             (single-page; minimal styling)      approve from browser

  PM         End-to-end test: ESP32 → POST →     Full demo run captured
             urgent flag → AI draft → approve →  on video/screenshots
             PDF                                 

  PM         README documentation + .env.example Project is presentable
             file                                and reproducible
  -----------------------------------------------------------------------

# **9. Environment Configuration**

All secrets and config must be in a .env file loaded via godotenv at
startup. Never commit .env to Git --- add it to .gitignore.

> \# .env
>
> SERVER_PORT=8080
>
> DB_HOST=localhost
>
> DB_PORT=5432
>
> DB_NAME=health_monitor
>
> DB_USER=postgres
>
> DB_PASSWORD=yourpassword
>
> JWT_SECRET=your_strong_jwt_secret_here
>
> LLM_API_KEY=your_llm_api_key_here
>
> LLM_MODEL=claude-sonnet-4-20250514
>
> LLM_API_URL=https://api.anthropic.com/v1/messages
>
> DEVICE_SECRET_KEY=esp32_device_key_here
>
> PDF_STORAGE_PATH=./storage/reports

# **10. Error Handling**

All errors must return a consistent JSON structure. Never expose
internal error messages or stack traces in production responses.

> { \"error\": \"validation_failed\", \"message\": \"bpm must be between
> 0 and 300\", \"code\": 400 }

  --------------------------------------------------------------------------
  **HTTP     **Error Type**      **When to Return**
  Code**                         
  ---------- ------------------- -------------------------------------------
  400        validation_failed   Missing required fields, out-of-range
                                 sensor values

  401        unauthorized        Missing or expired JWT token; invalid
                                 device key

  403        forbidden           Doctor trying to access another doctor\'s
                                 patients; patient accessing doctor routes

  404        not_found           Patient ID or Report ID does not exist in
                                 DB

  409        conflict            Approving an already-approved report

  500        internal_error      Database failure, PDF generation failure,
                                 LLM API down

  503        ai_unavailable      LLM API returns non-200; draft deferred,
                                 reading still stored
  --------------------------------------------------------------------------

# **11. Future Scope (Post v1.0)**

-   Real-time ECG waveform streaming via WebSockets.

-   Email and push notification to doctor on urgent flag.

-   Patient mobile app (React Native) consuming the same API.

-   Predictive analytics using historical readings for long-term trend
    > detection.

-   Multi-patient dashboard with aggregate population health insights.

-   Integration with hospital EMR systems via FHIR-compliant API layer.

-   OCI Resource Manager / Terraform-based cloud deployment (OCI Always
    > Free tier).

# **12. Sign-off**

  --------------------------------------------------------------------------------
  **Name**                **Role**                **Sign-off**
  ----------------------- ----------------------- --------------------------------
  Md Yunus                Backend Lead / DevOps   \_\_\_\_\_\_\_\_\_\_\_\_\_\_\_

  Vishal Yadav            Hardware & Integration  \_\_\_\_\_\_\_\_\_\_\_\_\_\_\_

  Ankur Sharma            Frontend / Doctor       \_\_\_\_\_\_\_\_\_\_\_\_\_\_\_
                          Portal                  
  --------------------------------------------------------------------------------

Document Version: 1.0 \| Status: Draft \| Next Review: Post Day-3 Demo
