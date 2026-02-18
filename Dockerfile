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
RUN CGO_ENABLED=0 GOOS=linux go build -o /promote-admin cmd/promote-admin/main.go
RUN CGO_ENABLED=0 GOOS=linux go build -o /demote-admin cmd/demote-admin/main.go
RUN CGO_ENABLED=0 GOOS=linux go build -o /reset-instance cmd/reset-instance/main.go
RUN CGO_ENABLED=0 GOOS=linux go build -o /config-export cmd/config-export/main.go
RUN CGO_ENABLED=0 GOOS=linux go build -o /config-import cmd/config-import/main.go
RUN CGO_ENABLED=0 GOOS=linux go build -o /user-export cmd/user-export/main.go

# Final stage
FROM alpine:latest

WORKDIR /app

# Copy the binary and migrations
COPY --from=builder /gotodo .
COPY --from=builder /restore-languages .
COPY --from=builder /promote-admin .
COPY --from=builder /demote-admin .
COPY --from=builder /reset-instance .
COPY --from=builder /config-export .
COPY --from=builder /config-import .
COPY --from=builder /user-export .
COPY internal/db/migrations ./internal/db/migrations
COPY internal/db/restore_languages.sql ./internal/db/restore_languages.sql

# Set environment variables
ENV PORT=8080
ENV MIGRATIONS_PATH=/app/internal/db/migrations

EXPOSE 8081

CMD ["./gotodo"]
