package research

import (
	"go.temporal.io/sdk/workflow"
)

// DefaultMaxSearches is the fixed search budget for an M2 linear run
// (FR-5.4 partial; becomes a per-iteration cap in M4).
const DefaultMaxSearches = 5

// ResearchInput starts a company research run (FR-1.1).
type ResearchInput struct {
	Company     string
	MaxSearches int // 0 means DefaultMaxSearches
}

// CompanyResearch is the M2 linear research workflow:
// PlanResearch -> WebResearch -> ExtractFindings -> GenerateBrief ->
// WriteBrief. No loop and no corpus yet; those arrive in M3/M4.
func CompanyResearch(ctx workflow.Context, in ResearchInput) (CompanyBrief, error) {
	return CompanyBrief{}, errNotImplemented
}
