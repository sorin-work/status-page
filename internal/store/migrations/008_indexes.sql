-- The status page lists open and recently resolved incidents on every
-- request; the admin list sorts all incidents by start time.
CREATE INDEX incidents_status_started ON incidents (status, started_at);
CREATE INDEX incidents_started ON incidents (started_at);

-- Component rollups join daily_stats with monitors by component.
CREATE INDEX monitors_component ON monitors (component_id);
