# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o /gotodo cmd/server/main.go
RUN CGO_ENABLED=0 GOOS=linux go build -o /restore-languages cmd/restore-languages/main.go

# Final stage
FROM alpine:latest

WORKDIR /app

# Copy the binary and migrations
COPY --from=builder /gotodo .
COPY --from=builder /restore-languages .
COPY internal/db/migrations ./internal/db/migrations
COPY internal/db/restore_languages.sql ./internal/db/restore_languages.sql

# Set environment variables
ENV PORT=8080
ENV MIGRATIONS_PATH=/app/internal/db/migrations

EXPOSE 8081

CMD ["./gotodo"]
