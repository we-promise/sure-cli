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
- **API errors** set `error.code` from a stable CLI set (`bad_request`, `auth_required`, `mfa_required`, `auth_invalid` for a missing scope, `forbidden`, `not_found`, `conflict`, `validation_failed`, `rate_limited`, `server_error`, `network_error`, ...) and `error.message` to Sure's own message. `error.details` holds `status`, the capped `body`, Sure's machine code as `api_error` (e.g. `insufficient_scope`, `feature_disabled`, `export_not_ready`), and on 429 `retry_after` and `rate_limit`
- **Retries**: only idempotent requests (GET/HEAD/PUT/DELETE/OPTIONS) are retried, on network errors and 5xx, and on 429 when `Retry-After` is at most 5 s. POST/PATCH are never retried, since Sure may have persisted the write before failing
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
sure-cli accounts list --all                 # every page (Sure serves at most 100 per page)
sure-cli accounts list --include-disabled    # disabled accounts are hidden by default
sure-cli accounts show <account_id>

# Transactions
sure-cli transactions list --start-date 2026-01-01 --end-date 2026-02-01 --per-page 50 --format=table
sure-cli transactions show <transaction_id>

# Safe writes (default is --dry-run)
# --amount is a positive decimal sent exactly as typed (Sure stores 4 decimals);
# --nature (expense|income|outflow|inflow) gives the sign.
sure-cli transactions create --amount 12.34 --nature expense --date 2026-02-04 --name "Coffee" --account-id <id>
sure-cli transactions create --amount 12.34 --date 2026-02-04 --name "Coffee" --account-id <id> \
  --category-id <category_id> --merchant-id <merchant_id> --tag-id <tag_id> --tag-id <tag_id> --apply
# Idempotent create: a retry with the same --external-id (+ --source, default "api") returns the existing row
sure-cli transactions create --amount 12.34 --date 2026-02-04 --name "Coffee" --account-id <id> \
  --external-id bank-ref-123 --source my-importer --user-modified --apply

sure-cli transactions update <tx_id> --name "Coffee (fixed)"
sure-cli transactions update <tx_id> --merchant-id <merchant_id> --tag-id <tag_id> --apply
sure-cli transactions update <tx_id> --clear-tags --apply   # without --tag-id/--clear-tags tags are kept
sure-cli transactions delete <tx_id> --apply

# Imports and family exports
sure-cli imports list --type TransactionImport
sure-cli imports rows <import_id>
sure-cli imports create --file data.csv --date-col-label Date --amount-col-label Amount --name-col-label Name
sure-cli imports create --file bank-export.csv --rows-to-skip 3 --date-col-label Date --amount-col-label Amount
sure-cli imports create --file backup.ndjson --publish --apply   # .ndjson/.json defaults to --type SureImport

# Chunked Sure NDJSON import (create -> add-chunk... -> publish)
sure-cli import-sessions create --expected-chunks 2 --apply
sure-cli import-sessions add-chunk <session_id> --sequence 1 --file part-1.ndjson --apply
sure-cli import-sessions add-chunk <session_id> --sequence 2 --raw-content "$(cat part-2.ndjson)" --apply
sure-cli import-sessions show <session_id>
sure-cli import-sessions publish <session_id> --apply
sure-cli family-exports create
sure-cli family-exports create --apply

# Statement Vault (archives PDF/CSV/XLSX statements, max 25 MB; never imports transactions)
sure-cli account-statements list --review-status unmatched --start-date 2026-01-01
sure-cli account-statements list --content-sha256 "$(shasum -a 256 march.pdf | cut -d' ' -f1)"
sure-cli account-statements show <statement_id>
sure-cli account-statements upload --file march.pdf --account-id <account_id>          # dry-run
sure-cli account-statements upload --file march.pdf --account-id <account_id> --apply  # 200 + meta.duplicate if already stored
sure-cli family-exports download <export_id> --out sure-export.zip

# Reference data and rules
sure-cli categories list --roots-only
sure-cli categories show <category_id>
sure-cli categories create --name Food --color '#3b82f6'
sure-cli categories create --name Food --color '#3b82f6' --icon utensils --parent-id <parent_id> --apply
sure-cli merchants list   # every merchant in one array (the endpoint has no pagination)
# CSV columns: name (required), color, website_url; existing names are skipped
sure-cli merchants import --file merchants.csv
sure-cli merchants import --file merchants.csv --apply
sure-cli tags create --name Travel --color '#3b82f6'
sure-cli tags create --name Travel --color '#3b82f6' --apply
sure-cli rules list --active true
sure-cli rule-runs list --status success --execution-type immediate   # manual|scheduled|immediate

# Budgets
sure-cli budgets list --start-date 2026-01-01
sure-cli budgets show <budget_id>
sure-cli budget-categories list --budget-id <budget_id>
sure-cli budget-categories show <budget_category_id>

# Insights: the feed Sure generates nightly (spending anomalies, cash flow
# warnings, subscription audits, bank fees, budgets at risk, ...).
# Needs Preview features on (Sure: Settings > Preferences); otherwise 403
# forbidden with details.api_error feature_disabled. No filters or paging.
sure-cli insights list
# JSON only: --format table prints the same JSON.
sure-cli insights ack <insight_id>            # dry-run; the id must be a UUID
sure-cli insights ack <insight_id> --apply    # read_write scope; guests refused; idempotent

# Sure answers 204 also when the insight was already acknowledged or expired,
# so ack reports {"id": ..., "in_feed": false}, not that this call acknowledged it.
# Analyze: client-side heuristics Sure does not compute (JSON only)
sure-cli analyze leaks --months 3            # or --start-date/--end-date; --currency to keep one
sure-cli analyze runway --account-id <id> --days 90
# runway: balance from balance_cents; burn is the whole family's reportable
# expenses in the account currency, from every account, over the last --days
# days (today included), extrapolated to 30-day months.

# Propose automations
sure-cli propose rules --months 3
sure-cli propose rules --months 3 --apply --min-confidence 0.8

# Export
sure-cli export transactions --months 12 --out-format csv --out transactions.csv

# Financial history
sure-cli balance-sheet show
sure-cli cash-flow show                                   # current month
sure-cli cash-flow show --month 2026-03 --include sankey  # summary + Sankey graph
sure-cli cash-flow show --view sankey --group-by account --start-date 2026-01-01 --end-date 2026-03-31
sure-cli spending-vs-normal show                          # current month
sure-cli spending-vs-normal show --start-date 2026-01-01 --end-date 2026-03-31
sure-cli balances list --account-id <account_id> --start-date 2026-01-01
sure-cli family-settings show
sure-cli valuations create --account-id <account_id> --amount 123.45 --date 2026-05-01
sure-cli valuations create --account-id <account_id> --amount 123.45 --date 2026-05-01 --upsert --apply

# Status (financial snapshot)
# net_worth comes from /api/v1/balance_sheet (family currency, FX-converted,
# excludes exclude_from_reports accounts). accounts.by_currency totals assets,
# liabilities, net worth and cash per native currency with the same exclusions.
# Runway uses depository cash in the family currency only.
# monthly.subscriptions is Sure's active recurring outflows (no income, no
# transfers, no one-offs) priced per month from each series' cadence, in the
# family currency; subscriptions_by_currency lists every currency apart.
# budget_pacing lists each current budget in its own currency (budgeted,
# actual and available spending, percent spent vs percent of period elapsed).
sure-cli status

# Holdings (requires Sure investment API)
sure-cli holdings list --account-id <account_id> --date 2026-05-01
sure-cli holdings show <holding_id>

# Securities and prices
sure-cli securities list --ticker AAPL
sure-cli securities show <security_id>
sure-cli security-prices list --security-id <security_id> --start-date 2026-01-01
# Import closes for an offline security no provider tracks (self-hosted Sure only).
# CSV header with date,price,currency, or JSON [{"date","price","currency"}] / {"prices":[...]}.
# 1-1000 rows, unique ISO dates, price > 0 with at most 4 decimals, one currency.
sure-cli security-prices create --security-id <security_id> --file closes.csv
sure-cli security-prices create --security-id <security_id> --file - --apply < closes.json

# Trades (requires Sure investment API)
# create --type: buy|sell|sweep_in|sweep_out|reinvestment need --qty, --price and one of
#   --security-id/--ticker/--manual-ticker (--fee optional); dividend needs --amount and a security;
#   interest|fee need --amount (security optional); deposit|withdrawal need --amount
#   (--transfer-account-id optional). Sure sets the activity label from the type.
# create returns a trade for share types, dividend, interest and fee; deposit/withdrawal return a
#   transaction, or a transfer when --transfer-account-id is set.
# update --type: buy|sell|sweep_in|sweep_out|reinvestment; --investment-activity-label must then
#   be that type's label ("Buy", "Sell", "Sweep In", "Sweep Out", "Reinvestment").
sure-cli trades list
sure-cli trades show <trade_id>
sure-cli trades create --account-id <account_id> --date 2026-05-01 --type buy --qty 1 --price 100 --security-id <security_id>
sure-cli trades create --account-id <account_id> --date 2026-05-01 --type buy --qty 1 --price 100 --security-id <security_id> --apply
sure-cli trades create --account-id <account_id> --date 2026-05-01 --type sell --qty 2 --price 101.25 --fee 1.50 --ticker AAPL --apply
sure-cli trades create --account-id <account_id> --date 2026-05-01 --type dividend --amount 12.34 --security-id <security_id> --apply
sure-cli trades create --account-id <account_id> --date 2026-05-01 --type interest --amount 3.10 --apply
sure-cli trades create --account-id <account_id> --date 2026-05-01 --type deposit --amount 1000 --transfer-account-id <checking_id> --apply
sure-cli trades update <trade_id> --type sweep_in --qty 5 --apply
sure-cli trades delete <trade_id> --apply

# Recurring transactions
sure-cli recurring-transactions list --status active
sure-cli recurring-transactions create --name Rent --last-occurrence-date 2026-04-01 --next-expected-date 2026-05-01
sure-cli recurring-transactions create --name Rent --last-occurrence-date 2026-04-01 --next-expected-date 2026-05-01 --apply
# --status accepts active|inactive (the API writes only these)
sure-cli recurring-transactions update <id> --status inactive --autopay true \
  --payment-url https://example.com/pay --notes "paid by direct debit" --apply

# Account reset and deletion
sure-cli users reset
sure-cli users reset --apply
sure-cli users reset status
sure-cli users delete-me
sure-cli users delete-me --apply

# Sync (trigger a one-shot sync)
sure-cli sync

# Transfers (categorized transfers, payments, loan payments)
sure-cli transfers list --status pending --account-id <account_id> --start-date 2026-01-01
sure-cli transfers show <transfer_id>

# Rejected transfer suggestions
sure-cli rejected-transfers list --account-id <account_id>
sure-cli rejected-transfers show <rejected_id>

# Sync history (list/inspect background syncs)
sure-cli syncs list --per-page 25
sure-cli syncs latest
sure-cli syncs show <sync_id>

# API usage + rate-limit info (singleton)
sure-cli usage show

# Version: local build info (plain text, offline); --server adds the Sure
# server's {version, commit_sha} as a JSON envelope. An unreachable server is
# reported under data.server_error and does not fail the command.
sure-cli version
sure-cli version --server

# Import preflight (validate config + content before creating)
sure-cli imports preflight --type TransactionImport --file data.csv \
  --date-col-label Date --amount-col-label Amount --name-col-label Name
sure-cli imports preflight --type SureImport --raw-file-content "$(cat backup.ndjson)"

# Provider connections (aggregator status)
sure-cli provider-connections list

# AI chats (requires AI enabled on the account)
sure-cli chats list --page 1
sure-cli chats show <chat_id> --page 1
sure-cli chats create --title "Tax planning" --apply
sure-cli chats create --title "Brainstorm" --message "Let's start" --model gpt-4o --apply
sure-cli chats update <chat_id> --title "Renamed" --apply
sure-cli chats delete <chat_id> --apply
sure-cli chats messages create --chat-id <chat_id> --content "Hello" --apply
sure-cli chats messages retry --chat-id <chat_id> --apply

# Auth (account-level writes)
sure-cli auth enable-ai           # dry-run
sure-cli auth enable-ai --apply
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

If the account has two-factor authentication, login fails with `error.code=mfa_required` until you pass `--otp`.

Required device payload fields are stored under `auth.device.*` in config (defaults are provided; `device_type` accepts `ios`, `android` or `web`). Sure revokes a device's previous tokens on every login, so the first login stores a random per-install `auth.device.device_id`; logging in on another machine no longer logs this one out. A custom `device_id` you set is kept.

## Docs

- Roadmap: `docs/ROADMAP.md`
- ADR: `docs/ADR-001-go-agent-first.md`
- JSON Schemas: `docs/schemas/v1/`
- Smoke test: `tools/smoke-oauth.sh`

## Amounts and Totals

Sure stores transaction amounts in the entry convention: positive is an outflow (expense), negative an inflow (income). The `amount` field is a localized display string rounded to the currency's precision (`"-€2,000.00"`).

`status`, `analyze`, `propose` and `export` read `amount_decimal` instead: the stored decimal(19,4) value, exact and locale-independent (Sure #4422). They flip it to the CLI convention, **income positive, expense negative**. Against a server without `amount_decimal` they fall back to parsing `amount` and take the sign from `classification`.

Totals count what Sure's own income statement counts. These rows are skipped:

- `split_parent: true`: the split's children carry the amount, so counting the parent would count it twice.
- `excluded: true`: the user left the row out of budgets and reports.
- `kind` `funds_movement`, `cc_payment` or `one_time` (Sure's `BUDGET_EXCLUDED_KINDS`): transfers between your own accounts, card payments and one-offs.

`export transactions` writes every row. Its `amount` column is the exact signed decimal; `kind`, `excluded` and `split_parent` columns tell you which rows to leave out of a sum.

## Heuristics

`analyze leaks` groups the window's expenses by the merchant Sure matched (by id, else by name), falling back to the transaction name, and keeps one group per currency: amounts in different currencies are never added.

A group is a leak with at least `--min-count` expenses (default 3), a total of at least 1500 and an average of at most 1000 minor units of its currency: 15.00 and 10.00 in EUR or USD, ¥1500 and ¥1000 in yen. Minor units follow Sure's currency table, so for currencies with a tiny minor unit (BTC, ETH) the defaults are close to zero; pass your own thresholds there. `--min-total` and `--max-avg` are major units of one currency and need `--currency`; `--min-total 0` keeps groups of any total:

```bash
sure-cli analyze leaks --months 3 --min-count 5
sure-cli analyze leaks --start-date 2026-01-01 --end-date 2026-03-31 --currency EUR --min-total 20 --max-avg 5
sure-cli analyze leaks --currency JPY --min-total 3000 --max-avg 800
```

The output echoes the window (`start`, `end`, inclusive, and `days`) and the thresholds applied (`min_count`, `min_total_minor`, `max_avg_minor`, `currency`). Candidate amounts are major units rounded to the currency's minor unit.

See `docs/ROADMAP.md` for planned features.
