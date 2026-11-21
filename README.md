# status-page

Self-hosted uptime monitoring and a public status page, written in Go.

status-page checks HTTP endpoints and TCP ports on a schedule, opens an
incident when something goes down and resolves it when it comes back. A
public page shows the state of each component, 90 days of uptime bars,
open incidents and planned maintenance. Everything runs as one binary with
an embedded SQLite database.

## Features

- **HTTP and TCP monitors** with their own interval, timeout, expected
  status code, keyword match, redirect handling and retries before a
  monitor is marked down.
- **Concurrent scheduler** with a worker limit; slow checks are never
  started twice and running checks finish cleanly on shutdown.
- **Automatic incidents** opened when a monitor goes down and resolved on
  recovery, with the total downtime. A monitor that flaps within ten
  minutes reopens its last incident instead of creating a new one.
- **Manual incidents** with impact levels and a timeline of updates.
- **Scheduled maintenance** per component or for everything: the status
  page shows it, and failing monitors open no incidents meanwhile.
- **Uptime** over 24 hours, 7 days and 90 days, and **response time
  charts** for the last day or week.
- **Public status page** with components, an overall banner, 90-day bars
  with tooltips, recent incidents and a full incident history.
- **Webhooks** signed with HMAC-SHA256, retried with backoff, with a
  delivery log and a test button.
- **JSON API** for monitors, checks, uptime, incidents and maintenance,
  plus a public status endpoint.
- **Admin area** behind a login, with CSRF protection, login rate limiting
  and CSV export of the check history.
- Demo data and a small demo target service so there is something to look
  at right away.

## Quick start

```sh
docker compose up --build
```

| URL                                | What                                  |
| ---------------------------------- | ------------------------------------- |
| http://localhost:4417              | Public status page                    |
| http://localhost:4417/admin        | Admin area                            |
| http://localhost:4417/api/status   | Public JSON status                    |

**Demo login:** `admin` / `demo-password`. The JSON API token is
`demo-api-token-change-me`.

On the first start the database is filled with five components, seven
monitors pointing at the bundled `demo-target` service, 90 days of
generated history, a few past incidents, an upcoming maintenance window and
a webhook that delivers to the demo target.

Change the port with `WEB_PORT=8080 docker compose up --build`.

### Simulating an outage

The demo target can switch its endpoints off:

```sh
# Take the API down; it is marked down after its retry, about a minute later.
docker compose exec demo-target wget -qO- --post-data= 'localhost:8081/control/outage?service=api&state=on'

# Bring it back.
docker compose exec demo-target wget -qO- --post-data= 'localhost:8081/control/outage?service=api&state=off'

# Webhook deliveries the demo target received.
docker compose exec demo-target wget -qO- localhost:8081/webhooks
```

Services that can be switched off: `storefront`, `api`, `checkout`,
`search`.

## Local development

Requires Go 1.26. No C compiler is needed: SQLite is
[modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite).

```sh
make demo     # demo target + server with demo data on :4417
make run      # server only, data in data/status.db
make test     # go test ./...
make race     # tests with the race detector
make lint     # golangci-lint
make build    # binaries in bin/
```

Without `ADMIN_PASSWORD` the first start creates the `admin` account with a
random password and prints it in the log.

## Configuration

All settings are environment variables. See
[docs/configuration.md](docs/configuration.md) for details.

| Variable           | Default                 | Description                                              |
| ------------------ | ----------------------- | -------------------------------------------------------- |
| `LISTEN_ADDR`      | `:4417`                 | HTTP listen address                                      |
| `DATABASE_PATH`    | `data/status.db`        | SQLite database file                                     |
| `SITE_TITLE`       | `Status`                | Name shown in the header and page titles                 |
| `TIMEZONE`         | `UTC`                   | Time zone for dates and the maintenance form             |
| `ADMIN_USERNAME`   | `admin`                 | Username of the first admin account                      |
| `ADMIN_PASSWORD`   | generated               | Password of the first admin account (8+ characters)      |
| `API_TOKEN`        | empty (API disabled)    | Bearer token for the JSON API (16+ characters)           |
| `SESSION_TTL`      | `168h`                  | How long an admin login lasts                            |
| `COOKIE_SECURE`    | `false`                 | Send the session cookie over HTTPS only                  |
| `TRUST_PROXY`      | `false`                 | Take client addresses from `X-Forwarded-For`             |
| `CHECK_WORKERS`    | `8`                     | Checks that may run at the same time                     |
| `RETENTION_DAYS`   | `30`                    | Days of raw checks to keep (daily rollups are kept)      |
| `SEED_DEMO`        | `false`                 | Fill an empty database with demo data                    |
| `DEMO_TARGET_URL`  | `http://localhost:8081` | Where the demo monitors point                            |
| `LOG_LEVEL`        | `info`                  | `debug`, `info`, `warn` or `error`                       |
| `LOG_FORMAT`       | `text`                  | `text` or `json`                                         |
| `SHUTDOWN_TIMEOUT` | `10s`                   | Time allowed for a graceful shutdown                     |

## Tests

```sh
go test ./...
```

Unit tests cover the domain packages (validation, uptime math, bars,
component levels, maintenance windows, signatures), the store runs against
a temporary SQLite file, the web and API tests drive the real handlers
with `httptest`, and `internal/e2e` runs the scheduler, alerting, webhooks
and the status page together against real HTTP servers.

## Project layout

```
cmd/statuspage     the server: config, wiring, housekeeping
cmd/demotarget     fake shop for the demo monitors, receives webhooks
internal/config    environment variables
internal/monitor   monitors, validation, HTTP and TCP checks
internal/scheduler runs due checks concurrently, retries, transitions
internal/alerting  opens, resolves and reopens incidents
internal/incident  incident model and validation
internal/maintenance maintenance windows
internal/component component and page levels
internal/uptime    uptime ratios, summaries, 90-day bars
internal/notify    webhook payloads, signatures, delivery with retries
internal/store     SQLite storage and migrations
internal/auth      password hashing and tokens
internal/seed      demo data
internal/web       admin UI, status page, JSON API
internal/e2e       end-to-end tests
docs/              API, webhooks, configuration, architecture
```

## Documentation

- [JSON API](docs/api.md)
- [Webhooks](docs/webhooks.md)
- [Configuration](docs/configuration.md)
- [Architecture](docs/architecture.md)
- [Troubleshooting](docs/troubleshooting.md)
- [Roadmap](ROADMAP.md): ideas for later, nothing promised

## License

[MIT](LICENSE)
