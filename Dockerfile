FROM golang:1.23-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /infosir ./cmd
FROM alpine:3.22
RUN apk add --no-cache ca-certificates && adduser -D -u 10001 app
WORKDIR /app
COPY --from=builder /infosir /app/infosir
COPY internal/db/migrations /app/migrations
ENV MIGRATIONS_PATH=/app/migrations
USER 10001:10001
EXPOSE 8080
ENTRYPOINT ["/app/infosir"]
