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
- `dry_run_request.schema.json` — `data` of a write command run without `--apply`

### Insights
- `insights_list.schema.json` — `insights list` (Sure's server feed)

### Analyze
- `analyze_leaks.schema.json` — `analyze leaks`
- `analyze_runway.schema.json` — `analyze runway`

### Automation
- `propose_rules.schema.json` — `propose rules`

## Examples

Each command schema has one example envelope in `docs/examples/<name>.json`. `tools/validate-samples.sh` checks that every schema has its example, that each example is a valid envelope, that its `data` matches `<name>.schema.json`, and that a `meta.schema` it declares names that file. CI runs the script, and `go test ./internal/schema` runs the same check.

Add a schema and its example together; the check fails on either alone.
