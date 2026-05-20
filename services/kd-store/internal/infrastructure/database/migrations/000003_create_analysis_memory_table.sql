CREATE TABLE IF NOT EXISTS analysis_memory (
    id BIGSERIAL PRIMARY KEY,
    cluster_uid TEXT NOT NULL,
    memory_key TEXT NOT NULL,
    namespace TEXT NOT NULL,
    analysis TEXT NOT NULL,
    embedding VECTOR(1536) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_analysis_memory_memory_key ON analysis_memory (memory_key);
CREATE INDEX IF NOT EXISTS idx_analysis_memory_embedding_hnsw
    ON analysis_memory USING hnsw (embedding vector_cosine_ops);
