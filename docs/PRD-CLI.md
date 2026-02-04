# PRD: Sure CLI

## Overview

A command-line interface for Sure, the self-hosted personal finance app.

## Problem Statement

Sure currently only offers a web UI. Power users, developers, and **LLM agents** need programmatic access to:
- Query financial data quickly
- Automate common operations
- Integrate with scripts and workflows
- Enable AI assistants to interact with Sure

## Jobs To Be Done (mapped to Sure JTBD)

| CLI Job | Sure JTBD | Commands |
|---------|-----------|----------|
| "Show me my financial snapshot" | #1 Complete picture | `sure status`, `sure net-worth` |
| "Where did my money go?" | #2 Spending patterns | `sure transactions`, `sure spending` |
| "Sync my accounts now" | #3 Auto-sync | `sure sync` |
| "How are my investments doing?" | #4 Investment tracking | `sure holdings`, `sure performance` |
| "Am I within budget?" | #5 Budget limits | `sure budget` |
| "Import my data" | #7 Data import | `sure import` |
| "Categorize this" | #8 Auto-organize | `sure categorize`, `sure rules` |

## Current Status (implemented)

This PRD started as a broad command sketch. The repo currently ships a working **agent-first** CLI named `sure-cli`.

### Implemented (today)

```bash
# Config + auth headers
sure-cli config set api_url http://localhost:3000
sure-cli config set token <access_token>      # OAuth bearer token
sure-cli config set api_key <key>             # API key

# Read-only core
sure-cli whoami
sure-cli accounts list [--page N --per-page N] [--format table|json]
sure-cli accounts show <account_id>           # falls back to list lookup (upstream 404)

sure-cli transactions list --from YYYY-MM-DD --to YYYY-MM-DD [--page N --per-page N] [--format table|json]
sure-cli transactions show <tx_id>

sure-cli sync

# Safe writes (default: --dry-run)
sure-cli transactions create ... [--apply]
sure-cli transactions update <tx_id> ... [--apply]
sure-cli transactions delete <tx_id> --apply

# Phase 4 (read-only heuristics)
sure-cli insights subscriptions --months 6
sure-cli insights fees --months 3
sure-cli insights leaks --months 3
```

### Not implemented yet (next)
- `sure-cli login` + refresh token flow (device payload required by Sure)
- Deeper typing + error hardening in the API layer beyond the window fetch helper

### Output Formats

```bash
sure transactions --format=json   # JSON output (default for piping)
sure transactions --format=table  # Human-readable table
sure transactions --format=csv    # CSV export
```

### Configuration

```bash
sure config                   # Show current config
sure config set api_url https://my-sure-instance.com
sure config set api_key <key>
```

Config stored in `~/.config/sure-cli/config.yaml`

## Intelligent CLI (beyond CRUD)

### Goal

Beyond CRUD, the CLI should **attempt to solve parts of the JTBD** directly: detect problems, propose actions, and (with explicit confirmation) execute safe automations.

This turns the CLI into a *finance co-pilot* that can be used by humans **and** by LLM agents via deterministic commands.

### Principles

- **Explain → Propose → Confirm → Execute**: default is read-only; any write/automation requires confirmation (`--apply`) and should support `--dry-run`.
- **Deterministic outputs**: every “intelligent” command must support `--format=json` with a stable schema.
- **Reproducibility**: every result includes the query window, filters, and assumptions.
- **Safety**: never place trades, move money, or contact third parties automatically.

### Intelligent Jobs & Commands

#### 1) "Help me find easy ways to cut costs" (subscriptions, fees, leakage)

```bash
sure insights leaks [--from DATE --to DATE] [--min-amount 5]
sure insights subscriptions [--months 6]
sure insights fees [--months 3]
sure insights merchants --top 20 [--by amount|count]
```

Output should include:
- suspected subscriptions (periodic cadence)
- bank fees (maintenance, ATM, overdraft)
- “ghost” merchants (no value, low frequency but high cost)
- proposed actions: tag, categorize, add rule, set alert

#### 2) "Am I on track?" (budget pacing + forecasting)

```bash
sure plan budget --month 2026-02
sure plan runway [--cash-account <id>] [--months 6]
sure plan forecast --days 30
```

- Budget pacing: expected spend vs actual so far (burn rate)
- Forecast: recurring + average daily spend
- Runway: cash buffer months based on burn rate

#### 3) "Compare my spend to peers/averages" (benchmarks)

```bash
sure compare spend --category groceries --country ES [--region GAL] [--household 4]
```

Notes:
- Needs external benchmark datasets (optional plugin). If none configured → gracefully degrade (explain missing data).
- Must label comparisons as **estimates**.

#### 4) "How much should I buy of X?" (allocation guidance)

```bash
sure advise allocation --asset "TSLA" --amount 1000 --framework "bogleheads" --risk medium
sure advise rebalance --target "60/40" --account <id>
```

Rules:
- Advice is **informational**, not financial advice. Provide ranges, rationale, and trade-offs.
- Requires user-provided `investment_policy.yml` (or interactive questionnaire) defining risk tolerance and targets.
- Never execute trades; at most produce a *trade plan*.

#### 5) "Help me plan my future" (FIRE, longevity, goals)

```bash
sure goals add "FIRE" --target 1500000 --horizon 12y
sure plan fire --spend 45000 --return 4% --inflation 2% --horizon 20y
sure plan longevity --age 42 --target-age 95 --spend 45000
```

Outputs:
- required savings rate
- probability bands (if Monte Carlo is enabled)
- sensitivity analysis (return/inflation/spend)

#### 6) Vendor lock-in / portability audit

```bash
sure export --format csv --out exports/
sure audit portability
```

- audits export completeness (transactions, accounts, holdings, categories, rules)
- produces a “portability score” and missing areas

### Automation Layer

Some intelligent commands can propose automations:

```bash
sure propose rules --from 2026-01-01 --to 2026-02-01
sure apply rules --plan plan.json --dry-run
sure apply rules --plan plan.json --apply
```

- `propose` generates rule suggestions (merchant → category, tags)
- `apply` executes them only with `--apply`

### Plugins (optional but recommended)

- **Benchmarks plugin**: pulls cost-of-living / spend averages (country/region/household)
- **Market data plugin**: quotes, ETF classifications (for holdings/performance)
- **Monte Carlo plugin**: retirement simulation

Plugins should be opt-in, explicitly configured, and isolated.

### API / Data Requirements for Intelligence

To support the above, we likely need additional endpoints beyond current `/api/v1`:
- `GET /api/v1/summary/net_worth`
- `GET /api/v1/summary/cashflow`
- `GET /api/v1/budgets` (+ pacing)
- `GET /api/v1/holdings` (+ asset class breakdown)
- `GET /api/v1/recurring` (subscriptions detection could also be client-side)
- `POST /api/v1/rules/propose` (optional; can start client-side)

### Phase 4: Intelligent CLI (after CRUD)

- Insights (subscriptions/fees/leaks)
- Planning (forecast/runway/FIRE)
- Advice (allocation/rebalance; plan-only)
- Automation plans (propose/apply rules)

## Technical Approach

### Option A: Ruby gem (recommended)
- Ships with Sure repo in `/cli` directory
- Reuses existing API client code
- `gem install sure-cli` or bundled with Docker
- Familiar to Rails contributors

### Option B: Go binary
- Single static binary, no dependencies
- Fast startup
- Cross-platform distribution via GitHub releases
- Separate repo: `we-promise/sure-cli`

### Option C: TypeScript/Node
- Modern tooling (oclif, commander)
- Easy JSON handling
- Familiar to frontend contributors

**Recommendation: Option A (Ruby gem)** for v1 — keeps it in-repo, reuses code, easy for existing contributors. Can always port to Go later for distribution.

## API Requirements

The existing `/api/v1` endpoints cover most needs:
- ✅ `GET /api/v1/accounts`
- ✅ `GET /api/v1/transactions`
- ✅ `POST /api/v1/transactions`
- ✅ `POST /api/v1/sync`
- ✅ `GET /api/v1/categories`
- ✅ `POST /api/v1/chats` + `/messages`

**Missing endpoints needed:**
- `GET /api/v1/net_worth` — aggregate net worth endpoint
- `GET /api/v1/budgets` — budget status endpoint
- `GET /api/v1/holdings` — investment holdings endpoint
- `GET /api/v1/search` — full-text search endpoint

## LLM Integration

The CLI enables AI assistants to:

```bash
# Agent queries Sure via CLI
$ sure transactions --from=2026-01-01 --category=groceries --format=json
[{"id": 123, "amount": -45.50, "merchant": "Whole Foods", ...}]

# Agent uses chat endpoint for natural language
$ sure chat "Am I on track with my grocery budget this month?"
"You've spent $245 of your $400 grocery budget (61%). At this pace..."
```

## Success Metrics

1. **Adoption**: 100+ CLI downloads in first month
2. **LLM usage**: CLI used in 10+ community AI integrations
3. **Contributor velocity**: Faster debugging/testing via CLI

## Phases

### Phase 1: MVP (2 weeks)
- `login`, `logout`, `whoami`
- `accounts`, `transactions` (list, show)
- `sync`
- JSON output

### Phase 2: Full CRUD (2 weeks)
- Transaction create/update/delete
- Tags CRUD
- Import command
- Table/CSV output formats

### Phase 3: Intelligence (2 weeks)
- `chat` command for AI interaction
- `budget`, `holdings`, `net-worth`
- `search` command

## Open Questions

1. Should CLI be in main repo or separate?
2. Ruby gem vs compiled binary for v1?
3. Auth: API key only, or also support OAuth device flow?
4. Should we support multiple Sure instances (profiles)?

---

*Author: @dgilperez + Wolfgang*
*Date: 2026-02-03*
*Status: Draft - Ready for Discussion*
