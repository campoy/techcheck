package corpus

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"math"
	"net/http"
	"strings"
	"unicode"
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
	base := v.BaseURL
	if base == "" {
		base = defaultVoyageURL
	}
	httpc := v.HTTP
	if httpc == nil {
		httpc = http.DefaultClient
	}

	body, err := json.Marshal(map[string]any{
		"model": voyageModel,
		"input": texts,
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/v1/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+v.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("voyage: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("voyage: unexpected status %s", resp.Status)
	}

	var parsed struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("voyage: decoding response: %w", err)
	}
	if len(parsed.Data) != len(texts) {
		return nil, fmt.Errorf("voyage: got %d embeddings for %d texts", len(parsed.Data), len(texts))
	}

	out := make([][]float32, len(parsed.Data))
	for i, d := range parsed.Data {
		if len(d.Embedding) != Dims {
			return nil, fmt.Errorf("voyage: embedding %d has %d dimensions, want %d", i, len(d.Embedding), Dims)
		}
		out[i] = d.Embedding
	}
	return out, nil
}

// LexicalEmbedder is a deterministic, offline Embedder: a feature-hashed
// bag-of-words, L2-normalized, so texts sharing vocabulary get high cosine
// similarity. It keeps integration tests and TECHCHECK_FAKE_PROVIDERS runs
// hermetic; real runs use Voyage.
type LexicalEmbedder struct{}

// Embed implements Embedder.
func (LexicalEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i, text := range texts {
		v := make([]float32, Dims)
		tokens := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
			return !unicode.IsLetter(r) && !unicode.IsDigit(r)
		})
		for _, tok := range tokens {
			h := fnv.New32a()
			_, _ = h.Write([]byte(tok))
			v[h.Sum32()%Dims]++
		}
		var norm float64
		for _, x := range v {
			norm += float64(x) * float64(x)
		}
		if norm > 0 {
			scale := float32(1 / math.Sqrt(norm))
			for j := range v {
				v[j] *= scale
			}
		}
		out[i] = v
	}
	return out, nil
}
