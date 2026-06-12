// Package llm abstracts the hosted LLM behind a two-tier interface: a cheap
// tier for high-volume extraction and a strong tier for planning, analysis,
// and brief generation (FR-9.4).
package llm

import (
	"context"
	"errors"
)

// Tier selects which model handles a request.
type Tier string

const (
	// TierExtract is the cheap model for high-volume extraction calls.
	TierExtract Tier = "extract"
	// TierReason is the strong model for planning, analysis, and briefs.
	TierReason Tier = "reason"
)

// Client generates a structured response to a prompt, unmarshaling the
// model's JSON output into out (a pointer to a struct or slice).
type Client interface {
	Generate(ctx context.Context, tier Tier, prompt string, out any) error
}

// NewAnthropic returns a Client backed by the Anthropic API.
func NewAnthropic(apiKey string) Client {
	return notImplemented{}
}

type notImplemented struct{}

func (notImplemented) Generate(context.Context, Tier, string, any) error {
	return errors.New("not implemented")
}
