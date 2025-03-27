# infosir – Realtime Klines Sync, Storage & Forecasting ⛓️📊

> A modern microservice for collecting, processing, and forecasting cryptocurrency Klines from Binance Futures in real-time. Built with Go, TimescaleDB, and NATS JetStream. Fully containerized with Docker Compose for local and production environments.

---

## 🚀 Features

- Fetch Klines (candlesticks) from Binance Futures in real-time
- Publish/consume Klines through NATS JetStream
- Store data efficiently in **TimescaleDB hypertables**
- Continuous aggregate views for fast timeframe queries (15m, 30m, 1h, 4h, 1d)
- Auto-compression & retention policies
- Configurable historical sync & live interval fetchers
- Modern structured logging with Zap
- Resilient architecture and graceful shutdown

---

## ⚙️ Technologies Used

| Component     | Tech Stack               |
|---------------|--------------------------|
| Language      | Go 1.23 (Alpine)         |
| Database      | PostgreSQL + TimescaleDB |
| Queue         | NATS + JetStream         |
| Migrations    | golang-migrate           |
| Scheduler     | Custom cron-like jobs    |
| Logger        | Uber Zap                 |
| Deployment    | Docker + Docker Compose  |

---

## 📦 Installation (Local)

~~~bash
# 1. Clone the repo
$ git clone https://github.com/youruser/infosir && cd infosir

# 2. Create .env file based on .env.example
$ cp .env.example .env

# 3. Start TimescaleDB, NATS, and infosir service
$ docker compose up --build
~~~

> Migration files must be named with `.up.sql`, e.g. `0001_init_schema.up.sql`, to work with golang-migrate.

---

## 🔎 Project Structure

~~~bash
infosir/
├── cmd/main.go             # Application entrypoint
├── cmd/config/             # Environment configs (dotenv)
├── internal/
│   ├── db/                 # DB init, migrations, repository
│   │   ├── migrations/     # TimescaleDB schema & views
│   ├── jobs/               # Historical sync & scheduler
│   ├── srv/                # Service logic
│   ├── utils/              # Logger & helpers
├── pkg/                    # External integrations
│   ├── crypto/             # Binance API client
│   ├── nats/               # NATS & JetStream client
├── Dockerfile
├── docker-compose.yml
├── .env
~~~

---

## 📝 Configuration

The `.env` file controls all runtime behavior:

~~~env
APP_ENV=dev
LOG_LEVEL=debug
HTTP_PORT=8080
SYNC_ENABLED=true

DB_HOST=db
DB_PORT=5432
DB_USER=root
DB_PASSWORD=yourpassword
DB_NAME=infosir_db

NATS_URL=nats://nats:4222
NATS_SUBJECT=infosir_kline
NATS_STREAM_NAME=infosir_kline_stream
NATS_CONSUMER_NAME=infosir_kline_consumer

BINANCE_BASE_URL=https://fapi.binance.com/fapi/v1/klines
KLINE_PAIRS=NILUSDT,XUSDUSDT
KLINE_INTERVAL=1m
KLINE_LIMIT=1
~~~

---

## ⚡️ TimescaleDB Features

- **Hypertable**: `futures_klines(time TIMESTAMPTZ, symbol TEXT, ...)`
- **Compression**: Auto-compressed after 30 days
- **Materialized Views**:
   - `klines_15m`, `klines_30m`, `klines_1h`, `klines_4h`, `klines_1d`
- **Policies**: Scheduled refresh every 5-15 minutes

---

## 🚧 Migration Troubleshooting

If you hit this error:

~~~
fatal: migrate.Up: Dirty database version X. Fix and force version.
~~~

Run:
~~~bash
# Log into the DB container
$ docker compose exec db psql -U root -d infosir_db

-- Clean migration state manually
> DELETE FROM schema_migrations;

# OR force the state in Go:
> m.Force(X)
~~~

---

## 🛌 REST API

Currently minimal:
~~~http
GET /healthz   # Returns 200 OK
~~~

More orchestrator routes coming soon.

---

## 🦜 Authors

- Kyrylo Malovychko ([@katalvlaran](https://github.com/katalvlaran)) – design, architecture, orchestration

---

## 🌎 Future Plans

- [ ] Add Prometheus & Grafana monitoring
- [ ] Web admin panel to manage jobs and DB entries
- [ ] Strategy backtester and optimizer module
- [ ] CI/CD to AWS (with Terraform support)

---

## 📊 Benchmarks & Performance

> Coming soon with full logs, request throughput, and resource profiling.

---

## 📅 Last updated: 2025-03-27

