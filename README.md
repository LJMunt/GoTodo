# GoToDo

[![Go Version](https://img.shields.io/github/go-mod/go-version/LJMunt/GoToDo)](https://go.dev/)
[![License](https://img.shields.io/github/license/LJMunt/GoToDo)](LICENSE)
[![API Docs](https://img.shields.io/badge/API-Documentation-blue)](https://docs.todexia.app)

GoToDo is a production-ready, clean, and robust ToDo API built with Go. It serves as a comprehensive backend for task management applications, featuring advanced capabilities like recurring tasks, multi-language support, and administrative controls.

### 📖 API Documentation

The complete API reference is available at:  
👉 **[https://docs.todexia.app](https://docs.todexia.app)**

---

### ✨ Features

- **Core Task Management:** CRUD operations for tasks, projects, and tags.
- **Advanced Recurrence:** Support for daily, weekly, and monthly recurring tasks with automatic occurrence generation.
- **Multi-language Support:** Dynamic translation system for UI labels and user-specific language preferences (English, French, German).
- **Secure Authentication:** JWT-based auth with brute-force protection and secure password hashing.
- **Admin Dashboard Ready:** Dedicated endpoints for user management, system health metrics, and global configuration.
- **Robust Infrastructure:** PostgreSQL storage, automatic migrations, and full Docker containerization.

### 🛠 Tech Stack

- **Language:** Go 1.24 (Modern idioms & toolchain)
- **Framework:** [Chi Router](https://github.com/go-chi/chi) (Lightweight, idiomatic)
- **Database:** PostgreSQL with [pgx](https://github.com/jackc/pgx)
- **Migrations:** [golang-migrate](https://github.com/golang-migrate/migrate)
- **Auth:** JWT (JSON Web Tokens)
- **Containerization:** Docker & Docker Compose

---

### 🚀 Getting Started

#### Prerequisites

- [Docker](https://docs.docker.com/get-docker/) & [Docker Compose](https://docs.docker.com/compose/install/)
- [Go 1.24+](https://go.dev/doc/install) (for local development)

#### Quick Start (Docker)

The fastest way to get GoToDo up and running:

1. **Clone the repository**
   ```bash
   git clone https://github.com/your-username/GoToDo.git
   cd GoToDo
   ```

2. **Launch with Docker Compose**
   ```bash
   docker-compose up --build
   ```

The API will be available at `http://localhost:8081/api/v1`.  
Migrations and seed data are applied automatically on startup.

#### Local Development

1. **Start the database only**
   ```bash
   docker-compose up -d db
   ```

2. **Set up environment variables**
   Create a `.env` file or export them:
   ```bash
   DATABASE_URL=postgres://gotodo:gotodo@localhost:5432/gotodo?sslmode=disable
   JWT_SECRET=your_super_secret_jwt_key
   PORT=8080
   ```

3. **Run the server**
   ```bash
   go run ./cmd/server
   ```
   The server listens on `http://localhost:8080/api/v1`.

---

### ⚙️ Configuration

| Variable | Description | Default |
|----------|-------------|---------|
| `DATABASE_URL` | PostgreSQL connection string | *Required* |
| `JWT_SECRET` | Secret key for signing JWTs | *Required* |
| `SECRETS_MASTER_KEY_B64` | Base64-encoded 32-byte master key for encrypted secrets | *Required for secret config* |
| `PORT` | API listening port | `8080` |
| `MIGRATIONS_PATH` | Path to SQL migrations | `./internal/db/migrations` |
| `HTTP_READ_HEADER_TIMEOUT` | Max time to read request headers (Go duration) | `5s` |
| `HTTP_READ_TIMEOUT` | Max time to read full request (Go duration) | `20s` |
| `HTTP_WRITE_TIMEOUT` | Max time to write response (Go duration) | `20s` |
| `HTTP_IDLE_TIMEOUT` | Max idle keep-alive time (Go duration) | `60s` |
| `HTTP_SHUTDOWN_TIMEOUT` | Graceful shutdown timeout (Go duration) | `10s` |
| `HTTP_MAX_HEADER_BYTES` | Max request header size in bytes | `1048576` |
| `HTTP_MAX_BODY_BYTES` | Max request body size (bytes) | `10485760` |
| `HTTP_ADMIN_MAX_BODY_BYTES` | Max request body size for `/api/v1/admin` (bytes) | `52428800` |
| `LOG_LEVEL_REFRESH_INTERVAL` | Log level refresh interval (Go duration) | `5s` |
| `CONFIG_WATCH_INTERVAL` | Config sanity check interval (Go duration) | `1m` |
| `TRUSTED_PROXIES` | Comma-separated proxy IPs/CIDRs for real client IP | *(empty)* |

---

### 🛠 Administrative Tools

#### Restoring Default Languages
If default translations are missing or corrupted, use the built-in restoration tool:

**Via Docker:**
```bash
docker-compose exec app ./restore-languages
```

**Via Local Go:**
```bash
go run ./cmd/restore-languages
```

---

### 📜 License

Distributed under the MIT License. See `LICENSE` for more information.

---
Built with ❤️ by the Todexia Team.
