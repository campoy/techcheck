package research_test

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/campoy/techcheck/internal/corpus"
	"github.com/campoy/techcheck/internal/llm"
	"github.com/campoy/techcheck/internal/research"
)

// fakeCorpus records retrieval and indexing calls and answers with canned
// excerpts.
type fakeCorpus struct {
	queries  [][]string
	k        []int
	excerpts []corpus.Excerpt
	indexed  map[string]string
}

func (f *fakeCorpus) Retrieve(ctx context.Context, queries []string, k int) ([]corpus.Excerpt, error) {
	f.queries = append(f.queries, queries)
	f.k = append(f.k, k)
	return f.excerpts, nil
}

func (f *fakeCorpus) IndexDocument(ctx context.Context, docPath, content string) (int, error) {
	if f.indexed == nil {
		f.indexed = map[string]string{}
	}
	f.indexed[docPath] = content
	return 1, nil
}

// FR-4.2, FR-4.3: corpus search uses the company plus the product, market,
// and risk findings as retrieval cues, and returns the precedents it finds.
func TestCorpusSearchUsesCues(t *testing.T) {
	fake := &fakeCorpus{excerpts: []corpus.Excerpt{
		{DocPath: "briefs/scale-ai.md", Section: "Risks", Content: "ruled out on values grounds"},
	}}
	a := &research.Activities{Corpus: fake}

	got, err := a.CorpusSearch(context.Background(), research.CorpusSearchRequest{
		Company: "acme defense",
		Findings: []research.Finding{
			{SourceURL: "https://example.com/p", Claim: "sells a data labeling platform", Category: research.CategoryProduct, Confidence: 0.9},
			{SourceURL: "https://example.com/r", Claim: "pursuing military contracts", Category: research.CategoryRisk, Confidence: 0.8},
			{SourceURL: "https://example.com/f", Claim: "raised $5M", Category: research.CategoryFunding, Confidence: 0.9},
		},
	})
	require.NoError(t, err)
	require.Equal(t, fake.excerpts, got)

	require.Len(t, fake.queries, 1, "one retrieval per corpus search")
	joined := strings.ToLower(strings.Join(fake.queries[0], " "))
	require.Contains(t, joined, "acme defense", "the company is a retrieval cue (FR-4.2)")
	require.Contains(t, joined, "data labeling platform", "product findings are retrieval cues (FR-4.2)")
	require.Contains(t, joined, "military contracts", "risk findings are retrieval cues (FR-4.2)")
	require.Equal(t, research.DefaultPrecedents, fake.k[0])
}

// Without a corpus configured the activity degrades to no excerpts instead
// of failing the run: an empty corpus is a normal early state.
func TestCorpusSearchWithoutCorpus(t *testing.T) {
	a := &research.Activities{}
	got, err := a.CorpusSearch(context.Background(), research.CorpusSearchRequest{Company: "acme"})
	require.NoError(t, err)
	require.Empty(t, got)
}

// FR-6.1 (completed in M3): brief generation receives corpus excerpts and
// produces comparable precedents grounded in them.
func TestGenerateBriefCitesPrecedents(t *testing.T) {
	brief := research.CompanyBrief{
		Company:              "acme",
		OneLiner:             "data labeling for defense",
		FitScore:             3,
		FitRationale:         "values precedent applies",
		ComparablePrecedents: []string{"similar values concern as Scale AI (briefs/scale-ai.md)"},
		Sources:              []string{"https://example.com/a"},
	}
	fake := newFakeLLM(func(tier llm.Tier, prompt string, out any) error {
		fill(t, out, brief)
		return nil
	})
	a := &research.Activities{LLM: fake}

	got, err := a.GenerateBrief(context.Background(), research.BriefRequest{
		Company: "acme",
		Findings: []research.Finding{
			{SourceURL: "https://example.com/a", Claim: "pursuing military contracts", Category: research.CategoryRisk, Confidence: 0.8},
		},
		Excerpts: []corpus.Excerpt{{
			DocPath: "briefs/scale-ai.md",
			Section: "Risks",
			Content: "Scale AI ruled out on values grounds: military and surveillance contracts",
		}},
	})
	require.NoError(t, err)
	require.NotEmpty(t, got.ComparablePrecedents, "briefs cite comparable precedents (FR-6.1)")

	require.Len(t, fake.prompts[llm.TierReason], 1)
	prompt := fake.prompts[llm.TierReason][0]
	require.Contains(t, prompt, "Scale AI ruled out on values grounds",
		"corpus excerpts must reach the brief prompt")
	require.Contains(t, strings.ToLower(prompt), "precedent",
		"the prompt instructs grounding comparable_precedents in the excerpts")
}

// FR-8.3: the rendered brief is indexed into the corpus under
// briefs/<slug>.md so future runs retrieve it as a precedent.
func TestIndexBrief(t *testing.T) {
	fake := &fakeCorpus{}
	a := &research.Activities{Corpus: fake}

	n, err := a.IndexBrief(context.Background(), research.CompanyBrief{
		Company:     "Scale AI",
		OneLiner:    "data labeling for ML",
		FitScore:    2,
		ValuesFlags: []string{"military contracts"},
		Sources:     []string{"https://example.com/scale"},
	})
	require.NoError(t, err)
	require.Positive(t, n)

	content, ok := fake.indexed["briefs/scale-ai.md"]
	require.True(t, ok, "briefs index under briefs/<normalized-company>.md, got %v", fake.indexed)
	require.Contains(t, content, "Scale AI")
	require.Contains(t, content, "military contracts")
}
