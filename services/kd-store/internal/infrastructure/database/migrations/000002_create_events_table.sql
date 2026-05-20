CREATE TABLE IF NOT EXISTS events (
    id BIGSERIAL PRIMARY KEY,
    cluster_uid TEXT NOT NULL,
    memory_key TEXT NOT NULL,
    severity TEXT NOT NULL,
    diagnosis TEXT NOT NULL,
    recommendations TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
