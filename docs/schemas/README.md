# JSON Schemas

Agent-first contracts for `sure-cli` outputs.

- Versioned under `docs/schemas/v1/`
- Default output is JSON (`--format=json`)
- Table output is best-effort and not schema-governed

## v1

### Core
- `envelope.schema.json` — top-level output envelope `{data, meta, error}`
- `accounts_list.schema.json` — `accounts list`
- `transactions_list.schema.json` — `transactions list`
- `dry_run_request.schema.json` — dry-run mode output

### Insights
- `insights_list.schema.json` — `insights list` (Sure's server feed)

### Analyze
- `analyze_leaks.schema.json` — `analyze leaks`
- `analyze_runway.schema.json` — `analyze runway`

### Automation
- `propose_rules.schema.json` — `propose rules`

CI validates samples against schemas on every push.
