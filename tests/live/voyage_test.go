//go:build live

package live

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/campoy/techcheck/internal/corpus"
)

// Drift check for the Voyage embeddings API: auth, request shape, response
// parsing, and the dimensionality the corpus schema depends on.
func TestVoyageEmbedLive(t *testing.T) {
	v := &corpus.Voyage{APIKey: key(t, "VOYAGE_API_KEY")}

	ctx, cancel := contextWithTimeout(t, 30*time.Second)
	defer cancel()

	vecs, err := v.Embed(ctx, []string{"company evaluation brief", "stage mismatch precedent"})
	require.NoError(t, err)
	require.Len(t, vecs, 2)
	for _, vec := range vecs {
		require.Len(t, vec, corpus.Dims, "corpus schema assumes %d-dim embeddings", corpus.Dims)
		var norm float64
		for _, x := range vec {
			norm += float64(x) * float64(x)
		}
		require.Positive(t, norm)
	}
}
