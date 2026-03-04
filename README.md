# GoToDo

[![Go Version](https://img.shields.io/github/go-mod/go-version/LJMunt/GoToDo)](https://go.dev/)
[![License](https://img.shields.io/github/license/LJMunt/GoToDo)](LICENSE)
[![API Docs](https://img.shields.io/badge/API-Documentation-blue)](https://docs.todexia.app)

GoToDo is a production-ready, clean, and robust ToDo API built with Go. It serves as a comprehensive backend for task management applications, featuring advanced capabilities like recurring tasks, multi-language support, and administrative controls.

### API Documentation

The complete API reference is available at:  
👉 **[https://docs.todexia.app](https://docs.todexia.app)**

---

### Roadmap
- [x] **User Management**
- [x] **Project Management**
- [x] **Task Management**
- [x] **Recurring Tasks**
- [x] **Tag Management**
- [x] **Task Tags**
- [x] **Occurrence Management**
- [x] **Language Management**
- [x] **Admin Tools**
- [x] **Configuration Management**
- [x] **Dockerization**
- [x] **User Authentication**
- [x] **User Password Reset**
- [x] **User Email Verification**
- [x] **User Preferences Management**
- [ ] **User Invitations**
- [ ] **Organizations**
- [ ] **Redis Caching**
- [ ] **GoTodo Runner**
- [ ] **Android App**

### Tech Stack

- **Language:** Go 1.24 (Modern idioms & toolchain)
- **Framework:** [Chi Router](https://github.com/go-chi/chi)
- **Database:** PostgreSQL with [pgx](https://github.com/jackc/pgx)
- **Migrations:** [golang-migrate](https://github.com/golang-migrate/migrate)
- **Auth:** JWT (JSON Web Tokens)
- **Containerization:** Docker & Docker Compose

---

### Getting Started

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

###  Configuration

| Variable | Description | Default |
|----------|-------------|---------|
| `DATABASE_URL` | PostgreSQL connection string | *Required* |
| `JWT_SECRET` | Secret key for signing JWTs | *Required* |
| `JWT_ISSUER` | JWT issuer claim (`iss`) | *Required* |
| `JWT_AUDIENCE` | JWT audience claim (`aud`) | *Required* |
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

### 🔐 Generating Secrets

Use strong, random values for both JWT signing and the secrets master key.

**JWT secret (base64, 32 bytes):**
```bash
openssl rand -base64 32
```

**Secrets master key (base64, 32 bytes for AES-256):**
```bash
openssl rand -base64 32
```

Set the outputs in your environment:
```bash
JWT_SECRET=...
SECRETS_MASTER_KEY_B64=...
```

---

### Administrative Tools

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

#### Promoting a User to Admin
Use the promotion tool to grant admin access by email. It will ask for a confirmation code.

**Via Docker:**
```bash
docker-compose exec app ./promote-admin user@example.com
```

**Via Local Go:**
```bash
go run ./cmd/promote-admin user@example.com
```

#### Demoting an Admin User
Use the demotion tool to remove admin access by email. It will ask for a confirmation code.

**Via Docker:**
```bash
docker-compose exec app ./demote-admin user@example.com
```

**Via Local Go:**
```bash
go run ./cmd/demote-admin user@example.com
```

#### Resetting an Instance (Dangerous)
Factory reset: drops the public schema, recreates it, and re-runs migrations. This permanently deletes all data.
It requires an interactive confirmation phrase plus a random code.

**Via Docker (TTY required):**
```bash
docker-compose exec app ./reset-instance
```

**Via Local Go:**
```bash
go run ./cmd/reset-instance
```

---

#### Exporting Configuration
Exports non-public config keys to a JSON file.
When running inside Docker, the file is written inside the container; copy it out or bind-mount a host folder.

**Via Docker:**
```bash
docker-compose exec app ./config-export --out /tmp/config-export.json
```

**Via Local Go:**
```bash
go run ./cmd/config-export --out config-export.json
```

#### Importing Configuration
Imports non-public config keys from a JSON file.

**Via Docker:**
```bash
docker-compose exec app ./config-import --in /tmp/config-export.json
```

**Via Local Go:**
```bash
go run ./cmd/config-import --in config-export.json
```

#### Exporting User Data
Exports users, projects, tasks, tags, task_tags, and occurrences to a JSON file.
Use `--tokenize` to anonymize emails and public IDs.
When running inside Docker, the file is written inside the container; copy it out or bind-mount a host folder.

**Via Docker:**
```bash
docker-compose exec app ./user-export --out /tmp/user-export.json
docker-compose exec app ./user-export --tokenize --out /tmp/user-export-anon.json
```

**Via Local Go:**
```bash
go run ./cmd/user-export --out user-export.json
go run ./cmd/user-export --tokenize --out user-export-anon.json
```

---

### License

Distributed under the MIT License. See `LICENSE` for more information.

---