# Architecture

status-page is one Go binary with an embedded SQLite database. The
templates, stylesheet and migrations are embedded too, so a deploy is the
binary plus a data directory.

```
             ┌──────────────┐  every second   ┌──────────────┐
             │  scheduler   │ ──────────────▶ │   checker    │ HTTP / TCP
             └──────┬───────┘   due monitors  └──────────────┘
      results       │ transitions (up ⇄ down)
   ┌────────────────┤
   ▼                ▼
┌───────┐     ┌──────────┐  events  ┌──────────┐  POST  ┌──────────┐
│ store │ ◀── │ alerting │ ───────▶ │  notify  │ ─────▶ │ webhooks │
└───┬───┘     └──────────┘          └──────────┘        └──────────┘
    │
    ▼
┌──────────────────────────────────────────┐
│ web: status page · admin · JSON API      │
└──────────────────────────────────────────┘
```

## Checks and the scheduler

`internal/scheduler` wakes up every second, loads the active monitors and
claims every monitor whose interval has passed since its last check
started. A claimed monitor is marked as running until its check returns,
so a check slower than the tick is never started twice. Checks run in
goroutines limited by `CHECK_WORKERS`.

`internal/monitor` does the actual work. HTTP checks send a `GET` with a
per-monitor timeout, read at most 1 MiB of the body, compare the status
code (any 2xx/3xx when no code is set) and look for the keyword. TCP
checks only open and close a connection.

Every result is stored. The scheduler keeps, per monitor, the number of
consecutive failures: a monitor goes down when the failures exceed its
retries, and up again on the first success. Only changes of state are
passed on, as a `Transition`.

When the server stops, checks that already started may finish; their
timeout still applies. Cancelling them would record failures that never
happened.

## Incidents and alerting

`internal/alerting` turns transitions into incidents:

- **up → down** opens an automatic incident for the monitor, unless a
  maintenance window covers the monitor's component. If the monitor's
  last incident was resolved less than ten minutes ago it is reopened
  instead.
- **down → up** resolves the open automatic incident with the downtime.

Each step also publishes events (`monitor.down`, `incident.created`, ...)
to `internal/notify`, which queues them for every enabled webhook. A
bounded queue keeps a dead receiver from slowing down the scheduler;
deliveries are retried in the background.

Manual incidents and their updates come from the admin area or the API and
publish the same incident events.

## Storage

`internal/store` wraps `database/sql` with the pure Go
`modernc.org/sqlite` driver, in WAL mode with `synchronous=NORMAL`. The
pool has a single connection: SQLite allows one writer anyway, and this way
concurrent workers queue instead of failing with `SQLITE_BUSY`.

Migrations are numbered SQL files in `internal/store/migrations`, applied
in order at startup and recorded in `schema_migrations`.

| Table               | Holds                                                     |
| ------------------- | --------------------------------------------------------- |
| `monitors`          | Monitor settings and last known status                    |
| `checks`            | Raw results, pruned after `RETENTION_DAYS`                |
| `daily_stats`       | Per monitor and UTC day: checks, successes, latency sum   |
| `components`        | Status page entries and their order                       |
| `incidents`         | Incidents, automatic or manual                            |
| `incident_updates`  | Timeline entries                                          |
| `maintenance`       | Planned windows                                           |
| `webhooks`          | Endpoints and signing secrets                             |
| `webhook_deliveries`| Delivery attempts                                         |
| `users`, `sessions` | Admin accounts and hashed session tokens                  |

Each check is written together with an upsert of its day in `daily_stats`,
in one transaction. That keeps 90 day uptime and the status page bars cheap
to compute and independent of how long raw checks are kept.

## Uptime

`internal/uptime` works with ratios of successful to total checks. 24 hour
and 7 day figures use raw checks, 90 day figures and the bars use the daily
rollups. Percentages are truncated, never rounded up: a day with one failed
check out of 1440 shows 99.93%, not 100.00%.

A day is green from 99.9%, amber from 95% and red below. Days without
checks are grey.

## Status levels

`internal/component` derives the level of a component from its monitors:
operational when all are up, partial outage when some are down, major
outage when all are. Open incidents raise the level of their component (or
of the whole page) by impact. An active maintenance window shows the
component as under maintenance instead of failing. The page banner shows
the worst level. Paused and never checked monitors are ignored.

## Web

`internal/web` uses only `net/http` and `html/template`. Every page is its
own template set cloned from `layout.html`, rendered into a buffer first so
template errors become a clean 500.

Middleware, from the outside in: request ID, request log, security
headers, session lookup. Admin handlers are wrapped with `admin()`, which
requires a session and checks the CSRF token on anything but `GET`. API
handlers are wrapped with `apiAuth()`, which checks the bearer token.

The public status data is cached for five seconds and dropped on any
write request, so a busy status page during an outage does not compete
with the scheduler for the database.

## Security notes

- Passwords: PBKDF2-HMAC-SHA256, 600,000 iterations, random salt.
- Sessions: random 256 bit tokens; only their SHA-256 is stored. Logout
  deletes the session.
- CSRF: per session token in every admin form.
- Login: rate limited per client address; redirects only to local paths.
- Output: everything goes through `html/template`; incident messages are
  escaped before line breaks are added.
- Webhooks: HMAC-SHA256 signature over the raw body.
