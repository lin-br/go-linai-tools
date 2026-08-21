-- internal/rag/store/schema.sql
-- DDL for the pgvector-backed chunk repository. Apply manually or via
-- Store.InitSchema. Kept alongside the code for human review (MP8 spec).

CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE IF NOT EXISTS chunks (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    content     TEXT NOT NULL,
    embedding   vector(2048) NOT NULL,
    metadata    JSONB NOT NULL DEFAULT '{}'::jsonb,
    source_path TEXT NOT NULL
);
