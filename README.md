# GoToDo

GoToDo is a clean and simple ToDo API written in Go. It provides a robust backend for managing tasks, projects, and users, featuring JWT authentication and PostgreSQL storage.

### API Documentation

Detailed API documentation is available at [https://docs.todexia.app](https://docs.todexia.app).

### Features

- **User Authentication:** Secure signup and login using JWT.
- **Project Management:** Create and organize tasks within projects.
- **Role-based Access:** Includes administrative routes for user management.
- **Database Migrations:** Automatic schema management on startup.
- **Dockerized:** Full containerization for both application and database.

### Prerequisites

- [Docker](https://docs.docker.com/get-docker/) and [Docker Compose](https://docs.docker.com/compose/install/)
- [Go](https://go.dev/doc/install) 1.24 or later (for local development without Docker)

### Getting Started (with Docker)

The easiest way to get the API running is using Docker Compose:

1. **Clone the repository**
   ```bash
   git clone https://github.com/your-username/GoToDo.git
   cd GoToDo
   ```

2. **Run with Docker Compose**
   ```bash
   docker-compose up --build
   ```

The API will be available at `http://localhost:8081`. The database migrations are applied automatically on startup.

### Local Development (without Docker)

If you prefer to run the Go application locally:

1. **Start the Database**
   ```bash
   docker-compose up -d db
   ```

2. **Configure Environment**
   Ensure you have a `DATABASE_URL` environment variable set:
   ```bash
   export DATABASE_URL=postgres://gotodo:gotodo@localhost:5432/gotodo?sslmode=disable
   export JWT_SECRET=your_secret_here
   ```

3. **Start the API Server**
   ```bash
   go run ./cmd/server
   ```
   The server will run migrations automatically on startup and listen on `http://localhost:8080`.
