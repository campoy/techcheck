package research_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/campoy/techcheck/internal/research"
)

// FR-1.2: company names map to stable identifiers.
func TestNormalize(t *testing.T) {
	for in, want := range map[string]string{
		"Scale AI":       "scale-ai",
		"  PaperCompute": "papercompute",
		"Resolve AI":     "resolve-ai",
		"O'Reilly Media": "oreilly-media",
		"TribeROI":       "triberoi",
	} {
		require.Equal(t, want, research.Normalize(in), "Normalize(%q)", in)
	}
}

// FR-3.4: findings without a source URL are invalid.
func TestFindingValidate(t *testing.T) {
	valid := research.Finding{
		SourceURL:  "https://example.com/news",
		Claim:      "raised a $5M seed round",
		Category:   research.CategoryFunding,
		Confidence: 0.8,
	}
	require.NoError(t, valid.Validate())

	noSource := valid
	noSource.SourceURL = ""
	require.Error(t, noSource.Validate(), "finding without source_url must be invalid")

	noClaim := valid
	noClaim.Claim = ""
	require.Error(t, noClaim.Validate(), "finding without a claim must be invalid")

	badCategory := valid
	badCategory.Category = "vibes"
	require.Error(t, badCategory.Validate(), "unknown category must be invalid")
}

// FR-6.1–FR-6.3: briefs carry a 1–10 fit score and cite sources.
func TestCompanyBriefValidate(t *testing.T) {
	valid := research.CompanyBrief{
		Company:  "acme",
		OneLiner: "roadrunner countermeasures",
		FitScore: 7,
		Sources:  []string{"https://example.com/about"},
	}
	require.NoError(t, valid.Validate())

	for _, score := range []int{0, -1, 11} {
		b := valid
		b.FitScore = score
		require.Error(t, b.Validate(), "fit score %d must be invalid", score)
	}

	noSources := valid
	noSources.Sources = nil
	require.Error(t, noSources.Validate(), "brief without sources must be invalid")
}
