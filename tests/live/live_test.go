//go:build live

// Package live tests the real Tavily and Anthropic integrations. It needs
// TAVILY_API_KEY and ANTHROPIC_API_KEY and costs money: it runs via `make
// test-live` and the weekly live workflow, never in PR CI. Assertions are
// deliberately loose — this suite catches API drift (auth, request shape,
// response parsing), not model quality.
package live

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/campoy/techcheck/internal/llm"
	"github.com/campoy/techcheck/internal/research"
	"github.com/campoy/techcheck/internal/search"
)

func key(t *testing.T, name string) string {
	t.Helper()
	v := os.Getenv(name)
	if v == "" {
		t.Skipf("%s not set", name)
	}
	return v
}

func TestTavilyLive(t *testing.T) {
	tavily := &search.Tavily{APIKey: key(t, "TAVILY_API_KEY")}

	ctx, cancel := contextWithTimeout(t, 30*time.Second)
	defer cancel()

	results, err := tavily.Search(ctx, "Anthropic company funding", 3)
	require.NoError(t, err)
	require.NotEmpty(t, results)
	require.LessOrEqual(t, len(results), 3)
	for _, r := range results {
		require.True(t, strings.HasPrefix(r.URL, "http"), "result URL %q", r.URL)
		require.NotEmpty(t, r.Content)
	}
}

func TestAnthropicStructuredOutputLive(t *testing.T) {
	client := llm.NewAnthropic(key(t, "ANTHROPIC_API_KEY"))

	ctx, cancel := contextWithTimeout(t, 60*time.Second)
	defer cancel()

	var findings []research.Finding
	prompt := `Extract findings from this text. Source URL: https://example.com/acme-seed
Text: "Acme, founded in 2024, raised a $5M seed round led by XYZ Ventures."`
	require.NoError(t, client.Generate(ctx, llm.TierExtract, prompt, &findings))
	require.NotEmpty(t, findings)
	for _, f := range findings {
		require.NotEmpty(t, f.Claim)
		require.NotEmpty(t, f.SourceURL)
	}
}
