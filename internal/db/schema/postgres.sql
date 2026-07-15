CREATE TABLE IF NOT EXISTS load_events (
    id BIGSERIAL PRIMARY KEY,
    scenario_id TEXT NOT NULL,
    payload JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS scenario_events (
    id BIGSERIAL PRIMARY KEY,
    scenario_id TEXT NOT NULL,
    payload JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO scenario_events (scenario_id, payload, created_at)
SELECT load_events.scenario_id, load_events.payload, load_events.created_at
FROM load_events
WHERE load_events.payload ? 'status'
  AND NOT EXISTS (
      SELECT 1
      FROM scenario_events
      WHERE scenario_events.scenario_id = load_events.scenario_id
        AND scenario_events.payload = load_events.payload
        AND scenario_events.created_at = load_events.created_at
  );

CREATE TABLE IF NOT EXISTS chaos_events (
    id BIGSERIAL PRIMARY KEY,
    experiment_name TEXT NOT NULL,
    payload JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS metrics_snapshots (
    id BIGSERIAL PRIMARY KEY,
    snapshot JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
