# Attack Surface Analysis -- sure-cli

**Date:** 2026-03-03
**Project:** sure-cli (Go CLI for Sure personal finance)
**Type:** CLI tool (no server, no daemon)

## Project Overview

sure-cli is a Go-based command-line tool that acts as an API client for a self-hosted
personal finance application called "Sure" (we-promise/sure). It handles OAuth
authentication, API key authentication, financial transaction management, CSV/JSON
export, and client-side heuristic analysis of financial data.

## Architecture

```
User (CLI args / stdin)
    |
    v
sure-cli (cobra commands)
    |
    +-- internal/config  (viper, YAML config at ~/.config/sure-cli/config.yaml)
    |      stores: api_url, auth.token, auth.refresh_token, auth.api_key
    |
    +-- internal/api     (go-resty HTTP client -> Sure API)
    |      sends: Bearer token or X-Api-Key header
    |      receives: JSON from Sure REST API
    |
    +-- internal/analyze  (client-side heuristics: spending leaks, runway)
    +-- internal/rules    (rule proposal engine)
    +-- internal/output   (JSON/table formatting)
    +-- internal/schema   (JSON schema validation, dev/tooling only)
```

## Entry Points

| Entry Point              | Type           | Trust Level |
|--------------------------|----------------|-------------|
| CLI arguments (cobra)    | User input     | Untrusted   |
| stdin (password prompt)  | User input     | Untrusted   |
| Config file (YAML)       | Local file     | Semi-trusted |
| Sure API responses       | Network/remote | Untrusted   |
| install.sh (curl\|bash)  | Network/remote | Untrusted   |
| --file flag (imports)    | Local file     | Untrusted   |
| --out flag (export)      | Local file     | User-controlled |

## Trust Boundaries

1. **User -> CLI**: All CLI arguments flow through cobra flag parsing (typed).
   No shell expansion, no `exec.Command`, no `os/exec` usage anywhere.

2. **CLI -> Config File**: Reads/writes `~/.config/sure-cli/config.yaml` via viper.
   Contains OAuth tokens and API keys in plaintext YAML.

3. **CLI -> Sure API**: HTTPS/HTTP requests via go-resty. Bearer tokens or API keys
   sent in headers. No TLS certificate pinning. Default `api_url` is `http://localhost:3000`.

4. **Sure API -> CLI**: JSON responses deserialized via `encoding/json`. No `eval`,
   no template execution, no dynamic code.

5. **CLI -> Filesystem**: Export writes files to user-specified path (`--out`).
   Import reads files from user-specified path (`--file`).

## Data Flows

### Authentication Flow
```
User --[email/password via stdin]--> sure-cli --[POST /api/v1/auth/login]--> Sure API
Sure API --[access_token, refresh_token]--> sure-cli --[save to config.yaml]--> Filesystem
```

### API Request Flow
```
sure-cli --[read token from config]--> viper
sure-cli --[check token expiry]--> auto-refresh if needed
sure-cli --[GET/POST/PUT/DELETE with auth header]--> Sure API
Sure API --[JSON response]--> sure-cli --[format + print]--> stdout
```

### Export Flow
```
sure-cli --[fetch transactions]--> Sure API
sure-cli --[write CSV/JSON]--> os.Create(user-provided path)
```

## Risk Areas

### High Risk
- **Credential storage in plaintext** (config.yaml contains tokens/keys)
- **Config file permissions** (directory created with 0o755, file permissions delegated to viper)
- **Default HTTP (not HTTPS)** api_url default is `http://localhost:3000`
- **install.sh curl|bash pattern** (standard but inherently risky)

### Medium Risk
- **`config set` allows arbitrary viper keys** (could overwrite any config value)
- **No input sanitization on URL path segments** (transaction IDs, account IDs interpolated into paths)
- **Smoke test script contains hardcoded test password**

### Low Risk
- **No TLS certificate validation customization** (relies on Go defaults, which is fine)
- **No subprocess execution** (no shell=True, no exec.Command, no backticks)
- **No regex usage** (no ReDoS risk)
- **No eval/exec of user input**

### Not Applicable
- **XSS/CSRF**: CLI tool, no web interface
- **SQL Injection**: No database, API client only
- **SSRF**: Does not proxy requests, fixed API URL
- **Deserialization attacks**: Standard JSON only, no gob/pickle/yaml unmarshalling of untrusted data
