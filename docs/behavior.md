# How sure-cli behaves

The detail behind the [README](../README.md): errors, retries, paging, the config file and how amounts are read. A command's own rules are in `sure-cli <command> --help`.

## Errors

A failing command exits 1 and prints an envelope with `error.code`, `error.message` (Sure's own message when Sure sent one) and `error.details`. [`internal/errors/errors.go`](../internal/errors/errors.go) lists every code.

`error.details` can hold:

- `status`: the HTTP status.
- `body`: Sure's response, capped.
- `api_error`: Sure's machine code, such as `insufficient_scope`, `feature_disabled` or `export_not_ready`.
- `import_id`: set when Sure created an import before failing. Read that import instead of repeating the request.
- `retry_after` and `rate_limit`: set on a 429.

Input the CLI rejects before sending anything fails with `validation_failed`.

Codes that need care before repeating a request:

- **`outcome_unknown`**: a POST, PATCH or DELETE reached Sure but the answer was lost (timeout or dropped connection), so the change may have been applied. `details.may_have_applied` is `true`. Read the resource back with `list` or `show` before trying again. A write that never left the CLI, such as one refused at connect, keeps `network_error`.
- **`invalid_response`**: Sure, or something in front of it such as a captive portal or an SSO proxy answering with HTML, returned a 2xx the CLI cannot read. It is also used for a body over 64 MiB, a page with more rows than were asked for, and a download shorter than the export's `byte_size`. `details.status` says whether a write was accepted (for example `201`), so do not repeat a POST that failed this way.
- **`redirect_refused`**: Sure's API never redirects. A 3xx usually comes from an `http://` `api_url` sent to `https://`, or from an SSO proxy. The response gives `details.location`, and no API key, token or password is sent to the other address. Set `api_url` to the address that serves the API.
- **`unknown_command`**: a subcommand that does not exist, with cobra's `suggestions`. For a command an earlier version had, `details.hint` names its replacement (see the [CHANGELOG](../CHANGELOG.md)).

`meta.warnings` lists what a command could not do and worked around, such as reading the family's time zone. Each warning has a `code`, `message` and `details`, and `data` is still valid.

## Retries and timeouts

Only GET, HEAD, PUT and OPTIONS are retried, on network errors and 5xx, and on a 429 whose `Retry-After` is at most 5 s. POST, PATCH and DELETE are never retried: Sure may have committed the change before the answer was lost. A 2xx body that cannot be decoded is never retried. A retry whose wait would outlast `--timeout` is not attempted, so the command reports Sure's last answer (for example a 503) instead of `timeout`.

`--timeout` (default `30s`, or the `timeout` config key) bounds each API call, retries included. It takes a duration (`90s`, `2m`) or a number of seconds (`90`); `0` disables it.

- **`family-exports download`:** the timeout bounds the wait for the response and each pause in the transfer, so a large export that keeps flowing is not cut off. Ctrl-C cancels it, also while it looks the export up, and removes the partial file. A second Ctrl-C kills the process.
- **Uploads (`imports create`, `account-statements upload`, ...):** the timeout bounds each pause in sending the file, then the wait for Sure's answer.

## Lists across pages

`--all`, `status`, `analyze`, `propose` and `export` read every page, following the page count Sure reports on each page. They stop at 1000 pages, or 10,000 for `export transactions`, which writes rows as they arrive. Past that the command fails with `too_many_pages`: narrow the window, or use `family-exports` for a full export.

When Sure's `X-RateLimit-Remaining` cannot cover the pages left, the command fails after the first page with `rate_limited`, `details.needed_requests` and `details.rate_limit`, instead of spending the hour's budget.

If the list loses rows while it is read, offset paging may skip one. The read then fails with `list_changed`; run the command again. Two signs trigger it: `total_count` drops, or fewer rows are read than the first page counted. Rows added meanwhile do not fail the read, and a row served twice is kept once.

One case is not detected. If a row already read is deleted while another is added further down, the count stays the same. The rows after the deleted one move up a place, and the one that moves onto a page already read is missed without an error. Sure's offset paging gives no way to tell, so use `family-exports` for an export that must be complete.

## Config file and tokens

The config lives at `~/.config/sure-cli/config.yaml` (`--config` picks another file).

**Writes:**

- The file is written atomically with owner-only permissions (0600). A file you made stricter (0400) keeps its mode; a looser one is tightened to 0600.
- Only the keys a command changed are merged into the file as it is on disk, under a lock file (`.config.yaml.lock` next to it). Concurrent commands therefore do not undo each other's changes or a token rotation.
- A symlinked config is written through to its target, even one that does not exist yet.
- The CLI refuses to replace a config file another user owns, so `sudo sure-cli ...` cannot hand yours to root.
- Temp files left by a crashed save are removed on the next run.

**`api_url`** must be `https://`, except for this machine (`localhost`, `*.localhost`, `127.0.0.0/8`, `::1`). Any other `http://` URL fails with `validation_failed` before a request is sent, because credentials would cross the network unencrypted. Pass `--allow-insecure-http` or set `allow_insecure_http: true` to accept that, for example on a trusted LAN. This check and the timeout check run only for commands that call Sure, so `config` can repair either setting, and `version`, `help` and `completion` still run.

**Logging in:**

- `login` prompts for the email (unless `--email` is given) and the password. On a terminal the password is hidden.
- With two-factor authentication on, login fails with `mfa_required` until you pass `--otp`.
- Sure revokes a device's previous tokens on every login. The first login therefore stores a random per-install `auth.device.device_id`, so logging in on another machine does not log this one out. A `device_id` you set yourself is kept. The other device fields live under `auth.device.*`; `device_type` is `ios`, `android` or `web`.

**Refreshing tokens:**

- Before `login`, `refresh` or an automatic refresh, the CLI checks that the config file is writable, and fails with `config_save_failed` if it is not, because Sure revokes the old tokens when it issues new ones.
- If saving still fails, the new tokens go to a new owner-only file in the user cache directory, and `details.rescue_file` gives its path. That directory is `~/.cache/sure-cli/` on Linux, `~/Library/Caches/sure-cli/` on macOS and `%LocalAppData%\sure-cli\` on Windows. Copy `auth.token` and `auth.refresh_token` from it with `sure-cli config set`, then delete it.
- Tokens never appear in the output.
- When two commands refresh at once, the one Sure rejects picks up the tokens the other saved.

## Amounts and totals

**Sign conventions:** Sure stores amounts in the entry convention: positive is an outflow (expense), negative an inflow (income). Its `amount` field is a localized display string rounded to the currency's precision (`"-€2,000.00"`). `analyze`, `propose` and `export` use the CLI convention instead: **income positive, expense negative**.

**Where those commands read the amount:**

1. `amount_decimal`: the stored decimal(19,4) value, exact and locale-independent (Sure #4422).
2. On an older server, `signed_amount_cents` (Sure #899), divided by the currency's minor unit (100 for EUR, 1 for JPY, 1000 for KWD).
3. Only without either, the localized `amount`, signed from `classification`. Its separators depend on the server's locale, so the CLI infers them: a `.` or `,` that repeats, or one followed by exactly three digits in a currency without three decimals, groups thousands.

**Rows left out of their totals,** which approximate Sure's income statement (`status` asks Sure for its own figures):

- `split_parent: true`: the children carry the amount, so counting the parent would count it twice.
- `excluded: true`: the user left the row out of budgets and reports.
- `kind` `funds_movement`, `cc_payment` or `one_time` (Sure's `BUDGET_EXCLUDED_KINDS`): transfers between your own accounts, card payments and one-offs.
- Rows on accounts marked `exclude_from_reports`, or not draft or active (absent from `accounts list` without `--include-disabled`).

`loan_payment` rows count as expenses whatever their sign, as in Sure. `investment_contribution` rows count as expenses too. That is Sure's default, but a family can turn off `investment_contributions_as_spending`, and the API does not say whether it did.

**Rows the CLI counts but Sure leaves out,** because the API gives no way to spot them:

- rows on tax-advantaged accounts (401k, IRA, ISA, HSA, ...);
- rows on accounts the user took out of their finances (`include_in_finances`);
- pending rows;
- investment-account rows labelled Transfer, Sweep In, Sweep Out or Exchange.

A family with any of these sees CLI totals that differ from Sure's.

## Dates

Windows that end today (`status`, `analyze`, `propose`, `export`) end on the family's date. `transactions create`, `valuations create` and `trades create` without `--date` use that date as well.

The time zone is the one `family-settings show` reports, resolved as Sure does: a Rails name such as `Madrid` or `Eastern Time (US & Canada)` maps to its IANA zone, and no zone, or one Sure does not know, means UTC. When the settings cannot be read, the local zone stands in and `meta.warnings` holds the failed request's code and message.

A `--months` window ending on the last day of a month covers whole calendar months.
