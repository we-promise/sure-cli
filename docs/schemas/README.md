# JSON Schemas

Agent-first contracts for `sure-cli` outputs.

- Versioned under `docs/schemas/v1/`
- Default output is JSON (`--format=json`)
- Table output is best-effort and not schema-governed

## v1
- `envelope.schema.json` — top-level output envelope
- `accounts_list.schema.json` — `.data` for `accounts list`
- `transactions_list.schema.json` — `.data` for `transactions list`
- `dry_run_request.schema.json` — `.data` for commands in dry-run mode

Planned: add automated schema validation in CI (golden samples + jsonschema validator).
