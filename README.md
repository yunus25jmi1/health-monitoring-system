# Smart Health Monitoring System Backend

A robust Golang-based backend for real-time health telemetry processing, AI-driven medical report drafting, and a Doctor-in-the-Loop (DIL) approval workflow.

## 🚀 Overview

This system receives vitals (BPM, SpO2, Temperature, Glucose, ECG) from IoT devices (ESP32), evaluates them against clinical thresholds, and automatically triggers an asynchronous AI report generation if anomalies are detected. Doctors can then review, edit, and approve these reports to generate downloadable dual-copy PDFs for both patients and clinical records.

## 🛠️ Tech Stack

- **Language:** Go 1.22+
- **Framework:** Gin Gonic (HTTP Routing)
- **ORM:** GORM v2
- **Database:** PostgreSQL (Optimized for Supabase with PgBouncer)
- **AI Integration:** NVIDIA NIM (Primary), Google Gemini (Secondary), OpenRouter (Tertiary)
- **PDF Generation:** gofpdf
- **Auth:** JWT (Role-based access control) & Secure Device Keys

## 🏗️ Architecture

The project follows a modular service-oriented architecture:

- **`/handlers`**: HTTP interface layer managing request binding and JSON responses.
- **`/services`**: Core business logic, including the AI provider fallback engine, PDF generator, and Threshold engine.
- **`/models`**: GORM database schemas and data structures.
- **`/middleware`**: Security layer handling JWT auth, per-IP rate limiting, and device authentication.
- **`/config`**: Environment and database connection management.
- **`/async_jobs`**: A durable, database-backed job queue system that ensures AI calls and PDF generation are processed without blocking the main telemetry pipeline.

## 📊 Database Schema

- **`users`**: Manages identities for Patients and Doctors.
- **`readings`**: Append-only table for IoT telemetry data.
- **`reports`**: Stores AI-drafted content and doctor annotations.
- **`async_jobs`**: Manages retry logic and concurrency control for background tasks using row-level locking (`SKIP LOCKED`).

## ⚙️ Setup Instructions

### 1. Prerequisites
- [Go](https://go.dev/doc/install) installed.
- Access to a PostgreSQL database (or Supabase).

### 2. Environment Configuration
Create a `.env` file in the root directory (refer to `.env.example`):
```env
SERVER_PORT=8080
DB_DSN=your_postgresql_connection_string
JWT_SECRET=your_secure_jwt_signing_secret
DOCTOR_REGISTRATION_TOKEN=token_for_new_doctor_accounts

# AI API Keys (At least one required)
NIM_API_KEY=...
GEMINI_API_KEY=...
OPENROUTER_API_KEY=...
```

### 3. Run the Application
```bash
# Install dependencies
go mod tidy

# Start the server
go run main.go
```
The server will automatically run migrations and start listening on `:8080`.

## 🛡️ Key Security Features
- **BOLA/IDOR Prevention:** Strict ownership checks ensuring doctors can only access data for their assigned patients.
- **Secure Registration:** Doctor account creation is protected by a secret registration token to prevent unauthorized privilege escalation.
- **Per-Device Authentication:** Each patient has a unique device key, preventing cross-device data spoofing.
- **Concurrency Control:** Row-level locking for background jobs prevents duplicate processing.
- **Rate Limiting:** Dynamic per-IP rate limiting to prevent telemetry flooding.
- **Identity:** Bcrypt password hashing and JWT token rotation.
- **Validation:** Strict clinical boundary checks for all incoming sensor data.

---
Developed by Md Yunus, Vishal Yadav, and Ankur Sharma.
