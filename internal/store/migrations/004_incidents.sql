CREATE TABLE incidents (
    id          INTEGER PRIMARY KEY,
    title       TEXT    NOT NULL,
    status      TEXT    NOT NULL,
    impact      TEXT    NOT NULL,
    monitor_id  INTEGER REFERENCES monitors (id) ON DELETE SET NULL,
    automatic   INTEGER NOT NULL DEFAULT 0,
    started_at  INTEGER NOT NULL,
    resolved_at INTEGER,
    updated_at  INTEGER NOT NULL
);

CREATE INDEX incidents_monitor_open ON incidents (monitor_id, status);

CREATE TABLE incident_updates (
    id          INTEGER PRIMARY KEY,
    incident_id INTEGER NOT NULL REFERENCES incidents (id) ON DELETE CASCADE,
    status      TEXT    NOT NULL,
    message     TEXT    NOT NULL,
    created_at  INTEGER NOT NULL
);

CREATE INDEX incident_updates_incident ON incident_updates (incident_id, created_at);
