-- Runs once on first Postgres startup. The temporal and temporal_visibility
-- databases are created by Temporal's auto-setup; research holds techcheck's
-- own data (pgvector corpus, finding archive).
CREATE DATABASE research;
\connect research
CREATE EXTENSION IF NOT EXISTS vector;
