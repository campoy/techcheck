// Package corpus maintains the searchable personal corpus (FR-4): resume,
// past evaluation briefs, the criteria document, and exported notes are
// split into coherent chunks, embedded, and stored in pgvector. Retrieval
// surfaces diverse, relevant excerpts — including precedents from past
// evaluations — to inform the current run.
package corpus

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
)

var errNotImplemented = errors.New("not implemented")

// Dims is the embedding dimensionality used across the whole corpus; one
// model embeds everything, so consistency is structural.
const Dims = 1024

// Chunk is one indexable piece of a document.
type Chunk struct {
	DocPath string `json:"doc_path"` // source document, relative to the corpus root
	Section string `json:"section"`  // heading trail, e.g. "Scale AI > Risks"
	Content string `json:"content"`
}

// Hash is the chunk's stable identity for idempotent upserts: re-ingesting
// unchanged content is a no-op (same hash), changed content gets a new row.
func (c Chunk) Hash() string {
	h := sha256.New()
	for _, part := range []string{c.DocPath, c.Section, c.Content} {
		h.Write([]byte(part))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

// Excerpt is one retrieval hit returned to the research workflow.
type Excerpt struct {
	DocPath    string    `json:"doc_path"`
	Section    string    `json:"section"`
	Content    string    `json:"content"`
	Similarity float64   `json:"similarity"`
	Embedding  []float32 `json:"-"` // retrieval-internal; never serialized into workflow state
}

// Client is what consumers depend on: indexing documents into the corpus
// and retrieving relevant, diverse excerpts from it.
type Client interface {
	// IndexDocument splits, embeds, and upserts one document's content,
	// returning the number of chunks indexed.
	IndexDocument(ctx context.Context, docPath, content string) (int, error)
	// Retrieve returns at most k excerpts relevant to the queries,
	// de-duplicated for diversity (FR-4.2–FR-4.4).
	Retrieve(ctx context.Context, queries []string, k int) ([]Excerpt, error)
}

// Indexer is the ingestion-side slice of Client.
type Indexer interface {
	IndexDocument(ctx context.Context, docPath, content string) (int, error)
}

// Corpus ties the store and the embedder together; it implements Client.
type Corpus struct {
	Store    *Store
	Embedder Embedder
}

// IndexDocument implements Client.
func (c *Corpus) IndexDocument(ctx context.Context, docPath, content string) (int, error) {
	return 0, errNotImplemented
}

// Retrieve implements Client: embeds the queries, over-fetches candidates,
// and selects a diverse top-k via maximal marginal relevance.
func (c *Corpus) Retrieve(ctx context.Context, queries []string, k int) ([]Excerpt, error) {
	return nil, errNotImplemented
}
