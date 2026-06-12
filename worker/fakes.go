package main

import (
	"context"

	"github.com/campoy/techcheck/internal/llm"
	"github.com/campoy/techcheck/internal/research"
	"github.com/campoy/techcheck/internal/search"
)

// Hermetic stand-ins for the paid providers, selected by
// TECHCHECK_FAKE_PROVIDERS=1. They let integration tests (and curious
// humans) run complete research workflows with no API keys and no cost.

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
		Content: "canned content for hermetic runs",
	}}, nil
}
