FROM golang:1.23-alpine AS builder
LABEL authors="katalvlaran"
WORKDIR /app

# Copy go.mod and go.sum first to take advantage of layer caching
COPY go.mod go.sum ./

# Download modules
RUN go mod download

# Copy everything else
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o infosir ./cmd/main.go

# Final stage
FROM alpine:3.17
WORKDIR /app

# Copy the compiled binary, migrations and .env
COPY --from=builder /app/infosir /app/infosir
COPY --from=builder /app/internal/db/migrations /app/migrations
COPY .env /app/.env

# Expose the configured port, default 8080
EXPOSE 8080

# Set working directory explicitly (important!)
WORKDIR /app

# Default entrypoint
ENTRYPOINT ["/app/infosir"]
