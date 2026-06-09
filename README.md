# status-page

Self-hosted uptime monitoring and a public status page, written in Go.

It checks HTTP endpoints and TCP ports on a schedule, opens incidents when
something goes down, and shows a public page with the state of each
component and 90 days of uptime history.

## Quick start

```sh
docker compose up --build
```

Open http://localhost:4417 for the status page and
http://localhost:4417/admin to manage monitors. The demo login is
`admin` / `demo-password`.

The compose file also starts a small demo service that the seeded monitors
check, so there is something to look at right away.

## Development

```sh
make test   # go test ./...
make lint   # golangci-lint
make run    # starts the server on :4417 with data/status.db
```
