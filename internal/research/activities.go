package research

import (
	"context"

	"github.com/campoy/techcheck/internal/llm"
	"github.com/campoy/techcheck/internal/search"
)

// Activities holds the dependencies the research activities need. All
// non-determinism (LLM, search, fetch, disk) lives here, behind interfaces
// so tests run hermetically.
type Activities struct {
	LLM       llm.Client
	Searcher  search.Searcher
	BriefsDir string // where approved briefs land (FR-8.2)
}

// PlanRequest asks for a research plan for a company.
type PlanRequest struct {
	Company string
}

// PlanResearch produces the list of open questions (FR-2.1). The initial
// plan must cover funding, founders/team, product/differentiation,
// competitors/commoditization, and Staff/Principal IC hiring signals
// (FR-2.2). Uses the reason tier.
func (a *Activities) PlanResearch(ctx context.Context, req PlanRequest) ([]string, error) {
	return nil, errNotImplemented
}

// WebResearchRequest carries the open questions and the search budget.
type WebResearchRequest struct {
	Questions   []string
	MaxSearches int // fixed budget in M2; per-iteration cap in M4 (FR-5.4)
}

// WebResearch runs searches against the open questions, capped at
// MaxSearches total (FR-3.1).
func (a *Activities) WebResearch(ctx context.Context, req WebResearchRequest) ([]search.Result, error) {
	return nil, errNotImplemented
}

// ExtractRequest carries raw search results for finding extraction.
type ExtractRequest struct {
	Results []search.Result
}

// ExtractFindings parses results into Findings using the extract tier
// (FR-3.2, FR-9.4). Findings without a source URL are dropped (FR-3.4).
func (a *Activities) ExtractFindings(ctx context.Context, req ExtractRequest) ([]Finding, error) {
	return nil, errNotImplemented
}

// BriefRequest carries everything brief generation needs.
type BriefRequest struct {
	Company  string
	Findings []Finding
}

// GenerateBrief produces the validated CompanyBrief using the reason tier
// (FR-6.1–FR-6.3, FR-9.4).
func (a *Activities) GenerateBrief(ctx context.Context, req BriefRequest) (CompanyBrief, error) {
	return CompanyBrief{}, errNotImplemented
}

// WriteBrief renders the brief as Markdown to
// <BriefsDir>/<normalized-company>.md and returns the path (FR-8.2).
func (a *Activities) WriteBrief(ctx context.Context, brief CompanyBrief) (string, error) {
	return "", errNotImplemented
}
