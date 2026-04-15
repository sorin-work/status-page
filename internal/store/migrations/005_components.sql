CREATE TABLE components (
    id          INTEGER PRIMARY KEY,
    name        TEXT    NOT NULL,
    description TEXT    NOT NULL DEFAULT '',
    position    INTEGER NOT NULL DEFAULT 0,
    created_at  INTEGER NOT NULL
);

ALTER TABLE monitors ADD COLUMN component_id INTEGER REFERENCES components (id) ON DELETE SET NULL;
ALTER TABLE incidents ADD COLUMN component_id INTEGER REFERENCES components (id) ON DELETE SET NULL;
