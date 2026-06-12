package corpus

import (
	"context"
	"fmt"
	"math"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	pgvector "github.com/pgvector/pgvector-go"
	pgxvector "github.com/pgvector/pgvector-go/pgx"
)

// Store persists embedded chunks in the research database's pgvector table.
type Store struct {
	pool *pgxpool.Pool
}

// NewStore connects to the research database.
func NewStore(ctx context.Context, dsn string) (*Store, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parsing dsn: %w", err)
	}
	cfg.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		return pgxvector.RegisterTypes(ctx, conn)
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("connecting to research db: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("pinging research db: %w", err)
	}
	return &Store{pool: pool}, nil
}

// Migrate creates the corpus schema if it does not exist.
func (s *Store) Migrate(ctx context.Context) error {
	stmts := []string{
		`CREATE EXTENSION IF NOT EXISTS vector`,
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS corpus_chunks (
			hash       TEXT PRIMARY KEY,
			doc_path   TEXT NOT NULL,
			section    TEXT NOT NULL DEFAULT '',
			content    TEXT NOT NULL,
			embedding  vector(%d) NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`, Dims),
		`CREATE INDEX IF NOT EXISTS corpus_chunks_embedding_idx
			ON corpus_chunks USING hnsw (embedding vector_cosine_ops)`,
	}
	for _, stmt := range stmts {
		if _, err := s.pool.Exec(ctx, stmt); err != nil {
			return fmt.Errorf("migrating corpus schema: %w", err)
		}
	}
	return nil
}

// Upsert inserts chunks keyed by content hash and returns how many were new;
// re-upserting identical content is a no-op, which makes ingestion
// idempotent.
func (s *Store) Upsert(ctx context.Context, chunks []Chunk, embeddings [][]float32) (int, error) {
	if len(chunks) != len(embeddings) {
		return 0, fmt.Errorf("%d chunks but %d embeddings", len(chunks), len(embeddings))
	}

	batch := &pgx.Batch{}
	for i, c := range chunks {
		batch.Queue(`INSERT INTO corpus_chunks (hash, doc_path, section, content, embedding)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (hash) DO NOTHING`,
			c.Hash(), c.DocPath, c.Section, c.Content, pgvector.NewVector(embeddings[i]))
	}

	br := s.pool.SendBatch(ctx, batch)
	defer func() { _ = br.Close() }()

	inserted := 0
	for range chunks {
		tag, err := br.Exec()
		if err != nil {
			return inserted, fmt.Errorf("upserting chunk: %w", err)
		}
		inserted += int(tag.RowsAffected())
	}
	return inserted, nil
}

// Search returns the limit nearest chunks to the embedding by cosine
// distance, embeddings included so callers can re-rank.
func (s *Store) Search(ctx context.Context, embedding []float32, limit int) ([]Excerpt, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT doc_path, section, content, embedding, 1 - (embedding <=> $1) AS similarity
		FROM corpus_chunks
		ORDER BY embedding <=> $1
		LIMIT $2`,
		pgvector.NewVector(embedding), limit)
	if err != nil {
		return nil, fmt.Errorf("searching corpus: %w", err)
	}
	defer rows.Close()

	var out []Excerpt
	for rows.Next() {
		var e Excerpt
		var vec pgvector.Vector
		if err := rows.Scan(&e.DocPath, &e.Section, &e.Content, &vec, &e.Similarity); err != nil {
			return nil, fmt.Errorf("scanning excerpt: %w", err)
		}
		e.Embedding = vec.Slice()
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("searching corpus: %w", err)
	}
	return out, nil
}

// Close releases the connection pool.
func (s *Store) Close() {
	s.pool.Close()
}

// dupThreshold is the cosine similarity above which two excerpts count as
// near-duplicates: such candidates are deferred until nothing else remains.
const dupThreshold = 0.95

// MMR selects up to k results by maximal marginal relevance: each pick
// balances similarity to the query against similarity to what is already
// selected, and near-duplicates of an already-picked excerpt are skipped
// while broader coverage is available (FR-4.4). lambda in [0,1] weighs
// relevance vs. novelty.
func MMR(query []float32, candidates []Excerpt, k int, lambda float64) []Excerpt {
	if k <= 0 || len(candidates) == 0 {
		return nil
	}

	rel := make([]float64, len(candidates))
	for i := range candidates {
		rel[i] = cosine(query, candidates[i].Embedding)
	}

	used := make([]bool, len(candidates))
	var selected []Excerpt
	var selectedIdx []int

	for len(selected) < k && len(selected) < len(candidates) {
		pick := func(skipDups bool) int {
			best, bestScore := -1, math.Inf(-1)
			for i := range candidates {
				if used[i] {
					continue
				}
				maxSim := 0.0
				for _, j := range selectedIdx {
					maxSim = math.Max(maxSim, cosine(candidates[i].Embedding, candidates[j].Embedding))
				}
				if skipDups && maxSim > dupThreshold {
					continue
				}
				score := lambda*rel[i] - (1-lambda)*maxSim
				if score > bestScore {
					best, bestScore = i, score
				}
			}
			return best
		}

		best := pick(true)
		if best < 0 {
			best = pick(false) // only near-duplicates remain
		}
		used[best] = true
		e := candidates[best]
		e.Similarity = rel[best]
		selected = append(selected, e)
		selectedIdx = append(selectedIdx, best)
	}
	return selected
}

func cosine(a, b []float32) float64 {
	n := min(len(a), len(b))
	var dot, na, nb float64
	for i := range n {
		dot += float64(a[i]) * float64(b[i])
		na += float64(a[i]) * float64(a[i])
		nb += float64(b[i]) * float64(b[i])
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}
