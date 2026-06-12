package corpus_test

import (
	"math"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/campoy/techcheck/internal/corpus"
)

func cosine(a, b []float32) float64 {
	var dot, na, nb float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		na += float64(a[i]) * float64(a[i])
		nb += float64(b[i]) * float64(b[i])
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}

// FR-4.4: when near-duplicates from one document dominate raw similarity,
// MMR trades a little relevance for coverage instead of returning the same
// excerpt three times.
func TestMMRPrefersDiversity(t *testing.T) {
	query := []float32{1, 0, 0}
	dups := []corpus.Excerpt{
		{DocPath: "a.md", Content: "dup one", Embedding: []float32{0.99, 0.10, 0}},
		{DocPath: "a.md", Content: "dup two", Embedding: []float32{0.98, 0.12, 0}},
		{DocPath: "a.md", Content: "dup three", Embedding: []float32{0.97, 0.14, 0}},
	}
	distinct := []corpus.Excerpt{
		{DocPath: "b.md", Content: "different angle", Embedding: []float32{0.70, 0.70, 0}},
		{DocPath: "c.md", Content: "another topic", Embedding: []float32{0.60, 0, 0.80}},
	}
	candidates := append(append([]corpus.Excerpt{}, dups...), distinct...)

	got := corpus.MMR(query, candidates, 3, 0.7)
	require.Len(t, got, 3)

	require.Equal(t, "dup one", got[0].Content,
		"the most relevant candidate is always picked first")

	fromA := 0
	docs := map[string]bool{}
	for _, e := range got {
		docs[e.DocPath] = true
		if e.DocPath == "a.md" {
			fromA++
		}
	}
	require.Equal(t, 1, fromA, "near-duplicates from one document must not crowd the results (FR-4.4)")
	require.Len(t, docs, 3, "selection covers distinct documents")

	for _, e := range got {
		require.InDelta(t, cosine(query, e.Embedding), e.Similarity, 1e-6,
			"similarity reports relevance to the query")
	}
}

// With fewer candidates than k, MMR returns them all, most relevant first.
func TestMMRFewerCandidatesThanK(t *testing.T) {
	candidates := []corpus.Excerpt{
		{DocPath: "far.md", Embedding: []float32{0, 1, 0}},
		{DocPath: "near.md", Embedding: []float32{1, 0.1, 0}},
	}
	got := corpus.MMR([]float32{1, 0, 0}, candidates, 5, 0.7)
	require.Len(t, got, 2)
	require.Equal(t, "near.md", got[0].DocPath)
}

func TestMMREmpty(t *testing.T) {
	require.Empty(t, corpus.MMR([]float32{1}, nil, 3, 0.7))
}
