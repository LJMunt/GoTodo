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

# Final stage
FROM alpine:latest

WORKDIR /app

# Copy the binary and migrations
COPY --from=builder /gotodo .
COPY internal/db/migrations ./internal/db/migrations

# Set environment variables
ENV PORT=8080
ENV MIGRATIONS_PATH=/app/internal/db/migrations

EXPOSE 8080

CMD ["./gotodo"]
