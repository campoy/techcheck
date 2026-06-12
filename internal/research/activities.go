package research

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
	prompt := fmt.Sprintf(`You are planning research to evaluate the company %q for a job search.

Produce a list of specific, answerable research questions. The plan must cover all of:
- funding history and investors
- founders and team background
- product and differentiation
- competitors and commoditization risk
- hiring signals for Staff/Principal individual-contributor roles

Respond with a JSON array of question strings.`, req.Company)

	var plan []string
	if err := a.LLM.Generate(ctx, llm.TierReason, prompt, &plan); err != nil {
		return nil, fmt.Errorf("planning research: %w", err)
	}
	if len(plan) == 0 {
		return nil, fmt.Errorf("empty research plan for %q", req.Company)
	}
	return plan, nil
}

// WebResearchRequest carries the open questions and the search budget.
type WebResearchRequest struct {
	Questions   []string
	MaxSearches int // fixed budget in M2; per-iteration cap in M4 (FR-5.4)
}

// WebResearch runs searches against the open questions, capped at
// MaxSearches total (FR-3.1).
func (a *Activities) WebResearch(ctx context.Context, req WebResearchRequest) ([]search.Result, error) {
	const resultsPerSearch = 5

	var out []search.Result
	seen := map[string]bool{}
	for i, q := range req.Questions {
		if i >= req.MaxSearches {
			break
		}
		results, err := a.Searcher.Search(ctx, q, resultsPerSearch)
		if err != nil {
			return nil, fmt.Errorf("searching %q: %w", q, err)
		}
		for _, r := range results {
			if !seen[r.URL] {
				seen[r.URL] = true
				out = append(out, r)
			}
		}
	}
	return out, nil
}

// ExtractRequest carries raw search results for finding extraction.
type ExtractRequest struct {
	Results []search.Result
}

// ExtractFindings parses results into Findings using the extract tier
// (FR-3.2, FR-9.4). Findings without a source URL are dropped (FR-3.4).
func (a *Activities) ExtractFindings(ctx context.Context, req ExtractRequest) ([]Finding, error) {
	raw, err := json.MarshalIndent(req.Results, "", "  ")
	if err != nil {
		return nil, err
	}
	prompt := fmt.Sprintf(`Extract factual findings from these search results.

Each finding needs: source_url (the result URL the claim comes from), claim
(one specific fact), category (one of funding, team, product, market, risk),
and confidence (0-1, lower for weakly supported claims).

Search results:
%s

Respond with a JSON array of findings.`, raw)

	var findings []Finding
	if err := a.LLM.Generate(ctx, llm.TierExtract, prompt, &findings); err != nil {
		return nil, fmt.Errorf("extracting findings: %w", err)
	}

	valid := findings[:0]
	for _, f := range findings {
		if f.Validate() == nil { // drop unsourced/invalid findings (FR-3.4)
			valid = append(valid, f)
		}
	}
	return valid, nil
}

// BriefRequest carries everything brief generation needs.
type BriefRequest struct {
	Company  string
	Findings []Finding
}

// GenerateBrief produces the validated CompanyBrief using the reason tier
// (FR-6.1–FR-6.3, FR-9.4).
func (a *Activities) GenerateBrief(ctx context.Context, req BriefRequest) (CompanyBrief, error) {
	raw, err := json.MarshalIndent(req.Findings, "", "  ")
	if err != nil {
		return CompanyBrief{}, err
	}
	prompt := fmt.Sprintf(`Write a structured evaluation brief for the company %q based on these findings.

Rules:
- fit_score is 1-10: 1-3 rule out, 4-6 worth a conversation, 7-10 strong fit.
- Every claim must be supported by the findings; list every source_url you used in sources.
- values_flags lists ethical concerns (e.g. defense contracts, surveillance).
- questions_to_ask are for the user to ask the company directly.
- Leave comparable_precedents empty; precedent retrieval is not available yet.

Findings:
%s

Respond with a single JSON object matching the brief structure.`, req.Company, raw)

	var brief CompanyBrief
	if err := a.LLM.Generate(ctx, llm.TierReason, prompt, &brief); err != nil {
		return CompanyBrief{}, fmt.Errorf("generating brief: %w", err)
	}
	brief.Company = req.Company
	if err := brief.Validate(); err != nil {
		return CompanyBrief{}, fmt.Errorf("model produced invalid brief: %w", err)
	}
	return brief, nil
}

// WriteBrief renders the brief as Markdown to
// <BriefsDir>/<normalized-company>.md and returns the path (FR-8.2).
func (a *Activities) WriteBrief(ctx context.Context, brief CompanyBrief) (string, error) {
	if err := os.MkdirAll(a.BriefsDir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(a.BriefsDir, Normalize(brief.Company)+".md")
	if err := os.WriteFile(path, []byte(renderBrief(brief)), 0o644); err != nil {
		return "", err
	}
	return path, nil
}

func renderBrief(b CompanyBrief) string {
	var md strings.Builder
	p := func(format string, args ...any) { fmt.Fprintf(&md, format, args...) }
	list := func(header string, items []string) {
		if len(items) == 0 {
			return
		}
		p("\n## %s\n\n", header)
		for _, it := range items {
			p("- %s\n", it)
		}
	}

	p("# %s\n\n%s\n", b.Company, b.OneLiner)
	p("\n**Fit score: %d/10** — %s\n", b.FitScore, b.FitRationale)

	p("\n## Funding\n\n- Total raised: %s\n- Last round: %s\n", b.Funding.TotalRaised, b.Funding.LastRound)
	if len(b.Funding.Investors) > 0 {
		p("- Investors: %s\n", strings.Join(b.Funding.Investors, ", "))
	}

	p("\n## Team\n\n")
	if len(b.Team.Founders) > 0 {
		p("- Founders: %s\n", strings.Join(b.Team.Founders, ", "))
	}
	if b.Team.Notes != "" {
		p("- %s\n", b.Team.Notes)
	}

	if b.ProductAssessment != "" {
		p("\n## Product\n\n%s\n", b.ProductAssessment)
	}
	if b.StageAssessment != "" {
		p("\n## Stage\n\n%s\n", b.StageAssessment)
	}

	list("Values flags", b.ValuesFlags)
	if len(b.Risks) > 0 {
		p("\n## Risks\n\n")
		for _, r := range b.Risks {
			p("- **%s**: %s\n", r.Severity, r.Description)
		}
	}
	list("Questions to ask", b.QuestionsToAsk)
	list("Comparable precedents", b.ComparablePrecedents)
	list("Sources", b.Sources)
	return md.String()
}
