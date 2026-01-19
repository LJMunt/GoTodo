# GoToDo


## Local dev

1. Copy env
    - `cp .env.example .env`

2. Start Postgres
    - `docker-compose up -d`

3. Run migrations
    - `set -a; source .env; set +a`
    - `migrate -path internal/db/migrations -database "$DATABASE_URL" up`

4. Run API
    - `go run ./cmd/server`
