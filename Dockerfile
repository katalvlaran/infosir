# Используем официальный образ Go для сборки (stage 1)
FROM golang:1.23-alpine AS builder
LABEL authors="katalvlaran"
WORKDIR /app

# Скопируем go.mod и go.sum сначала, чтобы закешировать скачивание модулей
COPY go.mod go.sum ./
RUN go mod download

# Копируем весь оставшийся код
COPY . .

# Собираем бинарник
RUN go build -o /infoSir cmd/main.go

# --- Минимизируем финальный образ (stage 2) ---
FROM alpine:3.17

# Создадим непривилегированного пользователя (для безопасности)
RUN addgroup -S infosir && adduser -S infosir -G infosir

WORKDIR /app

# Копируем собранный бинарник и .env (если хотите внутри контейнера)
COPY --from=builder /infoSir /app/
COPY .env /app/.env

# Меняем владельца, чтобы запустить под пользователем
RUN chown -R infosir:infosir /app

USER infosir

# Пробрасываем порт 8080 для веб-сервера
EXPOSE 8080

# Запускаем бинарник
CMD ["/app/infoSir"]
