# Roadmap

Future features and ideas for `sure-cli`.

## Planned

### Analyze
- `analyze merchants --top 20` — top merchants by spend/count

### Search
- `search "query"` — full-text search across transactions

### Budgets
- `budgets list/show/create/update` — budget CRUD (requires upstream API)

## Ideas (no timeline)

### FIRE / Longevity Planning
- `analyze fire --spend 45000 --return 4%` — FIRE calculations
- `analyze longevity --age 42 --target-age 95` — longevity planning

### Benchmarks
- `compare spend --category groceries --country ES` — peer comparisons
- Requires external benchmark data (plugin)

### Allocation Advice
- `advise allocation --framework bogleheads` — portfolio guidance
- `advise rebalance --target "60/40"` — rebalance suggestions
- Read-only advice; never executes trades

### Plugins
- Benchmark data plugin (cost-of-living averages)
- Market data plugin (quotes, ETF classification)
- Monte Carlo plugin (retirement simulations)

## Completed

- Sure's insights feed via `insights list` and `insights ack`; local subscription, fee, forecast and budget heuristics removed in favour of Sure's own.
- Client-side heuristics grouped under `analyze` (`leaks`, `runway`).
- Import row diagnostics via `imports rows`.
- Import dry-run validation via `imports preflight`.
- CSV and Sure NDJSON import creation via `imports create`.
- Family export list/show/create/download commands.
- Category creation via `categories create --name --color [--icon --parent-id]`.
- Transfer review surface via `transfers list/show` and `rejected-transfers list/show`.
- Sync history surface via `syncs list/latest/show`.
- API usage / rate-limit visibility via `usage show`.
- Provider connection inspection via `provider-connections list`.
- AI chats CRUD + message send/retry via `chats` subtree.
- Account-level AI enable via `auth enable-ai`.

See [CHANGELOG](../CHANGELOG.md) for unreleased changes and [GitHub releases](https://github.com/we-promise/sure-cli/releases) for shipped features.
