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
	"fmt"
	"math"
)

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
	// returning the number of newly indexed chunks; re-indexing unchanged
	// content returns 0.
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
	chunks := SplitMarkdown(docPath, content)
	if len(chunks) == 0 {
		return 0, nil
	}
	texts := make([]string, len(chunks))
	for i, ch := range chunks {
		texts[i] = ch.Content
	}
	embeddings, err := c.Embedder.Embed(ctx, texts)
	if err != nil {
		return 0, fmt.Errorf("embedding %s: %w", docPath, err)
	}
	return c.Store.Upsert(ctx, chunks, embeddings)
}

// mmrLambda weighs relevance against novelty during retrieval.
const mmrLambda = 0.7

// Retrieve implements Client: embeds the queries, over-fetches candidates
// for each, and selects a diverse top-k via maximal marginal relevance
// against the queries' centroid.
func (c *Corpus) Retrieve(ctx context.Context, queries []string, k int) ([]Excerpt, error) {
	if len(queries) == 0 || k <= 0 {
		return nil, nil
	}
	embeddings, err := c.Embedder.Embed(ctx, queries)
	if err != nil {
		return nil, fmt.Errorf("embedding queries: %w", err)
	}

	seen := map[string]bool{}
	var candidates []Excerpt
	for _, emb := range embeddings {
		found, err := c.Store.Search(ctx, emb, k*3)
		if err != nil {
			return nil, err
		}
		for _, e := range found {
			key := e.DocPath + "\x00" + e.Section + "\x00" + e.Content
			if !seen[key] {
				seen[key] = true
				candidates = append(candidates, e)
			}
		}
	}
	return MMR(centroid(embeddings), candidates, k, mmrLambda), nil
}

// centroid is the normalized mean of the vectors.
func centroid(vectors [][]float32) []float32 {
	if len(vectors) == 0 {
		return nil
	}
	out := make([]float32, len(vectors[0]))
	for _, v := range vectors {
		for i := range min(len(v), len(out)) {
			out[i] += v[i]
		}
	}
	var norm float64
	for _, x := range out {
		norm += float64(x) * float64(x)
	}
	if norm > 0 {
		scale := float32(1 / math.Sqrt(norm))
		for i := range out {
			out[i] *= scale
		}
	}
	return out
}
