```
  ____  _   _ ____  _____      ____ _     ___
 / ___|| | | |  _ \| ____|    / ___| |   |_ _|
 \___ \| | | | |_) |  _|     | |   | |    | |
  ___) | |_| |  _ <| |___    | |___| |___ | |
 |____/ \___/|_| \_\_____|    \____|_____|___|

 sure-cli — Agent-first CLI for Sure
```

Agent-first CLI for **Sure** (we-promise/sure) — self-hosted personal finance app.

## Agent-first contract

- **Default output is JSON** (`--format=json`) wrapped in a stable **Envelope**: `{data, meta, error}`
- **Write operations are safe by default**: `--dry-run` is the default; use `--apply` to execute
- **Schemas are versioned** under `docs/schemas/v1/` and should remain backward compatible

Links:
- ADR: `docs/ADR-001-go-agent-first.md`
- Schemas: `docs/schemas/README.md`

## Goals

- Deterministic, scriptable commands for agents and power users
- JSON-first output (stable schemas)
- Safe automation patterns (`--dry-run` / `--apply`)

## Install (dev)

```bash
go install ./...
```

## Usage

```bash
sure-cli --help

# Configure
sure-cli config set api_url http://localhost:3000
sure-cli config set token <access_token>

# Accounts
sure-cli accounts list --format=table
sure-cli accounts list --format=json
sure-cli accounts show <account_id>

# Transactions
sure-cli transactions list --from 2026-01-01 --to 2026-02-01 --per-page 50 --format=table
sure-cli transactions show <transaction_id>

# Safe writes (default is --dry-run)
sure-cli transactions create --amount "-12.34" --date 2026-02-04 --name "Coffee" --account-id <id>
sure-cli transactions create --amount "-12.34" --date 2026-02-04 --name "Coffee" --account-id <id> --apply

sure-cli transactions update <tx_id> --name "Coffee (fixed)" --dry-run
sure-cli transactions delete <tx_id> --apply

# Phase 4 (read-only heuristics)
sure-cli insights subscriptions --days 120
sure-cli insights fees --days 120
sure-cli insights leaks --days 120

# Sync
sure-cli sync
```

## Auth

Sure supports OAuth bearer tokens and API keys.

- OAuth: `Authorization: Bearer <token>`
- API key: `X-Api-Key: <key>`

## OAuth login + refresh

```bash
sure-cli login --email you@example.com --password "..." [--otp 123456]

# Later (refresh access token using stored refresh token)
sure-cli refresh
```

Required device payload fields are stored under `auth.device.*` in config (defaults are provided).

## Docs

- PRD: `docs/PRD-CLI.md`
- ADR: `docs/ADR-001-go-agent-first.md`

## TODO / Open Questions

### API quirks / gaps (found while testing)
- **`GET /api/v1/accounts/:id` returns 404** upstream (route exists, but controller/view missing). `sure-cli accounts show` currently falls back to list lookup.
- **Transaction sign mismatch**: UI shows income `+2.00€` and expense `-1.00€`, but API payload returned inverted signs (income as `-€2.00`, expense as `€1.00`). Needs investigation in Sure serializer/formatting.

### CLI features (next)
- Add `login` (OAuth) and token refresh flow (device info required by Sure auth).
- Hardening/refactor: move more pagination/window logic into `internal/api` (typed models, better errors).

### Intelligent commands (Phase 4)
- Improve `insights subscriptions/fees/leaks` output richness (confidence, reasons, suggested actions).
- `plan budget/forecast/runway`
- `propose rules` + `apply rules --apply`

