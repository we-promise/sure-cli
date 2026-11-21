# sure-cli

A command-line client for [Sure](https://github.com/we-promise/sure), the self-hosted personal finance app. It is built for scripts and AI agents:

- every command prints JSON in one stable envelope;
- errors carry stable codes;
- every write is a dry run until you add `--apply`.

## Install

```bash
brew install we-promise/tap/sure-cli                                                  # macOS, Linux
curl -sSL https://raw.githubusercontent.com/we-promise/sure-cli/main/install.sh | bash
go install github.com/we-promise/sure-cli@latest
```

Binaries are on the [releases page](https://github.com/we-promise/sure-cli/releases). To upgrade, run `brew upgrade sure-cli` or the same install command again.

## Quick start

```bash
sure-cli config set api_url https://sure.example.com

# Authenticate with an API key...
sure-cli config set auth.mode api_key
sure-cli config set auth.api_key <key>
# ...or with your Sure login (prompts for the password; add --otp with 2FA)
sure-cli login --email you@example.com

sure-cli whoami
sure-cli accounts list --format table
sure-cli transactions list --start-date 2026-01-01 --end-date 2026-01-31 --format table
sure-cli status
```

Writes print the request they would send. Add `--apply` to send it:

```bash
sure-cli transactions create --account-id <id> --name Coffee --amount 3.50 --nature expense
sure-cli transactions create --account-id <id> --name Coffee --amount 3.50 --nature expense --apply
```

## Output

Commands print one JSON envelope on stdout (`help`, `completion` and `version` without `--server` print text):

```json
{ "data": { ... }, "meta": { "status": 200 } }
```

A failure exits 1 with an `error` instead:

```json
{ "error": { "code": "not_found", "message": "Tag not found", "details": { "status": 404, "api_error": "not_found", ... } } }
```

- **Error codes** are stable: `validation_failed`, `auth_required`, `not_found`, `rate_limited`, `outcome_unknown` and the others listed in [`internal/errors/errors.go`](internal/errors/errors.go). `outcome_unknown` means a write may have been applied even though its answer was lost; read the resource back before repeating it.
- **`meta.warnings`** lists anything a command worked around while still returning valid `data`.
- **`--format table`** prints a table for the list commands that have one. It is for people; scripts should read the JSON.
- **Schemas:** the JSON shapes are in [`docs/schemas/`](docs/schemas/README.md). Changes that break scripts are listed in the [CHANGELOG](CHANGELOG.md).

[docs/behavior.md](docs/behavior.md) covers errors, retries, timeouts, paging, the config file and how amounts are read.

## Commands

`sure-cli <command> --help` gives each command's flags and rules.

| Area | Commands |
|---|---|
| Accounts and transactions | `accounts`, `transactions`, `transfers`, `rejected-transfers`, `recurring-transactions`, `valuations`, `balances` |
| Organizing | `categories`, `merchants`, `tags`, `rules`, `rule-runs`, `budgets`, `budget-categories` |
| Reports | `status`, `balance-sheet`, `cash-flow`, `spending-vs-normal`, `insights` |
| Analysis | `analyze leaks`, `analyze runway`, `propose rules` (computed by the CLI, not by Sure) |
| Investments | `holdings`, `securities`, `security-prices`, `trades` |
| Import and export | `imports`, `import-sessions`, `export transactions`, `family-exports`, `account-statements` |
| Account and sync | `sync`, `syncs`, `provider-connections`, `family-settings`, `usage`, `users`, `auth`, `chats` |
| The CLI | `config`, `login`, `refresh`, `whoami`, `version`, `completion` |

Some commands need Sure features that may be off: `insights` needs Preview features, `chats` needs AI enabled, and `holdings` and `trades` need Sure's investment API.

## Examples

```bash
# Export a year of transactions (the file is readable only by you)
sure-cli export transactions --months 12 --out transactions.csv

# Retag a transaction (without --tag-id or --clear-tags, tags are kept)
sure-cli transactions update <id> --tag-id <tag> --tag-id <tag> --apply

# Create a transaction only once, even if the script retries
sure-cli transactions create --account-id <id> --name Rent --amount 900 \
  --external-id bank-ref-123 --source my-importer --apply

# Check and import a bank CSV
sure-cli imports preflight --type TransactionImport --file bank.csv \
  --date-col-label Date --amount-col-label Amount --name-col-label Name
sure-cli imports create --file bank.csv \
  --date-col-label Date --amount-col-label Amount --name-col-label Name --apply

# Full backup of the family's data
sure-cli family-exports create --apply
sure-cli family-exports download <export_id> --out sure-export.zip

# Cash flow for a quarter, grouped by account
sure-cli cash-flow show --view sankey --group-by account --start-date 2026-01-01 --end-date 2026-03-31

# Find small recurring spending, and categorization rules to apply
sure-cli analyze leaks --months 3
sure-cli propose rules --months 3
```

## Amounts

Sure stores an expense as a positive amount and income as a negative one. `analyze`, `propose` and `export` flip that: **income positive, expense negative**. They read the exact stored decimal, not the localized display string.

Their totals skip split parents, excluded rows, transfers between your own accounts and card payments, as Sure does. Some rows Sure also skips cannot be detected through the API, so the CLI's totals can differ from Sure's. `status` takes its income and spending from Sure itself. See [docs/behavior.md](docs/behavior.md#amounts-and-totals).

## More

- [CHANGELOG](CHANGELOG.md): what changed for scripts and agents, release by release.
- [docs/behavior.md](docs/behavior.md): errors, retries, paging, config file, amounts and dates in detail.
- [docs/schemas/](docs/schemas/README.md): JSON Schemas for the output.
- [docs/ADR-001-go-agent-first.md](docs/ADR-001-go-agent-first.md): why the CLI is built this way.
- [docs/ROADMAP.md](docs/ROADMAP.md): planned work.
- [docs/security/](docs/security/): attack surface and pentest notes.
- `tools/smoke-oauth.sh`: OAuth smoke test against a live server.
