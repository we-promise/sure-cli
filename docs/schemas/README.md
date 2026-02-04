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
- `insights_subscriptions.schema.json` — `insights subscriptions`
- `insights_fees.schema.json` — `insights fees`
- `insights_leaks.schema.json` — `insights leaks`

### Plan
- `plan_budget.schema.json` — `plan budget`
- `plan_forecast.schema.json` — `plan forecast`
- `plan_runway.schema.json` — `plan runway`

### Automation
- `propose_rules.schema.json` — `propose rules`

CI validates samples against schemas on every push.
