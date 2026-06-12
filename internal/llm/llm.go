// Package llm abstracts the hosted LLM behind a two-tier interface: a cheap
// tier for high-volume extraction and a strong tier for planning, analysis,
// and brief generation (FR-9.4).
package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
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

const (
	extractModel = "claude-haiku-4-5-20251001"
	reasonModel  = "claude-sonnet-4-6"
)

// NewAnthropic returns a Client backed by the Anthropic API.
func NewAnthropic(apiKey string) Client {
	return &anthropicClient{
		client: anthropic.NewClient(option.WithAPIKey(apiKey)),
		models: map[Tier]anthropic.Model{
			TierExtract: extractModel,
			TierReason:  reasonModel,
		},
	}
}

type anthropicClient struct {
	client anthropic.Client
	models map[Tier]anthropic.Model
}

func (c *anthropicClient) Generate(ctx context.Context, tier Tier, prompt string, out any) error {
	model, ok := c.models[tier]
	if !ok {
		return fmt.Errorf("unknown tier %q", tier)
	}

	msg, err := c.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     model,
		MaxTokens: 4096,
		System: []anthropic.TextBlockParam{{
			Text: "Respond with only valid JSON matching the requested structure. No prose, no markdown code fences.",
		}},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(prompt)),
		},
	})
	if err != nil {
		return fmt.Errorf("anthropic %s: %w", model, err)
	}

	var sb strings.Builder
	for _, block := range msg.Content {
		if block.Type == "text" {
			sb.WriteString(block.Text)
		}
	}
	text := strings.TrimSpace(sb.String())
	// Models occasionally fence the JSON despite instructions.
	text = strings.TrimPrefix(text, "```json")
	text = strings.TrimPrefix(text, "```")
	text = strings.TrimSuffix(text, "```")

	if err := json.Unmarshal([]byte(strings.TrimSpace(text)), out); err != nil {
		return fmt.Errorf("anthropic %s: parsing structured output: %w", model, err)
	}
	return nil
}
