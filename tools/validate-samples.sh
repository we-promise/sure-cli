#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

validate() {
  local schema="$1"; local json="$2"
  echo "validate: $json"
  go test ./internal/schema -run TestDummy >/dev/null 2>&1 || true
  go run ./tools/validate.go --schema "$schema" --json "$json"
}

validate "$root/docs/schemas/v1/envelope.schema.json" "$root/docs/examples/accounts_list.json"
validate "$root/docs/schemas/v1/envelope.schema.json" "$root/docs/examples/transactions_list.json"
validate "$root/docs/schemas/v1/envelope.schema.json" "$root/docs/examples/insights_subscriptions.json"
validate "$root/docs/schemas/v1/envelope.schema.json" "$root/docs/examples/insights_fees.json"
validate "$root/docs/schemas/v1/envelope.schema.json" "$root/docs/examples/insights_leaks.json"
