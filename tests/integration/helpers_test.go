//go:build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/campoy/techcheck/internal/llm"
	"github.com/campoy/techcheck/internal/research"
	"github.com/campoy/techcheck/internal/search"
)

func contextWithTimeout(t *testing.T, d time.Duration) (context.Context, context.CancelFunc) {
	t.Helper()
	return context.WithTimeout(context.Background(), d)
}

// fakeLLM and fakeSearcher keep integration tests hermetic: they answer
// like the providers would, with canned content, so runs complete without
// API keys. The worker binary ships equivalents behind
// TECHCHECK_FAKE_PROVIDERS=1.

type fakeLLM struct{}

func (fakeLLM) Generate(ctx context.Context, tier llm.Tier, prompt string, out any) error {
	switch v := out.(type) {
	case *[]string:
		*v = []string{"What is the funding history?", "Who are the founders?"}
	case *[]research.Finding:
		*v = []research.Finding{{
			SourceURL:  "https://example.com/fake",
			Claim:      "raised a $5M seed round",
			Category:   research.CategoryFunding,
			Confidence: 0.9,
		}}
	case *research.CompanyBrief:
		*v = research.CompanyBrief{
			OneLiner:     "fake one-liner for hermetic runs",
			FitScore:     5,
			FitRationale: "fake rationale",
			Sources:      []string{"https://example.com/fake"},
		}
	}
	return nil
}

type fakeSearcher struct{}

func (fakeSearcher) Search(ctx context.Context, query string, max int) ([]search.Result, error) {
	return []search.Result{{
		URL:     "https://example.com/fake",
		Title:   "Fake result",
		Content: "canned content for hermetic integration runs",
	}}, nil
}
