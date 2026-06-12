package research

import (
	"time"

	"go.temporal.io/sdk/workflow"

	"github.com/campoy/techcheck/internal/search"
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
	if in.MaxSearches <= 0 {
		in.MaxSearches = DefaultMaxSearches
	}
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 2 * time.Minute,
	})

	var a *Activities // method references only; the worker provides the instance

	var plan []string
	if err := workflow.ExecuteActivity(ctx, a.PlanResearch, PlanRequest{Company: in.Company}).Get(ctx, &plan); err != nil {
		return CompanyBrief{}, err
	}

	var results []search.Result
	if err := workflow.ExecuteActivity(ctx, a.WebResearch, WebResearchRequest{
		Questions:   plan,
		MaxSearches: in.MaxSearches,
	}).Get(ctx, &results); err != nil {
		return CompanyBrief{}, err
	}

	var findings []Finding
	if err := workflow.ExecuteActivity(ctx, a.ExtractFindings, ExtractRequest{Results: results}).Get(ctx, &findings); err != nil {
		return CompanyBrief{}, err
	}

	var brief CompanyBrief
	if err := workflow.ExecuteActivity(ctx, a.GenerateBrief, BriefRequest{
		Company:  in.Company,
		Findings: findings,
	}).Get(ctx, &brief); err != nil {
		return CompanyBrief{}, err
	}

	var path string
	if err := workflow.ExecuteActivity(ctx, a.WriteBrief, brief).Get(ctx, &path); err != nil {
		return CompanyBrief{}, err
	}

	return brief, nil
}
