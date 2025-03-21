# InfoSir – Crypto Info Microservice

**InfoSir** is a microservice that fetches short-term cryptocurrency kline/candle data from Binance and publishes it to a NATS topic. It also exposes an HTTP endpoint for on-demand data requests from an orchestrator (e.g., `katalvlaran_core`).

---

## Table of Contents

1. [Overview](#overview)  
2. [Features](#features)  
3. [Tech Stack](#tech-stack)  
4. [Architecture](#architecture)  
5. [Installation](#installation)  
6. [Configuration](#configuration)  
7. [Usage](#usage)  
8. [HTTP Endpoints](#http-endpoints)  
9. [NATS Publishing](#nats-publishing)  
10. [Testing](#testing)  
11. [Roadmap / Future Work](#roadmap--future-work)  
12. [License](#license)

---

## 1. Overview

Modern crypto-based applications often need an efficient means of collecting real-time or near-real-time data from exchanges, then distributing or publishing that data for downstream services. **InfoSir** addresses this by:

- Periodically fetching the last N minute-summaries (klines) from Binance for selected currency pairs.
- Publishing these data in JSON format to a NATS topic for further consumption.
- Optionally allowing immediate fetches from an HTTP endpoint, triggered by your orchestrator.

The design is minimal, focusing on reliability and ease of customization.

---

## 2. Features

1. **Scheduled Data Fetch**  
   - Every minute (by default) job fetches fresh minute-level klines for configured pairs, up to a limit (e.g. 10).
   - Minimizes local storage overhead – data is ephemeral.

2. **NATS Publication**  
   - Publishes kline arrays as JSON to a configured NATS subject, letting other microservices subscribe.

3. **On-Demand Fetch**  
   - Exposes an HTTP `POST /orchestrator/fetch` allowing an orchestrator to request quick queries for a pair/time/klines combination.

4. **Flexible Configuration**  
   - All settings (pairs, intervals, limit, etc.) are environment-driven.  
   - `.env` support via [joho/godotenv](https://github.com/joho/godotenv).

5. **Go-based**  
   - Leverages concurrency through goroutines.  
   - Full logging with [uber-go/zap](https://github.com/uber-go/zap).

---

## 3. Tech Stack

- **Language**: [Go 1.20+](https://go.dev/)  
- **Queue**: [NATS.io](https://nats.io/)  
- **HTTP**: net/http or standard library  
- **Binance**: [ccxt/go-binance](https://github.com/ccxt/go-binance) or [gjvr/binance-api](https://github.com/gjvr/binance-api)  
- **Validation**: [go-ozzo/ozzo-validation](https://github.com/go-ozzo/ozzo-validation)  
- **Env**: [caarlos0/env](https://github.com/caarlos0/env), [joho/godotenv](https://github.com/joho/godotenv)  
- **Logging**: [go.uber.org/zap](https://github.com/uber-go/zap)

---

## 4. Architecture

```
infosir/
├── cmd/
│   ├── main.go
│   ├── config.go
│   └── handler/
│       └── orchestrator_handler.go
├── internal/
│   ├── model/
│   │   └── kline.go
│   ├── srv/
│   │   └── service.go
│   └── jobs/
│       └── request.go
├── pkg/
│   ├── crypto/
│   │   └── binance.go
│   └── nats/
│       └── nats_infosir.go
├── tests/
│   ├── unit_test.go
│   ├── integration_test.go
│   └── mocks/
│       ├── mock_binance.go
│       └── mock_nats.go
├── docker-compose.yml
├── Dockerfile
├── go.mod
├── go.sum
├── .env (optional)
└── README.md
```

Key components:

- **`cmd/main.go`**: Entry point. Loads config, initializes NATS, sets up the HTTP server, starts the periodic job.  
- **`internal/jobs/request.go`**: The scheduled (cron-like) job that fetches fresh klines.  
- **`internal/srv/service.go`**: Core logic to fetch klines (retry logic), publish to NATS, etc.  
- **`pkg/crypto/binance.go`**: Wraps the Binance client library.  
- **`pkg/nats/nats_infosir.go`**: Manages NATS Publish.  
- **`tests/`**: Unit and integration tests, plus mocks for external services.

---

## 5. Installation

```bash
git clone https://github.com/katalvlaran/infosir.git
cd infosir
go mod tidy
go build -o infosir cmd/main.go
```

**Requirements**:
- **Go** 1.20 or later
- **NATS** server running locally or accessible by a URL
- Public endpoints from **Binance** for candle data

---

## 6. Configuration

Environment variables or `.env` can define:

| Variable           | Default                  | Description                                   |
|--------------------|--------------------------|-----------------------------------------------|
| `APP_ENV`          | dev                      | e.g. dev, main, staging                       |
| `LOG_LEVEL`        | debug                    | debug, info, warn, error                      |
| `NATS_URL`         | nats://127.0.0.1:4222   | NATS server endpoint                          |
| `NATS_SUBJECT`     | infosir_kline           | Subject for publishing klines                 |
| `BINANCE_BASE_URL` | https://api.binance.com | Base API URL for public data fetch            |
| `PAIRS`            | BTCUSDT,ETHUSDT         | List of trading pairs, comma-separated        |
| `KLINE_INTERVAL`   | 1m                      | E.g. 1m, 5m, 1h, etc.                         |
| `KLINE_LIMIT`      | 10                       | Number of candles to fetch each time          |
| `HTTP_PORT`        | 8080                     | HTTP server port                              |

**Sample .env**:

```
APP_ENV=dev
LOG_LEVEL=debug
NATS_URL=nats://127.0.0.1:4222
NATS_SUBJECT=infosir_kline
BINANCE_BASE_URL=https://api.binance.com
PAIRS=BTCUSDT,ETHUSDT
KLINE_INTERVAL=1m
KLINE_LIMIT=10
HTTP_PORT=8080
```

---

## 7. Usage

1. **Check NATS**: Make sure a NATS server is up locally on port 4222 or somewhere you can connect.
2. **Run**:
   ```bash
   go run cmd/main.go
   ```
   or
   ```bash
   ./infosir
   ```
3. **Monitor logs**: You should see logs about successful NATS connections, scheduled job triggers every minute, etc.

**To test** the orchestrator fetch endpoint:
```bash
curl -X POST -H "Content-Type: application/json" \
     -d '{"pair":"BTCUSDT","time":"1m","klines":10}' \
     http://localhost:8080/orchestrator/fetch
```
Response:
```json
{"status":"ok"}
```

---

## 8. HTTP Endpoints

**`POST /orchestrator/fetch`**
- **Body**: JSON object:
  ```json
  {
    "pair": "BTCUSDT",
    "time": "1m",
    "klines": 10
  }
  ```
- **Validation**:
   - `pair` != ""
   - `klines` > 0
- **Action**:
   - Immediately request from Binance the last `klines` candles at `time` interval for `pair`.
   - Publish result to NATS in JSON array format.
   - Return JSON `{"status":"ok"}` with `200` status on success.

---

## 9. NATS Publishing

The microservice calls `PublishKlines` on a subject (by default `infosir_kline`):
- Publishes an array of klines in JSON.
- Example message body:

```json
[
  {
    "openTime": 1695108400000,
    "open": 26100.12,
    "high": 26110.99,
    "low": 25950.01,
    "close": 26055.55,
    "volume": 321.456,
    "closeTime": 1695108459999
  },
  ...
]
```
Other services can subscribe and handle these messages.

---

## 10. Testing

We provide four test files:

1. `tests/unit_test.go` – High-level unit test of `service.GetKlines(...)` and kline validations.
2. `tests/mocks/mock_binance.go` – Mocks the Binance client calls.
3. `tests/mocks/mock_nats.go` – Mocks NATS for publish calls.
4. `tests/integration_test.go` – Integration-like test using `httptest`.

Run them:
```bash
go test ./... -v
```

---

## 11. Roadmap / Future Work

- Add historical data caching with Redis or similar.
- Support multi-interval or multi-API concurrency.
- Expand to multiple exchanges (Bitfinex, Coinbase) with a unified client interface.
- Add advanced metrics & instrumentation (Prometheus).

---

## 12. License

This project is licensed under the [MIT License](LICENSE).  
You’re free to copy, modify, distribute as per the terms in the file.

---

*Last Updated: March 2025*  
Maintained by [katalvlaran](mailto:katalvlaran@gmail.com)
```
