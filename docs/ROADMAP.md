# Roadmap

Future features and ideas for `sure-cli`.

## Planned

### Insights
- `insights merchants --top 20` — top merchants by spend/count
- `insights anomalies` — unusual transactions detection

### Search
- `search "query"` — full-text search across transactions

### Budgets
- `budgets list/show/create/update` — budget CRUD (requires upstream API)

## Ideas (no timeline)

### FIRE / Longevity Planning
- `plan fire --spend 45000 --return 4%` — FIRE calculations
- `plan longevity --age 42 --target-age 95` — longevity planning

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

- Import row diagnostics via `imports rows`.
- Import dry-run validation via `imports preflight`.
- CSV and Sure NDJSON import creation via `imports create`.
- Family export list/show/create/download commands.
- Transfer review surface via `transfers list/show` and `rejected-transfers list/show`.
- Sync history surface via `syncs list/latest/show`.
- API usage / rate-limit visibility via `usage show`.

See [CHANGELOG](../CHANGELOG.md) or GitHub releases for shipped features.
