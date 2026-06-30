# Webhooks

Add webhooks under **Admin → Webhooks**. Each one gets a random signing
secret. status-page sends a `POST` with a JSON body when:

| Event              | When                                                  |
| ------------------ | ----------------------------------------------------- |
| `monitor.down`     | A monitor is marked down (after its retries)          |
| `monitor.up`       | A monitor that was down passes a check again          |
| `incident.created` | An incident is opened, automatically or by hand       |
| `incident.updated` | A timeline update is posted or an incident resolves   |
| `test`             | You press **Send test**                               |

## Request

```http
POST /your/endpoint HTTP/1.1
Content-Type: application/json
User-Agent: status-page-webhooks/1
X-StatusPage-Event: monitor.down
X-StatusPage-Delivery: 3q2-x9Kd0aZf
X-StatusPage-Signature: sha256=5f1c...e9

{
  "event": "monitor.down",
  "timestamp": "2026-06-30T19:40:00Z",
  "monitor": {
    "id": 3,
    "name": "Checkout",
    "target": "https://shop.example.com/checkout",
    "status": "down",
    "error": "expected status 200, got 502"
  }
}
```

Incident events carry an `incident` object instead:

```json
{
  "event": "incident.updated",
  "timestamp": "2026-06-30T19:52:00Z",
  "incident": {
    "id": 12,
    "title": "Checkout is down",
    "status": "resolved",
    "impact": "major",
    "message": "Checkout is responding again. Total downtime: 12m.",
    "url": "/incidents/12"
  }
}
```

`url` is relative to the status page address.

## Verifying the signature

`X-StatusPage-Signature` is `sha256=` followed by the hex HMAC-SHA256 of the
raw request body, keyed with the webhook secret. Compute it over the bytes
you received, before parsing the JSON, and compare in constant time.

```go
mac := hmac.New(sha256.New, []byte(secret))
mac.Write(body)
expected := "sha256=" + hex.EncodeToString(mac.Sum(nil))
ok := hmac.Equal([]byte(expected), []byte(r.Header.Get("X-StatusPage-Signature")))
```

```python
expected = "sha256=" + hmac.new(secret.encode(), body, hashlib.sha256).hexdigest()
ok = hmac.compare_digest(expected, request.headers["X-StatusPage-Signature"])
```

## Delivery and retries

Deliveries are sent in the background with a 10 second timeout. A
delivery succeeds when the receiver answers with any `2xx` status.

Network errors, `429` and `5xx` answers are retried after 5 seconds, 30
seconds and 2 minutes. Other `4xx` answers are not retried. Every attempt
of the same delivery carries the same `X-StatusPage-Delivery` value, so a
receiver can ignore duplicates.

Each webhook keeps a log of its recent attempts; open the webhook in the
admin area to see status codes, durations and errors.

## Trying it locally

The demo setup in `docker-compose.yml` registers a webhook that points at
the demo target. To see what it received:

```sh
docker compose exec demo-target wget -qO- localhost:8081/webhooks
```
