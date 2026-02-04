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

# Auth
sure-cli config set api_key <key>
# or OAuth:
sure-cli login --email you@example.com --password "..." [--otp 123456]

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

# Plan (client-side budget/runway/forecast)
sure-cli plan budget --month 2026-02
sure-cli plan runway --account-id <id> --days 90
sure-cli plan forecast --days 30 [--daily]

# Propose automations
sure-cli propose rules --months 3

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
- API sign convention note (important): `docs/notes-api-sign-convention.md`
- Smoke test: `tools/smoke-oauth.sh`
- JSON Schemas: `docs/schemas/v1/` (accounts, transactions, insights, plan)

## Heuristics Configuration

All insight heuristics are configurable via `~/.config/sure-cli/config.yaml`:

```yaml
heuristics:
  fees:
    keywords: []  # empty = use 60+ default keywords (EN/ES/DE/FR)
  subscriptions:
    period_min_days: 20
    period_max_days: 40
    weekly_min_days: 6
    weekly_max_days: 9
    stddev_max_days: 3.0
    amount_stddev_ratio: 0.1
  leaks:
    min_count: 3
    min_total: 15.0
    max_avg: 10.0
  rules:
    min_consistency: 0.7
    min_occurrences: 2
```

Inspect current config:
```bash
sure-cli config heuristics      # show all heuristic settings
sure-cli config fee-keywords    # show active fee keywords (60+ defaults)
```

## TODO / Open Questions

### API quirks / gaps (found while testing)
- **`GET /api/v1/accounts/:id` returns 404** upstream (route exists, but controller/view missing). `sure-cli accounts show` currently falls back to list lookup.
- **Transaction sign mismatch**: UI shows income `+2.00€` and expense `-1.00€`, but API payload returned inverted signs (income as `-€2.00`, expense as `€1.00`). Needs investigation in Sure serializer/formatting.

### CLI features (next)
- Improve OAuth UX (interactive prompts, safer password input, optional signup flow).
- Agent-first schemas: add per-command schemas (esp. insights) and validate samples against them.
- Hardening/refactor: reduce map[string]any parsing; expand typed models; improve error surfaces.

### Intelligent commands (Phase 4)
- Improve `insights subscriptions/fees/leaks` output richness (confidence, reasons, suggested actions).
- `plan budget/forecast/runway`
- `propose rules` + `apply rules --apply`

