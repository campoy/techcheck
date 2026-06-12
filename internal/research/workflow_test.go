package research_test

import (
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/testsuite"

	"github.com/campoy/techcheck/internal/corpus"
	"github.com/campoy/techcheck/internal/research"
	"github.com/campoy/techcheck/internal/search"
)

// The M3 linear workflow: plan -> search -> extract -> corpus -> brief ->
// write -> index, in that order, with the default search budget applied
// when none is given. M3 inserts corpus retrieval before brief generation
// (FR-4.2) and brief indexing after persistence (FR-8.3).
func TestCompanyResearchLinear(t *testing.T) {
	var ts testsuite.WorkflowTestSuite
	env := ts.NewTestWorkflowEnvironment()

	a := &research.Activities{}
	env.RegisterActivity(a.PlanResearch)
	env.RegisterActivity(a.WebResearch)
	env.RegisterActivity(a.ExtractFindings)
	env.RegisterActivity(a.CorpusSearch)
	env.RegisterActivity(a.GenerateBrief)
	env.RegisterActivity(a.WriteBrief)
	env.RegisterActivity(a.IndexBrief)

	plan := []string{"funding?", "founders?"}
	results := []search.Result{{URL: "https://example.com/a", Title: "A", Content: "alpha"}}
	findings := []research.Finding{
		{SourceURL: "https://example.com/a", Claim: "raised $5M", Category: research.CategoryFunding, Confidence: 0.9},
	}
	excerpts := []corpus.Excerpt{
		{DocPath: "briefs/scale-ai.md", Section: "Risks", Content: "values rule-out precedent"},
	}
	brief := research.CompanyBrief{
		Company:              "acme",
		OneLiner:             "roadrunner countermeasures",
		FitScore:             6,
		ComparablePrecedents: []string{"similar values concern as Scale AI"},
		Sources:              []string{"https://example.com/a"},
	}

	var order []string
	env.OnActivity(a.PlanResearch, mock.Anything, research.PlanRequest{Company: "acme"}).
		Run(func(mock.Arguments) { order = append(order, "plan") }).
		Return(plan, nil).Once()
	env.OnActivity(a.WebResearch, mock.Anything, research.WebResearchRequest{
		Questions:   plan,
		MaxSearches: research.DefaultMaxSearches,
	}).
		Run(func(mock.Arguments) { order = append(order, "search") }).
		Return(results, nil).Once()
	env.OnActivity(a.ExtractFindings, mock.Anything, research.ExtractRequest{Results: results}).
		Run(func(mock.Arguments) { order = append(order, "extract") }).
		Return(findings, nil).Once()
	env.OnActivity(a.CorpusSearch, mock.Anything, research.CorpusSearchRequest{Company: "acme", Findings: findings}).
		Run(func(mock.Arguments) { order = append(order, "corpus") }).
		Return(excerpts, nil).Once()
	env.OnActivity(a.GenerateBrief, mock.Anything, research.BriefRequest{Company: "acme", Findings: findings, Excerpts: excerpts}).
		Run(func(mock.Arguments) { order = append(order, "brief") }).
		Return(brief, nil).Once()
	env.OnActivity(a.WriteBrief, mock.Anything, brief).
		Run(func(mock.Arguments) { order = append(order, "write") }).
		Return("briefs/acme.md", nil).Once()
	env.OnActivity(a.IndexBrief, mock.Anything, brief).
		Run(func(mock.Arguments) { order = append(order, "index") }).
		Return(3, nil).Once()

	env.ExecuteWorkflow(research.CompanyResearch, research.ResearchInput{Company: "acme"})

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var got research.CompanyBrief
	require.NoError(t, env.GetWorkflowResult(&got))
	require.Equal(t, brief, got)

	require.Equal(t, []string{"plan", "search", "extract", "corpus", "brief", "write", "index"}, order,
		"M3 pipeline must run in order")
	env.AssertExpectations(t)
}

// An explicit search budget overrides the default.
func TestCompanyResearchCustomBudget(t *testing.T) {
	var ts testsuite.WorkflowTestSuite
	env := ts.NewTestWorkflowEnvironment()

	a := &research.Activities{}
	env.RegisterActivity(a.PlanResearch)
	env.RegisterActivity(a.WebResearch)
	env.RegisterActivity(a.ExtractFindings)
	env.RegisterActivity(a.CorpusSearch)
	env.RegisterActivity(a.GenerateBrief)
	env.RegisterActivity(a.WriteBrief)
	env.RegisterActivity(a.IndexBrief)

	env.OnActivity(a.PlanResearch, mock.Anything, mock.Anything).Return([]string{"q"}, nil)
	env.OnActivity(a.WebResearch, mock.Anything, mock.MatchedBy(func(req research.WebResearchRequest) bool {
		return req.MaxSearches == 2
	})).Return([]search.Result{{URL: "https://example.com/a"}}, nil)
	env.OnActivity(a.ExtractFindings, mock.Anything, mock.Anything).
		Return([]research.Finding{{SourceURL: "https://example.com/a", Claim: "c", Category: research.CategoryFunding, Confidence: 1}}, nil)
	env.OnActivity(a.CorpusSearch, mock.Anything, mock.Anything).Return([]corpus.Excerpt{}, nil)
	env.OnActivity(a.GenerateBrief, mock.Anything, mock.Anything).
		Return(research.CompanyBrief{Company: "acme", FitScore: 5, Sources: []string{"https://example.com/a"}}, nil)
	env.OnActivity(a.WriteBrief, mock.Anything, mock.Anything).Return("briefs/acme.md", nil)
	env.OnActivity(a.IndexBrief, mock.Anything, mock.Anything).Return(1, nil)

	env.ExecuteWorkflow(research.CompanyResearch, research.ResearchInput{Company: "acme", MaxSearches: 2})

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())
	env.AssertExpectations(t)
}
