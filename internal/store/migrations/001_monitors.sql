CREATE TABLE monitors (
    id               INTEGER PRIMARY KEY,
    name             TEXT    NOT NULL,
    type             TEXT    NOT NULL CHECK (type IN ('http', 'tcp')),
    target           TEXT    NOT NULL,
    interval_seconds INTEGER NOT NULL,
    timeout_seconds  INTEGER NOT NULL,
    expected_status  INTEGER NOT NULL DEFAULT 0,
    keyword          TEXT    NOT NULL DEFAULT '',
    retries          INTEGER NOT NULL DEFAULT 0,
    paused           INTEGER NOT NULL DEFAULT 0,
    status           TEXT    NOT NULL DEFAULT 'unknown',
    created_at       INTEGER NOT NULL,
    updated_at       INTEGER NOT NULL
);

CREATE TABLE checks (
    id          INTEGER PRIMARY KEY,
    monitor_id  INTEGER NOT NULL REFERENCES monitors (id) ON DELETE CASCADE,
    checked_at  INTEGER NOT NULL,
    up          INTEGER NOT NULL,
    latency_ms  INTEGER NOT NULL,
    status_code INTEGER NOT NULL DEFAULT 0,
    error       TEXT    NOT NULL DEFAULT ''
);

CREATE INDEX checks_monitor_time ON checks (monitor_id, checked_at);
