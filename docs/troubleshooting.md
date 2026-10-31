# Troubleshooting

## A monitor is down but the site works in my browser

Open the monitor in the admin area and look at the error column of the
check history.

- **`timed out after 10s`**: the target answered slower than the monitor
  timeout. Raise the timeout (it must stay below the interval) or find out
  why the endpoint is slow.
- **`unexpected status 403`** or **`expected status 200, got 403`**: the
  target blocks the check. Some firewalls and CDNs block unknown clients;
  allow the `status-page/1.0` user agent or the server address.
- **`keyword "..." not found in response`**: the keyword is case
  sensitive and only the first 1 MiB of the body is searched. Pages that
  build their content with JavaScript do not contain it in the HTML.
- **`dial tcp: lookup ... no such host`**: the name does not resolve from
  where status-page runs. Inside Docker, `localhost` is the container
  itself; use the service name or `host.docker.internal`.
- **`x509: certificate signed by unknown authority`**: the target uses a
  private CA. Add the CA to the system trust store of the container.

## A monitor went down and back up but there is no incident

Incidents are not opened while a maintenance window covers the monitor's
component. Check **Maintenance** for a window that is in progress. Monitors
without a component are only covered by windows for all components.

## The same incident keeps getting reopened

A monitor that fails again within ten minutes of recovering reopens its
last incident on purpose, so a flapping service gives one incident with a
timeline instead of many. Add retries to the monitor if single failed
checks are expected.

## 90 day uptime looks wrong after changing RETENTION_DAYS

It does not depend on the retention: the 90 day figure and the bars come
from daily rollups, which are never pruned. The 24 hour and 7 day figures
use raw checks; with fewer than 8 days of retention the 7 day figure would
be short, which is why 8 is the minimum.

## Webhooks are not arriving

Open the webhook in **Webhooks** to see the delivery log. Failed attempts
show the status code or the network error. `4xx` answers other than `429`
are not retried. Use **Send test** to try a delivery right away.

If the signature check fails on your side, make sure it runs over the raw
request body, before any JSON parsing or re-encoding.

## "The form has expired"

Admin forms carry a CSRF token tied to the login session. Reload the page
after logging in again, or when the session expired (`SESSION_TTL`).

## Too many failed attempts on the login page

After five failed logins an address is blocked for 15 minutes. Behind a
reverse proxy every client may share the proxy's address; set
`TRUST_PROXY=true` so the real client address is used.

## I lost the admin password

Stop the server, then delete the users so the next start creates the
account from `ADMIN_USERNAME` and `ADMIN_PASSWORD` again:

```sh
sqlite3 data/status.db "DELETE FROM sessions; DELETE FROM users;"
ADMIN_PASSWORD='a new password' ./statuspage
```

## "database is locked"

Another process holds a write lock on the database file, for example a
second status-page instance or an open `sqlite3` shell in a transaction.
Run one instance per database file.
