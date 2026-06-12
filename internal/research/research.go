// Package research holds the domain types, activities, and workflow for
// company evaluation runs.
package research

import (
	"errors"
	"fmt"
	"strings"
	"unicode"
)

// Categories a Finding can have.
const (
	CategoryFunding = "funding"
	CategoryTeam    = "team"
	CategoryProduct = "product"
	CategoryMarket  = "market"
	CategoryRisk    = "risk"
)

// Finding is a single sourced claim about a company (FR-3.2).
type Finding struct {
	SourceURL  string  `json:"source_url"`
	Claim      string  `json:"claim"`
	Category   string  `json:"category"`
	Confidence float64 `json:"confidence"` // 0–1; low values flag weak sourcing
}

// Validate enforces FR-3.4: every finding needs a source URL, a claim, and a
// known category.
func (f Finding) Validate() error {
	if f.SourceURL == "" {
		return errors.New("finding missing source_url")
	}
	if f.Claim == "" {
		return errors.New("finding missing claim")
	}
	switch f.Category {
	case CategoryFunding, CategoryTeam, CategoryProduct, CategoryMarket, CategoryRisk:
		return nil
	default:
		return fmt.Errorf("finding has unknown category %q", f.Category)
	}
}

// FundingSummary summarizes funding history.
type FundingSummary struct {
	TotalRaised string   `json:"total_raised"`
	LastRound   string   `json:"last_round"`
	Investors   []string `json:"investors"`
}

// TeamSummary summarizes founders and team background.
type TeamSummary struct {
	Founders []string `json:"founders"`
	Notes    string   `json:"notes"`
}

// Risk is one identified risk.
type Risk struct {
	Description string `json:"description"`
	Severity    string `json:"severity"`
}

// CompanyBrief is the structured evaluation output (FR-6.1).
type CompanyBrief struct {
	Company              string         `json:"company"`
	OneLiner             string         `json:"one_liner"`
	Funding              FundingSummary `json:"funding"`
	Team                 TeamSummary    `json:"team"`
	ProductAssessment    string         `json:"product_assessment"`
	FitScore             int            `json:"fit_score"` // 1–10
	FitRationale         string         `json:"fit_rationale"`
	ValuesFlags          []string       `json:"values_flags"`
	StageAssessment      string         `json:"stage_assessment"`
	Risks                []Risk         `json:"risks"`
	QuestionsToAsk       []string       `json:"questions_to_ask"`
	ComparablePrecedents []string       `json:"comparable_precedents"` // empty until M3
	Sources              []string       `json:"sources"`
}

// Validate enforces FR-6.1–FR-6.3: fit score in 1–10 and at least one
// source cited.
func (b CompanyBrief) Validate() error {
	if b.FitScore < 1 || b.FitScore > 10 {
		return fmt.Errorf("fit score %d outside 1-10", b.FitScore)
	}
	if len(b.Sources) == 0 {
		return errors.New("brief cites no sources")
	}
	return nil
}

// Normalize maps a company name to its stable identifier (FR-1.2):
// lowercase, words joined by hyphens, everything else stripped.
func Normalize(company string) string {
	var b strings.Builder
	pendingHyphen := false
	for _, r := range strings.ToLower(strings.TrimSpace(company)) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			if pendingHyphen && b.Len() > 0 {
				b.WriteByte('-')
			}
			pendingHyphen = false
			b.WriteRune(r)
		case unicode.IsSpace(r) || r == '-' || r == '_' || r == '.':
			pendingHyphen = true
		}
	}
	return b.String()
}
