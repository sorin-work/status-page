# JSON API

All endpoints live under `/api` and speak JSON. Times are RFC 3339 in UTC.

## Authentication

Everything except `GET /api/status` needs the token from the `API_TOKEN`
setting, sent as a bearer token:

```sh
curl -H "Authorization: Bearer $API_TOKEN" http://localhost:4417/api/monitors
```

Without `API_TOKEN` the protected endpoints answer `401`.

## Errors

Errors have an `error` message and, for validation problems, a `fields`
object:

```json
{
  "error": "validation failed",
  "fields": {
    "target": "must start with http:// or https://"
  }
}
```

| Status | Meaning                                   |
| ------ | ----------------------------------------- |
| 400    | Malformed JSON, unknown field, bad query  |
| 401    | Missing or wrong token                    |
| 404    | No such monitor or incident               |
| 413    | Body larger than 1 MiB                    |
| 422    | Validation failed, see `fields`           |

## Status

### `GET /api/status`

Public summary of the status page: overall state, components with their
90 day uptime, open incidents and current or upcoming maintenance. Sends
`Access-Control-Allow-Origin: *` so it can be embedded in other sites.

```json
{
  "status": "operational",
  "headline": "All systems operational",
  "components": [
    {
      "id": 1,
      "name": "Website",
      "status": "operational",
      "uptime_90d": { "percent": 99.98, "up": 129574, "total": 129600 }
    }
  ],
  "incidents": [],
  "maintenance": [],
  "updated_at": "2026-06-21T11:20:00Z"
}
```

Component and overall states are `operational`, `maintenance`,
`partial_outage`, `major_outage` and `nodata`.

## Monitors

### `GET /api/monitors`

```json
{
  "monitors": [
    {
      "id": 1,
      "name": "Storefront",
      "type": "http",
      "target": "http://demo-target:8081/",
      "interval_seconds": 60,
      "timeout_seconds": 10,
      "expected_status": 0,
      "keyword": "Acme Shop",
      "follow_redirects": true,
      "retries": 1,
      "paused": false,
      "status": "up",
      "component_id": 1,
      "created_at": "2026-06-01T10:00:00Z",
      "updated_at": "2026-06-01T10:00:00Z"
    }
  ]
}
```

### `GET /api/monitors/{id}`

One monitor, same shape as above.

### `POST /api/monitors`

Creates a monitor. Only `name` and `target` are required.

| Field              | Default | Notes                                              |
| ------------------ | ------- | -------------------------------------------------- |
| `name`             |         | Up to 100 characters                               |
| `type`             | `http`  | `http` or `tcp`                                    |
| `target`           |         | URL for HTTP, `host:port` for TCP                  |
| `interval_seconds` | 60      | 10 seconds to 24 hours                             |
| `timeout_seconds`  | 10      | 1 to 60 seconds, shorter than the interval         |
| `expected_status`  | 0       | 0 accepts any 2xx/3xx; HTTP only                   |
| `keyword`          | `""`    | Text the body must contain; HTTP only              |
| `follow_redirects` | true    | false checks the first response; HTTP only         |
| `retries`          | 0       | Extra failed checks before the monitor goes down   |
| `paused`           | false   |                                                    |
| `component_id`     | null    | Shows the monitor on the status page               |

Answers `201` with the monitor and a `Location` header.

### `PUT /api/monitors/{id}`

Replaces the editable fields, same body as `POST`.

### `DELETE /api/monitors/{id}`

Deletes the monitor and its history. Answers `204`.

### `POST /api/monitors/{id}/pause` and `POST /api/monitors/{id}/resume`

Stops or restarts the checks of a monitor, for example around a deploy.
Answers with the monitor.

### `GET /api/monitors/{id}/checks?limit=100&offset=0`

Check history, newest first. `limit` is 1 to 1000.

```json
{
  "checks": [
    { "checked_at": "2026-06-21T11:19:00Z", "up": true, "latency_ms": 84, "status_code": 200 },
    { "checked_at": "2026-06-21T11:18:00Z", "up": false, "latency_ms": 10001, "error": "timed out after 10s" }
  ],
  "total": 10080,
  "limit": 100,
  "offset": 0
}
```

### `GET /api/monitors/{id}/uptime`

```json
{
  "monitor_id": 1,
  "24h": { "percent": 100, "up": 1440, "total": 1440 },
  "7d":  { "percent": 99.95, "up": 10075, "total": 10080 },
  "90d": { "percent": 99.97, "up": 129561, "total": 129600 }
}
```

`percent` is `null` when there are no checks in the period.

## Incidents

### `GET /api/incidents?limit=50&offset=0`

Incidents newest first, without their timelines.

### `GET /api/incidents/{id}`

```json
{
  "id": 4,
  "title": "Checkout is down",
  "status": "resolved",
  "impact": "major",
  "component_id": 3,
  "monitor_id": 3,
  "automatic": true,
  "started_at": "2026-06-18T14:00:00Z",
  "resolved_at": "2026-06-18T14:40:00Z",
  "updates": [
    { "id": 9, "status": "resolved", "message": "Checkout is responding again.", "created_at": "2026-06-18T14:40:00Z" }
  ]
}
```

Statuses are `investigating`, `identified`, `monitoring` and `resolved`.
Impacts are `none`, `minor`, `major` and `critical`.

### `POST /api/incidents`

```json
{ "title": "Webhooks delayed", "impact": "minor", "message": "Deliveries are queued.", "component_id": 2 }
```

`status` defaults to `investigating` and `impact` to `minor`.

### `POST /api/incidents/{id}/updates`

```json
{ "status": "resolved", "message": "The queue has drained." }
```

Adds a timeline entry and moves the incident to its status. Resolving sets
`resolved_at`; posting a non-resolved status reopens the incident.

## Components

### `GET /api/components`

```json
{ "components": [ { "id": 1, "name": "Website", "description": "Storefront and marketing pages", "position": 1 } ] }
```

## Maintenance

### `GET /api/maintenance`

Windows that are running or have not started yet, soonest first.

### `POST /api/maintenance`

```json
{
  "title": "Database upgrade",
  "description": "Checkout may be read-only for a few minutes.",
  "component_id": 5,
  "starts_at": "2026-09-10T02:00:00Z",
  "ends_at": "2026-09-10T02:30:00Z"
}
```

`component_id` is optional; without it the window covers every component.
Windows can be at most 7 days long and must not be over already.

### `DELETE /api/maintenance/{id}`

Removes a window. Answers `204`.
