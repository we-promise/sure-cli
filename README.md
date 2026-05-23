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

## Install

**Homebrew (macOS/Linux)**
```bash
brew install we-promise/tap/sure-cli
```

**One-liner (macOS/Linux)**
```bash
curl -sSL https://raw.githubusercontent.com/we-promise/sure-cli/main/install.sh | bash
```

**Go install**
```bash
go install github.com/we-promise/sure-cli@latest
```

**Manual download**

[GitHub Releases](https://github.com/we-promise/sure-cli/releases)

## Upgrade

**Homebrew**
```bash
brew update && brew upgrade sure-cli
```

**One-liner** (re-run installer)
```bash
curl -sSL https://raw.githubusercontent.com/we-promise/sure-cli/main/install.sh | bash
```

**Go install**
```bash
go install github.com/we-promise/sure-cli@latest
```

## Usage

```bash
sure-cli --help

# Configure
sure-cli config set api_url http://localhost:3000

# Auth
sure-cli config set auth.mode api_key
sure-cli config set auth.api_key <key>
# or OAuth:
sure-cli login --email you@example.com --password "..." [--otp 123456]

# Accounts
sure-cli accounts list --format=table
sure-cli accounts list --format=json
sure-cli accounts show <account_id>

# Transactions
sure-cli transactions list --start-date 2026-01-01 --end-date 2026-02-01 --per-page 50 --format=table
sure-cli transactions show <transaction_id>

# Safe writes (default is --dry-run)
sure-cli transactions create --amount "-12.34" --date 2026-02-04 --name "Coffee" --account-id <id>
sure-cli transactions create --amount "-12.34" --date 2026-02-04 --name "Coffee" --account-id <id> --apply

sure-cli transactions update <tx_id> --name "Coffee (fixed)"
sure-cli transactions delete <tx_id> --apply

# Imports and family exports
sure-cli imports list --type TransactionImport
sure-cli imports rows <import_id>
sure-cli imports create --file data.csv --date-col-label Date --amount-col-label Amount --name-col-label Name
sure-cli imports create --file backup.ndjson --type SureImport --publish --apply
sure-cli family-exports create
sure-cli family-exports create --apply
sure-cli family-exports download <export_id> --out sure-export.zip

# Reference data and rules
sure-cli categories list --roots-only
sure-cli merchants list
sure-cli tags create --name Travel --color '#3b82f6'
sure-cli tags create --name Travel --color '#3b82f6' --apply
sure-cli rules list --active true
sure-cli rule-runs list --status success

# Budgets
sure-cli budgets list --start-date 2026-01-01
sure-cli budgets show <budget_id>
sure-cli budget-categories list --budget-id <budget_id>
sure-cli budget-categories show <budget_category_id>

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
sure-cli propose rules --months 3 --apply --min-confidence 0.8

# Export
sure-cli export transactions --months 12 --format csv --out transactions.csv

# Financial history
sure-cli balance-sheet show
sure-cli balances list --account-id <account_id> --start-date 2026-01-01
sure-cli family-settings show
sure-cli valuations create --account-id <account_id> --amount 123.45 --date 2026-05-01
sure-cli valuations create --account-id <account_id> --amount 123.45 --date 2026-05-01 --upsert --apply

# Status (financial snapshot)
sure-cli status

# Holdings (requires Sure investment API)
sure-cli holdings list --account-id <account_id> --date 2026-05-01
sure-cli holdings show <holding_id>

# Securities and prices
sure-cli securities list --ticker AAPL
sure-cli securities show <security_id>
sure-cli security-prices list --security-id <security_id> --start-date 2026-01-01

# Trades (requires Sure investment API)
sure-cli trades list
sure-cli trades show <trade_id>
sure-cli trades create --account-id <account_id> --date 2026-05-01 --type buy --qty 1 --price 100 --security-id <security_id>
sure-cli trades create --account-id <account_id> --date 2026-05-01 --type buy --qty 1 --price 100 --security-id <security_id> --apply

# Recurring transactions
sure-cli recurring-transactions list --status active
sure-cli recurring-transactions create --name Rent --last-occurrence-date 2026-04-01 --next-expected-date 2026-05-01
sure-cli recurring-transactions create --name Rent --last-occurrence-date 2026-04-01 --next-expected-date 2026-05-01 --apply

# Account reset and deletion
sure-cli users reset
sure-cli users reset --apply
sure-cli users reset status
sure-cli users delete-me
sure-cli users delete-me --apply

# Sync
sure-cli sync

# Transfers (categorized transfers, payments, loan payments)
sure-cli transfers list --status pending --account-id <account_id> --start-date 2026-01-01
sure-cli transfers show <transfer_id>

# Rejected transfer suggestions
sure-cli rejected-transfers list --account-id <account_id>
sure-cli rejected-transfers show <rejected_id>
```

## Auth

Sure supports OAuth bearer tokens and API keys.

- OAuth: `Authorization: Bearer <token>`
- API key: `X-Api-Key: <key>`

## OAuth login + refresh

```bash
sure-cli login --email you@example.com [--otp 123456]
# Password is prompted securely (hidden input)

# Or fully interactive:
sure-cli login
# Prompts for email and password

# Later (refresh access token using stored refresh token)
sure-cli refresh
```

Required device payload fields are stored under `auth.device.*` in config (defaults are provided).

## Docs

- Roadmap: `docs/ROADMAP.md`
- ADR: `docs/ADR-001-go-agent-first.md`
- JSON Schemas: `docs/schemas/v1/`
- Smoke test: `tools/smoke-oauth.sh`

## API Sign Convention

Sure API returns amounts with an accounting-style sign that may look inverted:

- `classification=income` can have **negative** `amount` string
- `classification=expense` can have **positive** `amount` string

**Rule:** treat `classification` as ground truth, not the sign. `sure-cli` normalizes this internally for all insights/heuristics.

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

See `docs/ROADMAP.md` for planned features.
