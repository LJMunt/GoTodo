# GoToDo

GoToDo is a clean and simple ToDo API written in Go. It provides a robust backend for managing tasks, projects, and users, featuring JWT authentication and PostgreSQL storage.

### Features

- **User Authentication:** Secure signup and login using JWT.
- **Project Management:** Create and organize tasks within projects.
- **Role-based Access:** Includes administrative routes for user management.
- **Database Migrations:** Easy schema management with `golang-migrate`.
- **Docker Support:** Quickly spin up a local PostgreSQL instance.

### Prerequisites

- [Go](https://go.dev/doc/install) 1.24 or later
- [Docker](https://docs.docker.com/get-docker/) and [Docker Compose](https://docs.docker.com/compose/install/)
- [golang-migrate](https://github.com/golang-migrate/migrate) CLI

### Local Setup

1. **Clone the repository**
   ```bash
   git clone https://github.com/your-username/GoToDo.git
   cd GoToDo
   ```

2. **Configure Environment Variables**
   Copy the example environment file and adjust if necessary:
   ```bash
   cp .env.example .env
   ```

3. **Start the Database**
   Launch the PostgreSQL container:
   ```bash
   docker-compose up -d
   ```

4. **Run Database Migrations**
   Apply the migrations to set up the database schema:
   ```bash
   # Make sure you have the environment variables loaded or replace $DATABASE_URL
   set -a; source .env; set +a
   migrate -path internal/db/migrations -database "$DATABASE_URL" up
   ```

5. **Start the API Server**
   ```bash
   go run ./cmd/server
   ```
   The API will be available at `http://localhost:8081/api/v1` (or the port specified in your `.env`).

### API Endpoints

- `GET /api/v1/health` - Check API status
- `POST /api/v1/auth/signup` - Register a new user
- `POST /api/v1/auth/login` - Authenticate and get a JWT
- `GET /api/v1/projects` - List user projects (requires auth)
