```
  ____  _   _ ____  _____      ____ _     ___
 / ___|| | | |  _ \| ____|    / ___| |   |_ _|
 \___ \| | | | |_) |  _|     | |   | |    | |
  ___) | |_| |  _ <| |___    | |___| |___ | |
 |____/ \___/|_| \_\_____|    \____|_____|___|

 sure-cli — Agent-first CLI for Sure
```

Agent-first CLI for **Sure** (we-promise/sure) — self-hosted personal finance app.

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
sure-cli accounts list

# Transactions
sure-cli transactions list --from 2026-01-01 --to 2026-02-01 --limit 50

# Sync
sure-cli sync
```

## Auth

Sure supports OAuth bearer tokens and API keys.

- OAuth: `Authorization: Bearer <token>`
- API key: `X-Api-Key: <key>`

`sure-cli login` (planned) will call `/api/v1/auth/login` and store token + refresh token.

## Docs

- PRD: `docs/PRD-CLI.md`
- ADR: `docs/ADR-001-go-agent-first.md`

## TODO / Open Questions

### API quirks / gaps (found while testing)
- **`GET /api/v1/accounts/:id` returns 404** upstream (route exists, but controller/view missing). `sure-cli accounts show` currently falls back to list lookup.
- **Transaction sign mismatch**: UI shows income `+2.00€` and expense `-1.00€`, but API payload returned inverted signs (income as `-€2.00`, expense as `€1.00`). Needs investigation in Sure serializer/formatting.

### CLI features
- Implement `--format=table` (human-friendly) while keeping JSON default for agents.
- Add `transactions create/update/delete` with safe pattern: `--dry-run` / `--apply`.
- Add `login` (OAuth) and token refresh flow (device info required by Sure auth).
- Add pagination flags (`--page`, `--per-page`) for list commands.
- Define and version JSON schemas for agent-first outputs (`docs/schemas/*`).

### Intelligent commands (Phase 4)
- `insights subscriptions/fees/leaks` (client-side heuristics first)
- `plan budget/forecast/runway`
- `propose rules` + `apply rules --apply`

