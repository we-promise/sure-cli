# Changelog

Releases are tagged on [GitHub](https://github.com/we-promise/sure-cli/releases). This file lists what changes for scripts and agents that call `sure-cli`.

## Unreleased

### Breaking changes

#### Removed and renamed commands

| Before | Now |
|---|---|
| `insights leaks` | `analyze leaks` |
| `plan runway` | `analyze runway` |
| `plan budget` | `budgets list`, `budget-categories list`, `status` (`data.budget_pacing`) |
| `plan forecast` | `insights list` (Sure's cash flow insights) |
| `insights subscriptions` | `status` (`data.monthly.subscriptions`), `recurring-transactions list` |
| `insights fees` | `insights list` (Sure's bank fee insights) |
| `config heuristics`, `config fee-keywords` | none: leak thresholds are `analyze leaks` flags |

The `plan` group is gone. A removed command now exits 1 with `error.code` `unknown_command` and `error.details.hint` naming its replacement. Before, a command group given a subcommand it lacks printed its help and exited 0.

The `heuristics.*` config keys are no longer read.

#### Removed flags

- `imports create --file-format` and `--source`: Sure neither permits nor reads them.
- `merchants list --page` and `--per-page`: the endpoint returns every merchant.
- `transactions update --currency`: Sure ignores it on update.
- `trades create --category-id` and `--investment-activity-label`, `trades update --category-id`: Sure no longer permits a category, and it sets the activity label from `--type`.
- `trades update --currency`: Sure changed only the entry's currency, not the trade's, leaving the two apart.

#### Schemas

- `insights_leaks.schema.json` is now `analyze_leaks.schema.json`, and `plan_runway.schema.json` is now `analyze_runway.schema.json`. `meta.schema` names the new files.
- `insights_subscriptions`, `insights_fees`, `plan_budget` and `plan_forecast` schemas are deleted with their commands.

#### Error codes

- 403 is `forbidden`. `auth_invalid` now means only a missing scope (403 `insufficient_scope`). Before, every 403 was `auth_invalid`.
- 400 is `bad_request` and 409 is `conflict`; both were `unknown_error`.
- A 401 asking for a second factor is `mfa_required`, also from `login`.
- `login` and `refresh` report `login_failed` and `refresh_failed` only when Sure turns the credentials down (400, 401, 403 or 422). Any other failure keeps its own code (`redirect_refused`, `timeout`, `network_error`, `outcome_unknown`, `rate_limited`, `server_error`, `not_found`, `invalid_response`, `validation_failed`); every failure was `login_failed` or `refresh_failed`.
- `request_failed` is gone: `status`, `export`, `propose` and `analyze` report the classified code (`forbidden`, `not_found`, `server_error`, ...) with Sure's message and details.
- `account_not_found` is gone: `analyze runway` reads the account by id and reports `not_found`.
- `missing_account` and `invalid_format` are now `validation_failed`, as is every input the CLI rejects before sending a request, unknown flags and wrong argument counts included.
- `compute_failed`, `invalid_month`, `auth_expired`, `config_invalid` and `internal_error` are gone.
- New codes: `invalid_response` (a 2xx body the CLI cannot read, too large or cut short), `canceled` (Ctrl-C or SIGTERM), `unknown_command`, `outcome_unknown` (a write that reached Sure but whose answer was lost), `redirect_refused`, `too_many_pages` (a list past the page cap), `list_changed` (a list that changed while its pages were read), `apply_incomplete`.

`internal/errors/errors.go` lists every code the CLI emits.

#### Output

- `status`: `data.accounts.total_balance` is replaced by `data.net_worth` (`currency`, `net_worth`, `assets`, `liabilities`, from Sure's balance sheet) and `data.accounts.by_currency`. Entries of `data.accounts.list` add `classification` and `exclude_from_reports`. `data.budget_pacing` is now a list with one entry per current budget, in that budget's currency (`id`, `name`, `currency`, `start_date`, `end_date`, `initialized`, `budgeted_spending`, `actual_spending`, `available_to_spend`, `percent_spent`, `percent_elapsed`); it was one object with `month`, `days_elapsed`, `spent`, `projected` and `avg_per_day`. `data.monthly.subscriptions` comes from Sure's active recurring transactions, and `data.monthly.subscriptions_by_currency` is new. `data.alerts` is `[]` when empty. Amounts use each currency's own decimals.
- `status`: `data.monthly.income` and `data.monthly.expenses`, and the runway and alerts built on them, are Sure's figures from `GET /api/v1/cash_flow?view=sankey` over the same rolling month, which `data.monthly.period` now reports. They are FX-converted to the family currency, so other currencies count instead of being skipped; they leave out the accounts and rows Sure leaves out; and spending is netted per category, so refunds lower it. Totals can differ from earlier releases. `status` now needs Sure v0.7.5 or later, which has that endpoint; an older one fails with `not_found`. `data.runway` adds `partial` and `cash_currencies_left_out`: runway counts family-currency cash only, so with depository cash in other currencies it is partial and `low_runway` is not raised.
- `meta.status` appears only when `data` is a single Sure response. `status`, `analyze runway`, `export transactions` and `propose rules` no longer report `meta.status: 200`. `analyze runway` reports an account body without a readable balance as `invalid_response`.
- `analyze leaks` (was `insights leaks`): `data.params` echoes the thresholds applied as `min_count`, `min_total_minor`, `max_avg_minor` and, with `--currency`, `currency`, in place of `min_total` and `max_avg`. `data.window` adds `days`. Candidates add `merchant_id`, `grouped_by` and `currency`; amounts round to the currency's minor unit.
- `analyze runway`: `currency` is the account's own, not always `EUR`.
- `export transactions` CSV: `amount` is the exact signed decimal, income positive and expense negative, where it was Sure's localized display string (`"-€2,000.00"`). The columns are now `id, date, name, amount, currency, classification, kind, category, account, merchant, excluded, split_parent`. Every row is exported, split parents and excluded rows included; the new columns mark the rows to leave out of a sum.
- `propose rules --apply` exits 1 with `apply_incomplete` when any update fails; it exited 0. `error.details` holds the report (`applied`, `skipped`, `errors` and their counts), and each entry of `errors` has the update's `code`, `error` message and `details`. A 429 stops the run: the updates left are listed in `skipped` with `reason` `rate_limited`. Empty lists are `[]`; they were `null`.
- `export transactions` JSON: each row has the CSV columns as keys (`id`, `date` as `YYYY-MM-DD`, `name`, `amount` as the exact decimal text, `currency`, `classification`, `kind`, `category`, `account`, `merchant`, `excluded`, `split_parent`). Rows were the CLI's internal Go struct: `ID`, `EntryID`, `Amount` as a float, `AmountText`, `Date` with a time of day, and so on.
- `export transactions` CSV: a text cell that starts with `=`, `+`, `-`, `@`, a tab or a carriage return gets a leading `'`, so a spreadsheet reads it as text instead of running it as a formula. `amount` is unchanged.
- `export transactions` writes the file readable only by you (0600), through a temp file renamed into place: a failed export leaves no file and keeps an earlier one, and a symlink at `--out` is replaced, not followed. The default file name carries the window's end date. A file it replaces keeps its mode tightened to the owner's bits: 0644 becomes 0600 and 0400 stays 0400 (financial data is never left readable by others). `--out` naming an existing pipe, terminal, socket or other character device, such as `/dev/stdout` or `/dev/fd/3`, is written directly. `family-exports download --out` follows the same rules.
- `--format table` and the error line on stderr drop control characters from Sure's text, so a name or message cannot send terminal escape sequences.
- The `imports create` dry run lists the request fields; it printed the internal Go struct. A raw `import-sessions add-chunk` dry run shows `sequence` as a string.

#### Stricter input

- `sync` is a dry run unless `--apply` is given, like every other write; it triggered a sync straight away.
- Decimal flags are plain decimal text: `3.` and `.5` are rejected. This covers `valuations create` and `update --amount` and the `recurring-transactions create` amounts, which were sent as typed: Sure read `1,234.56` as 1 and `abc` as 0.
- `transactions create` and `update`: `--amount` must be a positive decimal with at most 4 decimals and 15 digits; `--nature` gives the sign. It was a float sent with two decimals. `update --nature` without `--amount` fails instead of being dropped.
- `trades create` and `update`: `--date` must be `YYYY-MM-DD`; Rails read `05/01/2026` as 5 January.
- `trades update`: `--name` cannot be combined with `--qty` or `--price`; Sure renames the trade from its ticker (`Buy 5 shares of AAPL`).
- `trades update`: `--type` or `--nature` needs `--qty` or `--price`; `--nature` is `inflow` or `outflow`; `--amount` cannot be combined with `--qty` or `--price`; qty, price and amount are checked against Sure's columns. `trades create --fee` must be 0 or more.
- `analyze leaks --min-total` and `--max-avg` are major units of `--currency` and require it. Without them the defaults are 1500 and 1000 minor units of each group's currency (15.00 and 10.00 EUR, ¥1500 and ¥1000).
- `propose rules --months` and `export transactions --months` of 0 or less fail with `validation_failed`; they fell back to 3 and 12. `analyze leaks --months` and `analyze runway --days` reject them too.
- `export transactions` checks `--out-format` before fetching anything.
- `insights ack` requires a UUID.
- `transactions list`: `--start-date`/`--end-date` (and `--from`/`--to`) must be `YYYY-MM-DD` and in order, `--min-amount`/`--max-amount` plain decimals (Sure's sign: an expense is positive), `--type` `income` or `expense`. Sure answered a bad date with a 500, read `abc` as 0 and ignored an unknown type.
- `recurring-transactions create` and `update`: `--status` accepts only `active` or `inactive`; `--expected-day-of-month` is a whole number from 1 to 31 and the dates are `YYYY-MM-DD`. `create` requires `--amount`, `--currency` and `--expected-day-of-month`, which Sure validates and does not fill in, so a create without them always failed with a 422.
- A `--file` must be a regular file: devices, pipes and directories are rejected before any request. `merchants import` requires a `.csv` file.

#### Windows and totals

- `--months` windows start one day later: run on 2026-11-20, `--months 1` covers 2026-10-21 to 2026-11-20, so each day of the month counts once. A window that ends on the last day of a month covers whole calendar months: run on 2026-04-30, `--months 1` covers 2026-04-01 to 2026-04-30, where it took in the bill of 2026-03-31 too. This applies to `status` (monthly), `analyze leaks`, `propose rules` and `export transactions`.
- Windows end on the family's today, not the UTC date: `status`, `analyze runway`, `analyze leaks` without `--start-date`, `propose rules` and `export transactions` read the family's time zone from `GET /api/v1/family_settings`, one more request. The zone is resolved as Sure's `Family#resolved_time_zone` does: a Rails name (`Madrid`, `Eastern Time (US & Canada)`) maps to its IANA zone, and no zone or an unknown one is UTC. When the settings cannot be read, the CLI uses the local zone and reports the failure in `meta.warnings` (new: `code`, `message`, `details`). `status` budget pacing uses the same date.
- `transactions create`, `valuations create` and `trades create` without `--date` use the family's today (one request to `family_settings`), also in a dry run; the default was this machine's date when the command started.
- Totals in `analyze` and `propose` approximate Sure's income statement. They skip split parents, excluded rows, transfers (`funds_movement`, `cc_payment`, `one_time`) and accounts excluded from reports or not draft or active. `loan_payment` and `investment_contribution` rows count as expenses. See "Amounts and Totals" in the README for what the API does not let the CLI reproduce.

#### Requests

- POST, PATCH and DELETE are no longer retried. GET, HEAD, PUT and OPTIONS retry on network errors and 5xx, and on 429 when `Retry-After` is at most 5 s.
- `--timeout` (default 30 s) bounds a whole call, retries included; it bounded each attempt. A retry whose wait would outlast it is skipped and Sure's last answer is reported. Uploads are bounded by pauses in sending the file instead, then by the wait for the answer.
- A POST, PATCH or DELETE that times out or loses its connection after reaching Sure is `outcome_unknown` with `details.may_have_applied`; it was `timeout` or `network_error`.
- Redirects are no longer followed: they fail with `redirect_refused`, so credentials never go to the redirect target.
- Paged reads follow the page count each page reports, so rows pushed onto a new page mid-read are no longer dropped. A list that loses rows mid-read, so a row may have been skipped, is `list_changed` (rows added mid-read are not); one past 1000 pages (10,000 for `export transactions`, which now streams rows to the file) is `too_many_pages` (was `invalid_response`). When `X-RateLimit-Remaining` cannot cover the pages left, the command fails early with `rate_limited`.
- `family-exports download` honors Ctrl-C while it looks the export up, and a second Ctrl-C kills it.
- `export transactions` handles Ctrl-C and SIGTERM: it fails with `canceled` and removes its partial file. The JSON export spools its rows in a hidden file next to `--out` (it used `$TMPDIR`), removed as well.
- `family-exports download` gives each attempt the whole `--timeout` to answer and skips a retry that would wait longer, so a 5xx is reported with Sure's message instead of ending as `timeout`.
- `login` stores a random per-install `auth.device.device_id`, so logging in on another machine no longer logs this one out.
- A plain `http://` `api_url` on a host other than this machine fails with `validation_failed` unless `--allow-insecure-http` or the `allow_insecure_http` config key is set (SURE-SEC-002). The check, like the timeout check, runs only for commands that call Sure: `version`, `help`, `completion`, `config` and a bare command group run with an invalid setting.

#### Config and tokens

- When saving the config fails after Sure issued new tokens, `error.details.refresh_token` is gone: the new pair is written to an owner-only file in the user cache directory and `error.details.rescue_file` gives its path. This also covers `refresh` and `login`, which reported only the save error.
- `refresh` and `login` check that the config file is writable before contacting Sure and fail with `config_save_failed` otherwise.
- `--timeout` and the `timeout` config key take a bare number as seconds (`30` was 30 ns for the key and rejected by the flag). A negative or unreadable value fails with `validation_failed`; the key fell back to no timeout at all. `config set timeout` checks the value.
- Saving the config never loosens its mode: 0400 stays 0400, and anything looser than 0600 becomes 0600. It refuses a file owned by another user, and writes through a symlink whose target does not exist yet instead of replacing the link.

### Added

- `account-statements list`, `show` and `upload`.
- `analyze leaks --currency`, `--start-date` and `--end-date`.
- `accounts list --all` (every page, without `meta.status`) and `--include-disabled`.
- `cash-flow show` and `spending-vs-normal show`.
- `insights list` and `insights ack` for Sure's insights feed. Sure answers 204 also for an insight already acknowledged or expired, so `ack --apply` reports `{"id": ..., "in_feed": false}`.
- `merchants import`.
- `recurring-transactions create` and `update`: `--autopay`, `--notes` and `--payment-url`.
- `security-prices create`.
- `trades create`: every Sure trade type, with `--amount`, `--fee` and `--transfer-account-id`.
- `transactions create`: `--category-id`, `--merchant-id`, `--tag-id`, `--external-id`, `--source`, `--user-modified`. `transactions update`: `--merchant-id`, `--tag-id`, `--clear-tags`, `--user-modified`.
- `imports create --rows-to-skip`.
- `version --server`.
- The global `--timeout` flag and `timeout` config key.

### Fixed
- `propose rules` with nothing to propose prints `"proposals": []`; it printed `null`, which its schema rejects. `analyze runway` gives an overdrawn account 0 months of runway, not a negative number, which its schema rejected.
- `export transactions --out /dev/stdout` and `family-exports download --out /dev/stdout` write only the file to stdout; the result envelope, success or failure, goes to stderr. It was appended after the data, so a CSV gained JSON lines and a JSON export became two documents.
- `login` no longer refreshes the session it replaces: with an expired token and a refresh token Sure had revoked, it failed with `login_failed: Invalid refresh token` before sending the password, and kept failing until the config was edited by hand. `refresh` sends one refresh; near expiry it refreshed automatically first and then sent the revoked token, reporting failure after the pair had already been rotated and saved.
- `--notes ""` on `transactions update`, `trades update`, `valuations update` and `recurring-transactions update`, and `--payment-url ""` on `recurring-transactions update`, clear the field. The empty value was dropped, so the command failed with "no fields provided to update", or left the stored value unchanged when another flag was given.
- A `--config` file of any name is read as YAML. One without a `.yaml` extension (`~/.sure-cli`, `--config <(...)`) failed with `Unsupported Config Type`. An unreadable config is one `validation_failed` error, without the usage text.
- The HTTP library no longer prints its own `WARN RESTY` and `ERROR RESTY` lines to stderr on retries and refused redirects.
