# Changelog

## 1.0.0 - 2026-10-31

First stable release.

### Monitoring

- HTTP monitors with interval, timeout, expected status code, keyword
  match on the first MiB of the body and an option not to follow redirects.
- TCP port monitors.
- Retries: a monitor goes down after the first failed check plus its
  retries.
- Concurrent scheduler with a worker limit (`CHECK_WORKERS`); a monitor is
  never checked twice at the same time, first checks after a restart are
  spread out, and running checks finish on shutdown.
- Uptime over 24 hours, 7 days and 90 days, daily rollups that survive the
  raw check retention (`RETENTION_DAYS`).
- Response time chart for the last 24 hours or 7 days, paginated check
  history and CSV export.

### Incidents and maintenance

- Automatic incidents when a monitor goes down, resolved with the total
  downtime when it recovers; a flapping monitor reopens its last incident.
- Manual incidents with impact, affected component and a timeline of
  updates.
- Scheduled maintenance per component or for everything; no incidents are
  opened for covered monitors while a window is active.

### Status page

- Components with their current state, 90-day uptime bars with tooltips,
  overall banner, open incidents, planned maintenance and recent incidents.
- Public incident pages and a monthly incident history.
- Responsive layout, outage state in the page title.

### Notifications

- Webhooks for `monitor.down`, `monitor.up`, `incident.created` and
  `incident.updated`, signed with HMAC-SHA256, retried with backoff, with a
  delivery log and test deliveries.

### Admin and API

- Admin login with PBKDF2 password hashes, CSRF protection, login rate
  limiting, password change and confirmation messages.
- JSON API with bearer token for monitors, checks, uptime, components,
  incidents and maintenance, plus the public `GET /api/status`.

### Operations

- Single binary with embedded SQLite (no cgo), migrations and templates.
- Docker image, compose file with a demo target service, healthchecks and
  read-only containers.
- Request IDs, request logging, JSON logs (`LOG_FORMAT=json`), display time
  zone (`TIMEZONE`), `TRUST_PROXY` for reverse proxies.
- Demo data with 90 days of history.

### Fixed before the release

- Monitors went down one failed check too early.
- Slow checks were started again on every tick.
- Short outages were shown as 100.00% uptime.
- Incident messages could inject HTML into the status page.
- Incident updates could be posted through the API without a token.
- Logging out did not end the session on the server.
- The login redirect could point to another site.
- Check history pagination never reached the last page.
- The status page listed the oldest past incidents instead of the latest.
- Maintenance times were saved as UTC instead of the display time zone.
- TCP monitors without a port were accepted.
- Monitor names with non-Latin letters hit the length limit early.
