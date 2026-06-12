package corpus

import (
	"context"
)

// Store persists embedded chunks in the research database's pgvector table.
type Store struct{}

// NewStore connects to the research database.
func NewStore(ctx context.Context, dsn string) (*Store, error) {
	return nil, errNotImplemented
}

// Migrate creates the corpus schema if it does not exist.
func (s *Store) Migrate(ctx context.Context) error {
	return errNotImplemented
}

// Upsert inserts chunks keyed by content hash and returns how many were new;
// re-upserting identical content is a no-op, which makes ingestion
// idempotent.
func (s *Store) Upsert(ctx context.Context, chunks []Chunk, embeddings [][]float32) (int, error) {
	return 0, errNotImplemented
}

// Search returns the limit nearest chunks to the embedding by cosine
// distance, embeddings included so callers can re-rank.
func (s *Store) Search(ctx context.Context, embedding []float32, limit int) ([]Excerpt, error) {
	return nil, errNotImplemented
}

// Close releases the connection pool.
func (s *Store) Close() {}

// MMR selects up to k results by maximal marginal relevance: each pick
// balances similarity to the query against similarity to what is already
// selected, so near-duplicate excerpts from the same document give way to
// broader coverage (FR-4.4). lambda in [0,1] weighs relevance vs. novelty.
func MMR(query []float32, candidates []Excerpt, k int, lambda float64) []Excerpt {
	return nil
}
