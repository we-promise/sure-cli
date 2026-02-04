# sure-cli

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

