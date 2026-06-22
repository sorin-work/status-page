CREATE TABLE webhooks (
    id         INTEGER PRIMARY KEY,
    name       TEXT    NOT NULL,
    url        TEXT    NOT NULL,
    secret     TEXT    NOT NULL,
    enabled    INTEGER NOT NULL DEFAULT 1,
    created_at INTEGER NOT NULL
);

CREATE TABLE webhook_deliveries (
    id          INTEGER PRIMARY KEY,
    webhook_id  INTEGER NOT NULL REFERENCES webhooks (id) ON DELETE CASCADE,
    event       TEXT    NOT NULL,
    attempt     INTEGER NOT NULL,
    status_code INTEGER NOT NULL DEFAULT 0,
    error       TEXT    NOT NULL DEFAULT '',
    duration_ms INTEGER NOT NULL DEFAULT 0,
    created_at  INTEGER NOT NULL
);

CREATE INDEX webhook_deliveries_webhook ON webhook_deliveries (webhook_id, created_at);
