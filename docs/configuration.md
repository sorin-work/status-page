# Configuration

status-page is configured with environment variables only. Invalid values
stop the server at startup with a message naming the variable.

## Server

| Variable           | Default          | Notes                                                        |
| ------------------ | ---------------- | ------------------------------------------------------------ |
| `LISTEN_ADDR`      | `:4417`          | Address and port, e.g. `127.0.0.1:4417`                      |
| `SHUTDOWN_TIMEOUT` | `10s`            | Go duration. Running checks are allowed to finish first      |
| `LOG_LEVEL`        | `info`           | `debug` also logs health checks and static files             |
| `LOG_FORMAT`       | `text`           | `json` for log collectors                                    |
| `TRUST_PROXY`      | `false`          | Use the last `X-Forwarded-For` entry as the client address   |

Only enable `TRUST_PROXY` behind a reverse proxy that sets
`X-Forwarded-For`. Otherwise clients could pick their own address and get
around the login rate limit.

## Storage

| Variable         | Default          | Notes                                                    |
| ---------------- | ---------------- | -------------------------------------------------------- |
| `DATABASE_PATH`  | `data/status.db` | Created with its directory if missing                    |
| `RETENTION_DAYS` | `30`             | 8 to 3650. Raw checks older than this are pruned hourly  |

Raw checks feed the 24 hour and 7 day uptime, the response time chart and
the check history. Daily rollups feed the 90 day uptime and the status page
bars and are never pruned, so a short retention does not change the
long-range numbers.

Back up the database with the SQLite CLI while the server runs:

```sh
sqlite3 data/status.db ".backup 'status-backup.db'"
```

## Admin access

| Variable         | Default   | Notes                                                       |
| ---------------- | --------- | ----------------------------------------------------------- |
| `ADMIN_USERNAME` | `admin`   | Only used when the database has no users yet                |
| `ADMIN_PASSWORD` | generated | 8+ characters. Without it a random password is logged once  |
| `SESSION_TTL`    | `168h`    | At least `1m`                                               |
| `COOKIE_SECURE`  | `false`   | Set to `true` when the site is served over HTTPS            |

Changing `ADMIN_PASSWORD` later does not change an existing account; use
**Account → Change password** in the admin area. Changing the password logs
out every other session.

Five failed logins from one address block further attempts from it for 15
minutes.

## API

| Variable    | Default | Notes                                                        |
| ----------- | ------- | ------------------------------------------------------------ |
| `API_TOKEN` | empty   | 16+ characters. Empty disables everything but `/api/status`  |

Generate one with `openssl rand -base64 32`.

## Display

| Variable     | Default  | Notes                                                           |
| ------------ | -------- | --------------------------------------------------------------- |
| `SITE_TITLE` | `Status` | Shown in the header and the browser title                       |
| `TIMEZONE`   | `UTC`    | IANA name such as `Europe/Bucharest`; tz data is built in       |

The time zone is used for every date on the pages and for the times typed
into the maintenance form. Daily uptime bars are always UTC days.

## Checks

| Variable        | Default | Notes                                    |
| --------------- | ------- | ---------------------------------------- |
| `CHECK_WORKERS` | `8`     | 1 to 256 checks running at the same time |

Each monitor has its own interval (10 seconds to 24 hours), timeout (1 to
60 seconds and shorter than the interval) and number of retries (0 to 10).
After a restart the first round of checks is spread over ten seconds.

## Demo data

| Variable          | Default                 | Notes                                          |
| ----------------- | ----------------------- | ---------------------------------------------- |
| `SEED_DEMO`       | `false`                 | Only seeds a database without monitors         |
| `DEMO_TARGET_URL` | `http://localhost:8081` | Base URL of `cmd/demotarget`                   |

The demo target itself reads `LISTEN_ADDR` (default `:8081`) and
`WEBHOOK_SECRET`, which it uses to check the signature of incoming
webhooks.
