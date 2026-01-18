-- One row per monitor and UTC day, kept long after the raw checks are gone.
CREATE TABLE daily_stats (
    monitor_id     INTEGER NOT NULL REFERENCES monitors (id) ON DELETE CASCADE,
    day            TEXT    NOT NULL,
    total          INTEGER NOT NULL DEFAULT 0,
    up             INTEGER NOT NULL DEFAULT 0,
    latency_sum_ms INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (monitor_id, day)
);
