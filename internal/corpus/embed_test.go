package corpus_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/campoy/techcheck/internal/corpus"
)

// The Voyage client sends the documented request shape and parses
// embeddings back in input order.
func TestVoyageEmbed(t *testing.T) {
	canned := [][]float32{make([]float32, corpus.Dims), make([]float32, corpus.Dims)}
	canned[0][0], canned[1][1] = 1, 1

	var gotPath, gotAuth string
	var gotBody struct {
		Model string   `json:"model"`
		Input []string `json:"input"`
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		require.NoError(t, json.NewDecoder(r.Body).Decode(&gotBody))

		type item struct {
			Embedding []float32 `json:"embedding"`
		}
		resp := struct {
			Data []item `json:"data"`
		}{Data: []item{{canned[0]}, {canned[1]}}}
		require.NoError(t, json.NewEncoder(w).Encode(resp))
	}))
	defer srv.Close()

	v := &corpus.Voyage{APIKey: "test-key", BaseURL: srv.URL}
	got, err := v.Embed(t.Context(), []string{"first text", "second text"})
	require.NoError(t, err)

	require.Equal(t, "/v1/embeddings", gotPath)
	require.Equal(t, "Bearer test-key", gotAuth)
	require.Equal(t, []string{"first text", "second text"}, gotBody.Input)
	require.NotEmpty(t, gotBody.Model)

	require.Equal(t, canned, got)
}

// Non-200 responses surface as errors, not as empty embeddings.
func TestVoyageEmbedError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusUnauthorized)
	}))
	defer srv.Close()

	v := &corpus.Voyage{APIKey: "bad", BaseURL: srv.URL}
	_, err := v.Embed(t.Context(), []string{"text"})
	require.Error(t, err)
}

// The hermetic embedder is deterministic, Dims-dimensional, unit-length,
// and lexically meaningful: shared vocabulary means higher cosine.
func TestLexicalEmbedder(t *testing.T) {
	e := corpus.LexicalEmbedder{}
	texts := []string{
		"military defense contracts and surveillance",
		"defense contracts, military work, surveillance programs",
		"kubernetes database performance tuning",
	}
	vecs, err := e.Embed(t.Context(), texts)
	require.NoError(t, err)
	require.Len(t, vecs, 3)
	for _, v := range vecs {
		require.Len(t, v, corpus.Dims)
		var norm float64
		for _, x := range v {
			norm += float64(x) * float64(x)
		}
		require.InDelta(t, 1.0, norm, 1e-3, "embeddings are L2-normalized")
	}

	again, err := e.Embed(t.Context(), texts[:1])
	require.NoError(t, err)
	require.Equal(t, vecs[0], again[0], "embedding is deterministic")

	require.Greater(t, cosine(vecs[0], vecs[1]), cosine(vecs[0], vecs[2]),
		"texts sharing vocabulary must be closer than unrelated texts")
}
