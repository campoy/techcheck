package corpus

import (
	"context"
	"net/http"
)

// Embedder turns texts into vectors of Dims dimensions, in input order.
type Embedder interface {
	Embed(ctx context.Context, texts []string) ([][]float32, error)
}

const (
	defaultVoyageURL = "https://api.voyageai.com"
	voyageModel      = "voyage-3.5"
)

// Voyage is an Embedder backed by the hosted Voyage AI embeddings API — the
// one hosted embedding dependency, mirroring the LLM and search decisions.
type Voyage struct {
	APIKey  string
	BaseURL string // defaults to the public API; tests override it
	HTTP    *http.Client
}

// Embed implements Embedder.
func (v *Voyage) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	return nil, errNotImplemented
}

// LexicalEmbedder is a deterministic, offline Embedder: a feature-hashed
// bag-of-words, L2-normalized, so texts sharing vocabulary get high cosine
// similarity. It keeps integration tests and TECHCHECK_FAKE_PROVIDERS runs
// hermetic; real runs use Voyage.
type LexicalEmbedder struct{}

// Embed implements Embedder.
func (LexicalEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	return nil, errNotImplemented
}
