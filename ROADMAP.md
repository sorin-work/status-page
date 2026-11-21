# Roadmap

Things I might add to status-page. The 1.0.0 release covers the whole loop
(check, open an incident, notify, show it on the public page, resolve), so
nothing here is promised or scheduled. These are ideas, written down with
enough detail that I (or anyone else) can pick one up later without
starting from zero. Items may change, move between groups or be dropped.

What the app does today is in the [README](README.md) and the notes under
[docs/](docs/).

- **Next**: small or well understood. I would start with these.
- **Later**: worth doing, but bigger or waiting on a decision.
- **Maybe**: not sure yet that they are worth the extra complexity.

Sizes are rough: **S** is an evening or two, **M** a few evenings up to a
couple of weekends, **L** several weeks of spare time.

Each item is meant to fit in one pull request and to be covered by the test
suite. Tests sit next to the code: the domain packages (`internal/monitor`,
`internal/uptime`, `internal/component`, `internal/maintenance`,
`internal/incident`, `internal/notify`) have plain unit tests,
`internal/store` runs against a temporary SQLite file, `internal/web` drives
the real handlers with `httptest`, and `internal/e2e` runs the scheduler,
alerting, webhooks and the status page together against real HTTP servers.
So logic worth testing (uptime math, levels, windows, payloads) belongs in a
domain package, not inside a handler or a template. Schema changes come as
a new numbered file in `internal/store/migrations`. File names below are
relative to `internal/` unless they start with `cmd/`, `docs/` or are a
root file.

"Depends on" lists real prerequisites only. Items that merely touch the same
file are not listed there; the bigger overlaps are mentioned in the items.

When an item ships, its row in the Overview says `Shipped` in the Group
column and its section moves under a `## Shipped` heading at the end,
shortened to one paragraph on what was built and where it differs from the
text here. The same pull request updates the README, the matching page under
`docs/` and adds an entry under `## Unreleased` in the CHANGELOG.

## Overview

| #   | Item                                            | Group | Area          | Size | Depends on |
| --- | ----------------------------------------------- | ----- | ------------- | ---- | ---------- |
| 1   | Open incidents of deleted or paused monitors    | Next  | incidents     | S    | -          |
| 2   | Online backups and a restore guide              | Next  | ops           | S    | -          |
| 3   | Prune the webhook delivery log                  | Next  | webhooks      | S    | -          |
| 4   | TLS certificate expiry checks                   | Next  | monitors      | M    | -          |
| 5   | Heartbeat monitors                              | Next  | monitors      | M    | -          |
| 6   | Public base URL and absolute links              | Later | config        | S    | -          |
| 7   | Reset an admin password from the command line   | Later | ops           | S    | -          |
| 8   | Edit incidents, maintenance and webhooks        | Later | admin         | M    | -          |
| 9   | Choose events per webhook                       | Later | webhooks      | S    | -          |
| 10  | Maintenance events in webhooks                  | Later | webhooks      | S    | -          |
| 11  | Timestamps in webhook signatures                | Later | webhooks      | S    | -          |
| 12  | A webhook queue that survives restarts          | Later | webhooks      | M    | -          |
| 13  | Slack, Discord and Teams formats                | Later | notifications | M    | 6          |
| 14  | Email alerts over SMTP                          | Later | notifications | M    | 6          |
| 15  | Atom feed of incidents                          | Later | status page   | S    | 6          |
| 16  | Status badges                                   | Later | status page   | S    | -          |
| 17  | Degraded performance level                      | Later | monitors      | M    | -          |
| 18  | Planned maintenance left out of uptime          | Later | uptime        | M    | -          |
| 19  | Days in the configured time zone                | Later | uptime        | M    | -          |
| 20  | More HTTP check options                         | Later | monitors      | M    | -          |
| 21  | DNS monitors                                    | Later | monitors      | M    | -          |
| 22  | More than one admin account                     | Later | admin         | M    | -          |
| 23  | Audit log                                       | Later | admin         | M    | -          |
| 24  | API tokens stored in the database               | Later | api           | M    | -          |
| 25  | API for components and webhooks                 | Later | api           | S    | -          |
| 26  | Prometheus metrics                              | Later | ops           | S    | -          |
| 27  | Recurring maintenance                           | Later | maintenance   | M    | -          |
| 28  | Component groups                                | Later | status page   | M    | -          |
| 29  | Branding for the status page                    | Later | status page   | S    | -          |
| 30  | Dark mode                                       | Later | ui            | S    | -          |
| 31  | Accessibility pass                              | Later | ui            | M    | -          |
| 32  | Monthly uptime reports                          | Later | uptime        | M    | -          |
| 33  | Back-dated incidents and postmortems            | Later | incidents     | M    | 8          |
| 34  | Email subscribers on the status page            | Maybe | notifications | L    | 14         |
| 35  | Reads that do not wait for writes               | Maybe | performance   | L    | -          |
| 36  | Checks from more than one location              | Maybe | monitors      | L    | -          |
| 37  | Two-factor login                                | Maybe | admin         | M    | 22         |
| 38  | Outbound address policy                         | Maybe | security      | S    | -          |
| 39  | Ping monitors                                   | Maybe | monitors      | M    | -          |

## Next

### 1. Open incidents of deleted or paused monitors

An automatic incident is resolved only by a down to up transition of its
monitor (`monitorUp` in `alerting/alerting.go`). Two admin actions take
that transition away:

- Deleting a monitor that is down. `DeleteMonitor` in `store/monitors.go`
  removes the row, the foreign key in `004_incidents.sql` sets
  `incidents.monitor_id` to `NULL`, and the incident stays "investigating"
  forever. It keeps raising its component (or the whole page) through
  `component.ForIncident`, and nobody can resolve it from the monitor page
  because there is none.
- Pausing a monitor that is down. `ListActiveMonitors` stops returning it,
  so it is never checked again, and its incident stays open until someone
  resumes it.

Plan:

- In `handleDeleteMonitor` (`web/monitors.go`) and `apiDeleteMonitor`
  (`web/api_monitors.go`), resolve the open automatic incident first with
  an update such as "The monitor was removed." and publish
  `incident.updated`, the same way `monitorUp` does. Both handlers should
  call one helper in `alerting` so the web package does not learn incident
  rules.
- Pausing a monitor that is down shows a checkbox next to the pause button,
  "Resolve its open incident", checked by default; the API takes
  `?resolve=false` to keep it open. Resuming does not reopen anything: the
  next failed checks open a new incident as usual.
- The scheduler keeps a `monitorState` per monitor in memory. A deleted
  monitor's entry is never removed. That is harmless today but should be
  cleaned up in `RunDue` for ids that are no longer listed.
- Tricky: the delete and the resolve should happen in one transaction, or a
  crash in between leaves the same orphan. `store` needs a
  `DeleteMonitorResolving(ctx, id, message, at)` rather than two calls.
- Also a one-off migration step: resolve automatic incidents that are open
  and have `monitor_id IS NULL`, so existing databases are fixed too.
- Tests: `alerting` unit tests for the helper; `web/monitors_test.go` for
  delete and pause with an open incident; a store test that the migration
  resolves orphans.

### 2. Online backups and a restore guide

Everything lives in one SQLite file, which is easy to copy and easy to copy
wrongly. Copying `status.db` while the server runs can miss the WAL file,
and the image (`alpine:3.22`) has no `sqlite3` to make a proper copy. There
is no backup command and the docs do not mention backups at all.

- A `statuspage backup <path>` subcommand in `cmd/statuspage/main.go` that
  opens the database and runs `VACUUM INTO ?`. That gives a consistent,
  compacted copy while the server keeps running, through the same pure Go
  driver.
- Optional scheduled backups: `BACKUP_DIR` and `BACKUP_KEEP` (default 7). The
  `housekeeping` loop already runs every hour; it can write one backup a day
  named by date and remove the oldest beyond the limit.
- `docs/backups.md`: what to back up (the database file only, the secret
  for the API comes from the environment), how to restore (stop, replace
  the file, start; migrations run on start), and a compose example that
  mounts a host folder for `BACKUP_DIR`.
- Tricky: the container is `read_only: true` with only `/data` and `/tmp`
  writable. `BACKUP_DIR` must be on a volume, and the config should refuse
  a path that is not writable at start instead of failing at 3 a.m.
- Tricky: a restore of an older backup onto a newer binary is fine (the
  migrations run), the other way round is not. `Migrate` should refuse a
  database that has migrations it does not know, with a clear message.
- Undecided: `VACUUM INTO` holds a read transaction for the length of the
  copy. On a large check history that is seconds, during which the WAL
  grows. Acceptable once a day.
- Tests: a store test that backs up a database with checks and opens the
  copy; a config test for the new variables; a migrate test for the unknown
  migration check.

### 3. Prune the webhook delivery log

`housekeeping` in `cmd/statuspage/main.go` prunes raw checks and expired
sessions. Nothing ever deletes from `webhook_deliveries`. Every event
writes one row per enabled webhook and per attempt, so a flapping monitor
with three webhooks writes rows forever, and the deliveries page only ever
shows the last few (`ListDeliveries` with a limit).

- A `PruneDeliveries(ctx, before)` in `store/webhooks.go`, called from
  `housekeeping` with the same `RETENTION_DAYS` as the checks. A separate
  setting is not worth it.
- Keep at least the last 50 deliveries per webhook regardless of age, so a
  quiet webhook still shows its last test.
- The index `webhook_deliveries_webhook` covers `(webhook_id, created_at)`;
  a plain `created_at` delete needs its own index or it scans the table.
  Add it in the same migration.
- Tests: a store test with old and new rows and the keep-last rule.

### 4. TLS certificate expiry checks

The most common outage that a status page could have seen coming is an
expired certificate. HTTP monitors already connect over TLS and then throw
the certificate away.

- `checkHTTP` in `monitor/checker.go` reads `resp.TLS.PeerCertificates[0]`
  and stores its `NotAfter` in a new `Result.CertExpiresAt`. The `checks`
  table gets a nullable `cert_expires_at`, and `monitors` keeps the last
  value so the dashboard can show it without a query over checks.
- A new monitor field `CertWarnDays` (default 14, 0 turns it off). Under the
  limit the check still passes, but the monitor shows a warning on the
  dashboard and the monitor page, and a new `monitor.cert_expiring` event
  goes out once per day at most.
- An already expired or invalid certificate fails the check today with the
  `x509` error. That stays as it is.
- Tricky: when to send the warning. The scheduler only knows about up and
  down transitions. The warning needs its own state ("last warned on"), on
  the monitor row so a restart does not warn again.
- Tricky: `NoRedirects` and redirects to another host. The certificate
  checked should be the one of the first response's host, which is what
  `resp.TLS` holds only without redirects. With redirects followed it is
  the last host's. Document it; I lean towards checking the final host,
  since that is what users see.
- Not a separate monitor type: most people want this on the HTTP monitor
  they already have. A TCP monitor with TLS (SMTP on 465, say) can come
  later.
- Tests: `checker_test.go` with `httptest.NewTLSServer` and a short-lived
  certificate; scheduler or alerting tests for the once-a-day rule.

### 5. Heartbeat monitors

Everything today is pulled: status-page connects to the target. Cron jobs,
backups and queue workers have nothing to connect to. The usual answer is a
heartbeat: the job calls a secret URL when it finishes, and the monitor goes
down when the call does not arrive in time.

- A third `Type`, `heartbeat`, in `monitor/monitor.go`. `Target` is empty;
  the monitor gets a random token (`auth.NewToken`) and a grace period
  next to its interval.
- A public route `POST /heartbeat/{token}` (and `GET`, because many cron
  lines only have `curl`) in `web/server.go` that records a successful
  check. Unknown tokens answer 404 without saying more.
- The scheduler checks heartbeat monitors like the others, but the
  "check" is a store lookup: was there a ping since `interval + grace`
  ago? That keeps retries, transitions, incidents and webhooks working
  without changes.
- `Validate` in `monitor/validate.go` gets the new rules: no target, no
  expected status or keyword, grace at least the minimum interval.
- Tricky: the ping must not sit behind `admin()` or the CSRF check, and it
  should be rate limited per token so a broken loop cannot fill `checks`.
  One check row per ping, at most one per ten seconds per monitor.
- Tricky: the first ping. A new heartbeat monitor is `unknown` until its
  first ping, not down after the first interval.
- Undecided: a `/fail` variant so a job can report its own failure. Cheap
  to add and useful; I lean towards yes.
- Tests: validate tests; a web test for the ping route; an e2e test where a
  heartbeat stops and an incident opens.

## Later

### 6. Public base URL and absolute links

The server never knows its own public address. The `url` field of an
incident in webhook payloads (`IncidentInfo.URL` in `notify/notify.go`) is
empty for automatic incidents and a relative `/incidents/5` for manual ones
(`web/webhooks.go`). A receiver cannot turn that into a link. Feeds, emails
and chat messages (items 13 to 15) all need absolute links.

- `PUBLIC_URL` in `config/config.go`, validated as an absolute `http(s)`
  URL without a query. Empty keeps today's behaviour.
- A small `links` helper (`incident(id)`, `status()`) used by alerting, the
  webhook test and later the feeds, so the format lives in one place.
- The status page gets `<link rel="canonical">` and Open Graph tags from it.
- Tests: config tests; alerting tests that the URL is absolute when set.

### 7. Reset an admin password from the command line

`docs/troubleshooting.md` tells people who lost the password to run
`sqlite3 data/status.db "DELETE FROM users..."`. The Docker image has no
`sqlite3`, and deleting every user is a blunt tool.

- `statuspage reset-password <username>` reads a new password from stdin
  (or `--generate` prints one), hashes it with `auth.HashPassword` and
  deletes that user's sessions, through the store.
- Works with `docker compose exec app statuspage reset-password admin`.
- Update the troubleshooting page.
- Tests: the subcommand's function against a temporary store.

### 8. Edit incidents, maintenance and webhooks

Several things can be created but not changed:

- Incidents: title, impact and component are fixed once created. Only
  timeline updates can be added (`POST /admin/incidents/{id}/updates`). A
  typo in a public title stays public.
- Incident updates: a wrong message cannot be corrected.
- Maintenance: only "end now" and delete. Moving a window by an hour means
  deleting it and typing it again.
- Webhooks: only toggle and delete. Changing the URL loses the delivery
  log, and there is no way to rotate the secret.

Plan:

- Edit forms and `POST` handlers for each, reusing the create forms and the
  validation that exists (`incident.ValidateNew`, `Window.Validate`).
- Editing an update keeps its time and marks it edited on the public page.
- "Rotate secret" on a webhook shows the new secret once.
- Tricky: editing a maintenance window that is in progress. Moving its end
  earlier than now should end it; moving its start into the future while
  monitors are failing should not suddenly open incidents for failures that
  were covered. The alerting only looks at windows when a transition
  happens, so nothing reopens by itself; document that.
- Tricky: an automatic incident's title says "X is down". Editing it is
  fine, but the reopen logic must keep finding it by `monitor_id`, not by
  title. It does today; keep a test for it.
- Tests: handler tests for each form, including validation errors.

### 9. Choose events per webhook

Every webhook receives every event. A chat channel usually wants incident
events and not every single `monitor.down` of an internal monitor.

- An `events` column on `webhooks` (comma separated, empty means all) and
  checkboxes on the webhook form.
- Optionally a component filter: only events for monitors and incidents of
  the chosen components.
- `Dispatcher.Publish` in `notify/dispatcher.go` filters before queueing.
- Tests: dispatcher unit tests with filtered hooks; a store test for the
  new column.

### 10. Maintenance events in webhooks

Maintenance windows are silent. Subscribers of a webhook learn about an
outage, but not that a planned window started or ended, which is exactly
when people ask "is it you or us?".

- New events `maintenance.scheduled`, `maintenance.started` and
  `maintenance.ended`, with a `maintenance` object in the payload.
- `scheduled` comes from the create handlers. `started` and `ended` need a
  clock: a check in the `housekeeping` loop or, better, in the scheduler
  tick, which runs every second. It needs a "notified" flag per window so a
  restart does not send twice.
- Ending a window early ("End now") sends `ended` right away.
- Document the payload in `docs/webhooks.md`.
- Tests: notify payload tests; an e2e test where a window starts and ends.

### 11. Timestamps in webhook signatures

`X-StatusPage-Signature` is an HMAC over the body only. A captured request
can be replayed to the receiver at any time and still verifies. The body
has a `timestamp`, but receivers have to know to check it.

- Sign `timestamp + "." + body`, send the timestamp in a new
  `X-StatusPage-Timestamp` header, and keep the old header for one release
  so receivers can switch. This is the scheme Stripe and others use, so
  receivers have libraries for it.
- `notify.Verify` gets a tolerance argument; the demo target in
  `cmd/demotarget/webhooks.go` checks it.
- Update the verification examples in `docs/webhooks.md`.
- Tests: sign and verify with a fixed clock, inside and outside the
  tolerance.

### 12. A webhook queue that survives restarts

Deliveries wait in a channel of 256 jobs (`QueueSize`). Anything queued or
waiting for a retry is lost on restart, and when the queue is full new
deliveries are dropped with a log line. On shutdown `Dispatcher.Close`
waits for every queued delivery, including the `time.Sleep` of up to two
minutes between retries, while `SHUTDOWN_TIMEOUT` defaults to 10 seconds
and Docker kills the container soon after. So the deliveries most likely to
be lost are the retries of an outage, which are the ones that matter.

- Store pending deliveries in a `webhook_outbox` table (hook, event body,
  attempt, next attempt time) in the same transaction that records the
  incident, where possible.
- Workers poll the table for due rows instead of reading a channel. A retry
  updates `next_attempt_at` instead of sleeping.
- On shutdown, workers finish the attempt in flight and leave the rest in
  the table.
- Tricky: incident creation and the event happen in different packages.
  `alerting` would need to write the outbox row through the store in the
  same transaction as `CreateIncident`, which changes the `Publisher`
  interface. Writing the row right after the incident is simpler and loses
  only a crash between two statements; I lean towards that.
- Tricky: the delivery id has to be stored so retries keep the same
  `X-StatusPage-Delivery`.
- Tests: dispatcher tests with a fake clock; a test that restarts the
  dispatcher with a pending row and sees it delivered.

### 13. Slack, Discord and Teams formats

The only channel is a JSON webhook, so sending an alert to a chat room
needs a small relay in between. Slack, Discord and Teams all accept
incoming webhooks with their own JSON shape.

- A `format` on each webhook: `json` (today), `slack`, `discord`, `teams`.
- One function per format in `notify` that turns an `Event` into the body,
  with a short text and a link (from item 6).
- These receivers do not check signatures; the secret stays but is unused
  for them.
- Tricky: rate limits. Discord answers 429 with a retry time. The retry
  logic treats 429 as retryable already; it should honour `Retry-After`.
- Tests: golden tests for each format and event.

### 14. Email alerts over SMTP

Email is the alert channel everyone has. It needs only an SMTP server,
which self-hosters usually have.

- `SMTP_HOST`, `SMTP_PORT`, `SMTP_USERNAME`, `SMTP_PASSWORD`, `SMTP_FROM`
  and `SMTP_TLS` in the config. Without `SMTP_HOST` email is off.
- Email recipients as a new kind of notification target next to webhooks,
  with the same event choice (item 9) and the same delivery log.
- Plain text emails with a link to the incident (item 6).
- A mailpit service in `docker-compose.yml` for the demo, on a free port.
- Tricky: one email per transition is too many during a flap. Group
  `monitor.down` and `monitor.up` of the same monitor within a minute.
- Tests: a fake SMTP server in the test (`net/smtp` against a listener) or
  an interface with a recording sender.

### 15. Atom feed of incidents

Many people follow status pages through a feed reader or a chat bot that
reads feeds. The page has no feed.

- `GET /history.atom` with the last 50 incidents, each entry updated when
  a timeline update is added, linked from the status page `<head>`.
- Uses the same data as `handleHistory` and needs absolute links (item 6).
- Tests: a web test that parses the feed with `encoding/xml`.

### 16. Status badges

A README or an internal wiki often wants a small "API: operational" badge.

- `GET /badge/{component}.svg` (and `/badge.svg` for the whole page) with
  the level label and colour, and an `uptime` variant with the 90 day
  figure.
- Components get a slug for stable URLs; ids change when the database is
  rebuilt.
- Served from the five second status cache, with `Cache-Control: max-age=60`
  so image proxies such as GitHub's camo do not hit the server on every view.
- Tests: SVG output for each level; a web test for unknown slugs.

### 17. Degraded performance level

A component is operational, partially down, majorly down or under
maintenance (`component.Level`). A site that answers in eight seconds is
"operational". Most status pages have "degraded performance" for that, and
the response time data is already stored.

- A `LatencyThreshold` on monitors. A passing check slower than the
  threshold counts as degraded, not down.
- A new `Degraded` level in `component/component.go`, between operational
  and partial outage in `severity`, with its label and colour.
- `LevelFor` needs the latest check per monitor, which the status page
  already loads (`LatestChecks`).
- Tricky: one slow check should not flip the level. Use the same retries
  logic: degraded after `Retries + 1` slow checks in a row. That state
  lives in the scheduler, so it needs its own counter in `monitorState`.
- Undecided: open an incident for degraded monitors. I lean towards no, a
  level on the page and an event only.
- Tests: component unit tests for the new level; scheduler tests for the
  counter.

### 18. Planned maintenance left out of uptime

During a maintenance window failing checks open no incident, but they still
count against uptime. A planned hour of downtime takes a 24 hour figure to
95.8% and turns the day's bar amber, which contradicts the "planned" label
next to it.

- Checks during an active window covering the monitor's component get a
  `maintenance` flag. `RecordChecks` in `store/checks.go` writes it, and
  `daily_stats` counts those checks separately.
- `uptime.Ratio` ignores them; the bar tooltip says how long the day was
  under maintenance.
- Tricky: the scheduler does not know about maintenance today; only
  alerting does. Looking up windows on every check would be a query per
  check. Keep the current windows in memory and refresh them every tick.
- Tricky: existing history. Old rows stay as they are; document it.
- Undecided: make this a setting. I lean towards always on, since that is
  what "planned" means.
- Tests: uptime unit tests; a store test for the daily counts.

### 19. Days in the configured time zone

`TIMEZONE` changes how dates are shown, but `daily_stats` is keyed by UTC
day and the 90 bars are UTC days (`uptime.Bars`). For a status page in
`America/Los_Angeles` an outage at 18:00 local time lands on "tomorrow's"
bar, and the tooltip date does not match the incident date shown a few
lines below.

- Compute the day key with the configured location in `RecordChecks`.
- A migration cannot rebuild old days from raw checks that are gone, so
  keep the UTC days as they are and switch from the first day after the
  upgrade. Store the zone used in a settings row, so a later change of
  `TIMEZONE` can warn that history before the change uses the old days.
- Tricky: days with 23 or 25 hours around daylight saving time. Uptime is
  a ratio of checks, so it is fine; the "no data" logic must not mark the
  short day as partly missing.
- Tests: store tests with a fixed location and checks around midnight and
  around a daylight saving change.

### 20. More HTTP check options

HTTP checks are always `GET` without headers. Many health endpoints need a
header (an API key, a `Host` for a virtual host), basic auth, or a `HEAD`
to avoid downloading a page.

- Method (`GET`, `HEAD`, `POST`), request headers (a few name/value pairs),
  an optional body for `POST`, and basic auth, on the monitor form and the
  API.
- An "accept invalid certificates" switch for internal hosts with
  self-signed certificates, shown with a warning.
- Status code ranges ("200-299") or a list, instead of one expected code.
- Tricky: secrets. Headers and passwords are stored in plain text in the
  database and returned by `GET /api/monitors`. Mask them in API answers
  and in the edit form (write-only fields), like webhook secrets.
- Tricky: `Validate` grows; keep the header rules (no `Content-Length`,
  size limits) in `monitor/validate.go` with their own tests.
- Tests: checker tests against `httptest` for each option; validate tests.

### 21. DNS monitors

DNS failures take everything down at once, and a TCP or HTTP check only
reports them as `no such host`.

- A `dns` monitor type: a name, a record type (A, AAAA, CNAME, MX, TXT) and
  optionally an expected value and a resolver address.
- Uses `net.Resolver` with a custom `Dial` for a chosen resolver; no new
  dependency.
- Tricky: answers in a different order or with several values. Compare
  sets, and match "contains" by default.
- Tests: a tiny DNS server in the test is a lot of work; put the lookup
  behind an interface and test the comparison logic as a unit.

### 22. More than one admin account

The `users` table can hold many accounts, but there is no way to add one:
`ensureAdmin` creates the first account and that is all. A team shares one
password, and changing it logs everybody out.

- A users page in the admin area: list, invite by creating an account with
  a one-time password, remove (never the last one, never yourself).
- Optional roles: `admin` (everything) and `editor` (incidents and
  maintenance only). Writing incident updates during an outage is the job
  most people need; managing monitors and webhooks is not.
- `admin()` in `web/session.go` takes the role required.
- Tests: handler tests for each role and the last-admin rule.

### 23. Audit log

There is no record of who paused a monitor, deleted a webhook or posted an
incident update. With one shared account that does not matter much; with
item 22 it does.

- An `audit_log` table (time, user, action, object, short detail) written
  by the admin and API handlers, and a page that lists it with a filter.
- API calls are recorded as "API token" (or the token's name after
  item 24).
- Pruned like the checks.
- Tests: store tests; handler tests that a few actions are logged.

### 24. API tokens stored in the database

The API has one token from `API_TOKEN`. Rotating it means a restart and
updating every script at once, and a CI job that only pauses monitors has
the same power as one that deletes them.

- An `api_tokens` table (name, hash, scopes, created, last used), managed on
  the account page. The token is shown once and only its SHA-256 stored,
  like sessions.
- Scopes: `read`, `incidents`, `monitors`, `all`.
- `apiAuth` in `web/api.go` checks the table; `API_TOKEN` keeps working as
  an "all" token so existing setups do not break.
- Tests: handler tests for each scope; a store test for the hashing.

### 25. API for components and webhooks

The API can only list components (`GET /api/components`) and does not know
webhooks. Setting up an instance from a script still needs the admin UI.

- `POST`, `PUT` and `DELETE` for components (with order), and the same for
  webhooks, with the secret shown once on create.
- Document them in `docs/api.md`.
- Tests: `web/api_*_test.go` like the existing ones.

### 26. Prometheus metrics

People who run status-page often run Prometheus too, and want the same
numbers there.

- `GET /metrics` in the text format, hand written (a few gauges and
  counters, no client library needed): monitor up, last latency, check
  totals, open incidents, webhook failures, queue length.
- Protected by an optional `METRICS_TOKEN`, off by default, because the
  monitor names may be internal.
- Tests: a web test that parses a few lines.

### 27. Recurring maintenance

Windows are one-off. A weekly database maintenance on Sunday night has to
be entered every week.

- A recurrence on the window: none, daily, weekly on chosen days, monthly
  on a day, with an end date. Stored as a rule, expanded on read.
- `maintenance.Window` gets a function that returns the occurrences in a
  range; `ListCurrentMaintenance` and the status page use it.
- Tricky: time zones. A window "every Sunday 02:00" must stay at 02:00
  local time across daylight saving changes, so the rule is stored in the
  configured location, not as UTC offsets.
- Tricky: ending or deleting one occurrence without the series. Keep an
  exceptions list.
- Tests: occurrence expansion unit tests, including daylight saving.

### 28. Component groups

All components are one flat list. With twenty of them the page is long and
"Storefront EU" and "Storefront US" are far apart.

- A `component_groups` table and a group on each component. The page shows
  a group as one row with the worst level of its members, expandable.
- Order of groups and of components inside them on the components page.
- Tests: component level tests for groups; a status page test.

### 29. Branding for the status page

The page shows `SITE_TITLE` and the default colours. Most operators want
their logo, a link back to their site and maybe their colour.

- Settings in the admin area (not environment variables, so they can change
  without a restart): logo upload (stored in the database, size limited),
  accent colour, footer links, a short "about" text.
- Tricky: uploads. Accept PNG and SVG only; SVG must be served with
  `Content-Security-Policy: script-src 'none'` or sanitised, because it can
  carry scripts.
- Tests: handler tests for the settings; an upload test with an SVG that
  contains a script.

### 30. Dark mode

`web/static/style.css` has one light theme and no `prefers-color-scheme`
rule. A status page opened from a dark chat client at night is bright
white.

- Colours as custom properties, a dark set under
  `@media (prefers-color-scheme: dark)`, for the status page and the admin
  area.
- The bar colours (green, amber, red, grey) must keep enough contrast on a
  dark background.
- No test level covers CSS; check by hand and note it in the PR.

### 31. Accessibility pass

A few concrete problems:

- The 90 day bars sit in a `div` with `role="img"`, and each bar inside is
  focusable with its own `aria-label`. Children of `role="img"` are
  presentational, so screen readers may announce only "Uptime over the last
  90 days" and then 90 unlabelled tab stops. Either a list of days or one
  image with a text summary ("3 days with downtime, worst on Oct 12") and
  the details on the incident list.
- The tooltips appear on hover and focus only; on touch they need a tap.
- Levels are told apart by colour. Add an icon or a pattern.
- Admin forms: check that every input has a label and that errors are
  linked with `aria-describedby`.
- Tests: render tests in `web/render_test.go` that check the markup for
  the points above.

### 32. Monthly uptime reports

People who promise an SLA need a number per month: uptime, incidents,
total downtime, per component.

- A report page in the admin area with a month picker, and the same as CSV
  and in the API.
- Uses `daily_stats`, so it works for any month, not only within
  `RETENTION_DAYS`.
- Builds on item 18 if that has shipped: maintenance excluded.
- Tests: a unit test for the monthly aggregation in `uptime`.

### 33. Back-dated incidents and postmortems

An incident can only start "now". An outage noticed afterwards, or one that
happened before status-page was set up, cannot be recorded with the right
times. And there is no place for the write-up afterwards.

- A start time (and optionally a resolve time) on the new incident form,
  not in the future.
- A postmortem field on resolved incidents, shown on the public incident
  page under the timeline.
- Uses the edit forms from item 8.
- Tricky: a back-dated incident should not send `incident.created` as if it
  were happening now. Send it with the real start time, or not at all; I
  lean towards not at all for incidents that are created resolved.
- Tests: incident validation for the new times; handler tests.

## Maybe

### 34. Email subscribers on the status page

Hosted status pages let visitors subscribe to updates by email. It is the
feature people ask for most, and the one with the most new
responsibilities: storing visitors' addresses, double opt-in, unsubscribe
links, bounces, and not becoming a spam relay.

- A subscribe form on the status page, a confirmation email with a signed
  link, an unsubscribe link in every email, optionally per component.
- Sends on incident create, update and resolve, and on maintenance events.
- Rate limits per address and per client on the form.
- Builds on the SMTP settings of item 14. Without them the form is hidden.
- I am not sure it is worth it for a self-hosted tool; the Atom feed and
  chat webhooks cover most of the need.

### 35. Reads that do not wait for writes

`store.Open` limits the pool to one connection. Writes queue instead of
failing, which is right, but reads queue behind them too: during an outage
with many monitors the status page waits for check inserts, and a slow
`VACUUM INTO` (item 2) blocks everything.

- Two pools: one connection for writes, several read-only connections for
  reads. WAL mode allows readers during a write.
- Batch the check inserts: `RecordChecks` already takes a slice; the
  scheduler could flush every second instead of one transaction per check.
- A benchmark with 1,000 monitors at a 10 second interval to see whether
  any of this matters before building it.
- Tricky: every store method has to choose a pool; reads inside a write
  transaction must use the write connection.

### 36. Checks from more than one location

A check from one place cannot tell "the site is down" from "my network is
down". Hosted tools check from several regions and need two of them to
agree.

- A small agent binary (`cmd/statusagent`) that pulls its monitor list from
  the server with a token and posts results back.
- A monitor is down when a chosen number of locations agree.
- Large: a protocol, agent tokens, clock skew, and the scheduler's state per
  location. Only worth it if people run it in more than one place.

### 37. Two-factor login

TOTP for admin accounts: a QR code on the account page, codes at login,
recovery codes. Needs nothing hosted. Worth it once there are several
accounts (item 22); with one shared login it adds little.

### 38. Outbound address policy

Monitors and webhooks can point at any address, including
`169.254.169.254` or other services on the internal network. Only admins
and API token holders can set them, and monitoring internal services is
the point of the tool, so this is not a hole as such. But a leaked API
token can probe the network through the check history.

- An optional `ALLOW_PRIVATE_TARGETS=false` that refuses private, loopback
  and link-local addresses for webhooks (and optionally monitors), checked
  at dial time so DNS rebinding does not get around it.
- Off by default, since the demo itself checks a private address.

### 39. Ping monitors

ICMP ping is a common request for routers and hosts without a TCP port.
It needs raw sockets or the unprivileged `SOCK_DGRAM` ICMP that Linux
allows only when `net.ipv4.ping_group_range` includes the process's group.
The container runs as a non-root user with `no-new-privileges`, so it
would need a sysctl in the compose file. A TCP check of an open port covers
most of the cases; I would only add this if someone asks.
