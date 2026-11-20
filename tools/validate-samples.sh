#!/usr/bin/env bash
# Validates each docs/examples/<name>.json against the envelope schema and
# docs/schemas/v1/<name>.schema.json.
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$root"
go run ./tools/validate.go "$root"
