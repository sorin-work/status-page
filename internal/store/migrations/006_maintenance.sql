CREATE TABLE maintenance (
    id           INTEGER PRIMARY KEY,
    title        TEXT    NOT NULL,
    description  TEXT    NOT NULL DEFAULT '',
    component_id INTEGER REFERENCES components (id) ON DELETE CASCADE,
    starts_at    INTEGER NOT NULL,
    ends_at      INTEGER NOT NULL,
    created_at   INTEGER NOT NULL
);

CREATE INDEX maintenance_ends ON maintenance (ends_at);
