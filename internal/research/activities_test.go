package research_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/campoy/techcheck/internal/llm"
	"github.com/campoy/techcheck/internal/research"
	"github.com/campoy/techcheck/internal/search"
)

// fakeLLM records prompts per tier and fills out via the configured
// respond function.
type fakeLLM struct {
	prompts map[llm.Tier][]string
	respond func(tier llm.Tier, prompt string, out any) error
}

func newFakeLLM(respond func(tier llm.Tier, prompt string, out any) error) *fakeLLM {
	return &fakeLLM{prompts: map[llm.Tier][]string{}, respond: respond}
}

func (f *fakeLLM) Generate(ctx context.Context, tier llm.Tier, prompt string, out any) error {
	f.prompts[tier] = append(f.prompts[tier], prompt)
	return f.respond(tier, prompt, out)
}

// fill unmarshals canned JSON into the out pointer, mimicking structured
// output.
func fill(t *testing.T, out, value any) {
	t.Helper()
	raw, err := json.Marshal(value)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(raw, out))
}

type fakeSearcher struct {
	queries []string
	max     []int
	results []search.Result
}

func (f *fakeSearcher) Search(ctx context.Context, query string, max int) ([]search.Result, error) {
	f.queries = append(f.queries, query)
	f.max = append(f.max, max)
	return f.results, nil
}

// FR-2.1, FR-2.2, FR-9.4: planning produces open questions via the reason
// tier, and the prompt demands coverage of the five required areas.
func TestPlanResearch(t *testing.T) {
	fake := newFakeLLM(func(tier llm.Tier, prompt string, out any) error {
		fill(t, out, []string{
			"How much has acme raised and from whom?",
			"Who are the founders?",
		})
		return nil
	})
	a := &research.Activities{LLM: fake}

	plan, err := a.PlanResearch(context.Background(), research.PlanRequest{Company: "acme"})
	require.NoError(t, err)
	require.NotEmpty(t, plan)

	require.Empty(t, fake.prompts[llm.TierExtract], "planning must not use the extract tier")
	require.Len(t, fake.prompts[llm.TierReason], 1, "planning uses the reason tier (FR-9.4)")

	prompt := strings.ToLower(fake.prompts[llm.TierReason][0])
	for _, area := range []string{"funding", "founder", "product", "competitor", "hiring"} {
		require.Contains(t, prompt, area, "initial plan must demand %s coverage (FR-2.2)", area)
	}
}

// FR-3.1, FR-5.4 partial: web research queries the searcher for each open
// question but never exceeds the total search budget.
func TestWebResearchRespectsBudget(t *testing.T) {
	searcher := &fakeSearcher{results: []search.Result{
		{URL: "https://example.com/a", Title: "A", Content: "alpha"},
	}}
	a := &research.Activities{Searcher: searcher}

	questions := []string{"q1", "q2", "q3", "q4", "q5", "q6", "q7"}
	results, err := a.WebResearch(context.Background(), research.WebResearchRequest{
		Questions:   questions,
		MaxSearches: 3,
	})
	require.NoError(t, err)
	require.NotEmpty(t, results)
	require.Len(t, searcher.queries, 3, "search budget must cap total searches (FR-5.4)")
}

// FR-3.2, FR-3.4, FR-9.4: extraction parses results into findings on the
// extract tier and drops findings without a source URL rather than failing
// the batch.
func TestExtractFindings(t *testing.T) {
	fake := newFakeLLM(func(tier llm.Tier, prompt string, out any) error {
		fill(t, out, []research.Finding{
			{SourceURL: "https://example.com/a", Claim: "raised $5M", Category: research.CategoryFunding, Confidence: 0.9},
			{SourceURL: "", Claim: "unsourced rumor", Category: research.CategoryRisk, Confidence: 0.2},
			{SourceURL: "https://example.com/b", Claim: "founded by ex-Google team", Category: research.CategoryTeam, Confidence: 0.7},
		})
		return nil
	})
	a := &research.Activities{LLM: fake}

	findings, err := a.ExtractFindings(context.Background(), research.ExtractRequest{
		Results: []search.Result{{URL: "https://example.com/a", Title: "A", Content: "acme raised $5M"}},
	})
	require.NoError(t, err)
	require.Len(t, findings, 2, "the unsourced finding must be dropped (FR-3.4)")
	for _, f := range findings {
		require.NotEmpty(t, f.SourceURL)
		require.NoError(t, f.Validate())
	}

	require.Empty(t, fake.prompts[llm.TierReason], "extraction must not use the reason tier")
	require.NotEmpty(t, fake.prompts[llm.TierExtract], "extraction uses the cheap tier (FR-9.4)")
}

// FR-6.1–FR-6.3, FR-9.4: brief generation returns a validated brief on the
// reason tier and rejects invalid model output instead of passing it on.
func TestGenerateBrief(t *testing.T) {
	brief := research.CompanyBrief{
		Company:      "acme",
		OneLiner:     "roadrunner countermeasures",
		FitScore:     6,
		FitRationale: "stage fits, values unclear",
		Sources:      []string{"https://example.com/a"},
	}
	fake := newFakeLLM(func(tier llm.Tier, prompt string, out any) error {
		fill(t, out, brief)
		return nil
	})
	a := &research.Activities{LLM: fake}

	got, err := a.GenerateBrief(context.Background(), research.BriefRequest{
		Company: "acme",
		Findings: []research.Finding{
			{SourceURL: "https://example.com/a", Claim: "raised $5M", Category: research.CategoryFunding, Confidence: 0.9},
		},
	})
	require.NoError(t, err)
	require.Equal(t, brief.Company, got.Company)
	require.NoError(t, got.Validate())
	require.NotEmpty(t, fake.prompts[llm.TierReason], "briefs use the reason tier (FR-9.4)")

	invalid := brief
	invalid.FitScore = 0
	fake.respond = func(tier llm.Tier, prompt string, out any) error {
		fill(t, out, invalid)
		return nil
	}
	_, err = a.GenerateBrief(context.Background(), research.BriefRequest{Company: "acme"})
	require.Error(t, err, "invalid model output must not produce a brief")
}

// FR-8.2: approved briefs land in <BriefsDir>/<normalized-company>.md as
// Markdown containing the brief's substance and its sources.
func TestWriteBrief(t *testing.T) {
	dir := t.TempDir()
	a := &research.Activities{BriefsDir: dir}

	brief := research.CompanyBrief{
		Company:  "Scale AI",
		OneLiner: "data labeling for ML",
		FitScore: 2,
		ValuesFlags: []string{
			"military contracts",
		},
		Sources: []string{"https://example.com/scale"},
	}

	path, err := a.WriteBrief(context.Background(), brief)
	require.NoError(t, err)
	require.Equal(t, filepath.Join(dir, "scale-ai.md"), path)

	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	md := string(raw)
	require.Contains(t, md, "Scale AI")
	require.Contains(t, md, "data labeling for ML")
	require.Contains(t, md, "military contracts")
	require.Contains(t, md, "https://example.com/scale", "briefs must cite sources (FR-6.2)")
}
