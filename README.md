# MXIL Backend (Go)

A compact, realistic backend skeleton for an email service (MXIL) — includes user auth (bcrypt + JWT), session storage, a minimal repository layer, migrations manager, and HTTP handlers for common operations. This repository intentionally leaves advanced components (IMAP/POP3/SMTP servers, provider bridges, I2P/Tor adapters, full delivery pipelines) as extension points for you to implement next.

---

## What’s included

- `cmd/server` — server bootstrap and HTTP route wiring (Echo).
- `internal/config` — YAML + env-based configuration loader.
- `internal/models` — core data models (User, Session, Email, Contact).
- `internal/repository` — SQLx-backed repositories for core entities.
- `internal/api/handlers` — HTTP handlers for auth, user, admin, email, contact, health.
- `internal/migrations` — basic migration runner for `migrations/*.sql`.
- `migrations/0001_init.sql` — initial database schema.
- Docker & docker-compose examples.
- `config.example.yml`, `.env.example`, `Makefile`.

---

## Quick start (local)

Prerequisites:

- Go 1.20+ (or 1.25 as in the `go.mod`)
- PostgreSQL (or Docker)
- `git`

Clone and prepare:

```bash
git clone <repo>
cd <repo>

# copy config
cp config.example.yml config.yml

# edit config.yml to match your DB credentials or set environment variables
```
Run the server:

```bash
go run ./cmd/server
```

The server will start on `localhost:8080` by default. You can test the health endpoint:

```bash
curl http://localhost:8080/health
```
You should see a JSON response indicating the server is healthy.
{
  "status": "ok"
}
