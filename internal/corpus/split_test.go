package corpus_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/campoy/techcheck/internal/corpus"
)

// FR-4.5: header-aware splitting keeps logically coherent sections — like a
// brief's risks — together in one chunk.
func TestSplitMarkdownKeepsSectionsTogether(t *testing.T) {
	doc := `# Scale AI

Data labeling for ML; ruled out.

## Risks

- military contracts
- surveillance work
- heavy services revenue

## Team

Founded by a strong technical team.
`
	chunks := corpus.SplitMarkdown("briefs/scale-ai.md", doc)
	require.Len(t, chunks, 3)
	for _, c := range chunks {
		require.Equal(t, "briefs/scale-ai.md", c.DocPath)
		require.NotEmpty(t, c.Content)
	}

	var risks *corpus.Chunk
	for i := range chunks {
		if strings.Contains(chunks[i].Section, "Risks") {
			risks = &chunks[i]
		}
	}
	require.NotNil(t, risks, "the Risks section must be its own chunk")
	require.Equal(t, "Scale AI > Risks", risks.Section,
		"sections carry their heading trail")
	for _, bullet := range []string{"military contracts", "surveillance work", "heavy services revenue"} {
		require.Contains(t, risks.Content, bullet,
			"all risk bullets must stay in the same chunk (FR-4.5)")
	}

	require.Equal(t, "Scale AI", chunks[0].Section)
	require.Contains(t, chunks[0].Content, "Data labeling for ML")
}

// Content before any heading forms its own chunk instead of being dropped.
func TestSplitMarkdownPreamble(t *testing.T) {
	chunks := corpus.SplitMarkdown("notes.md", "just some notes\nwith no headings\n")
	require.Len(t, chunks, 1)
	require.Empty(t, chunks[0].Section)
	require.Contains(t, chunks[0].Content, "just some notes")
}

// Oversized sections split on paragraph boundaries, each piece keeping the
// heading so chunks stay self-describing.
func TestSplitMarkdownOversizedSection(t *testing.T) {
	var sb strings.Builder
	sb.WriteString("## Notes\n\n")
	for i := range 8 {
		fmt.Fprintf(&sb, "Paragraph %d: %s.\n\n", i, strings.Repeat("lorem ipsum ", 40))
	}
	chunks := corpus.SplitMarkdown("notes.md", sb.String())

	require.Greater(t, len(chunks), 1, "a section longer than MaxChunkLen must split")
	for _, c := range chunks {
		require.Equal(t, "Notes", c.Section)
		require.Contains(t, c.Content, "## Notes", "each piece repeats its heading")
		require.LessOrEqual(t, len(c.Content), corpus.MaxChunkLen+200,
			"chunks stay near the soft cap")
	}

	// Nothing is lost: every paragraph appears in some chunk.
	var joined strings.Builder
	for _, c := range chunks {
		joined.WriteString(c.Content)
	}
	all := joined.String()
	for i := range 8 {
		require.Contains(t, all, fmt.Sprintf("Paragraph %d:", i))
	}
}

// Empty documents produce no chunks.
func TestSplitMarkdownEmpty(t *testing.T) {
	require.Empty(t, corpus.SplitMarkdown("empty.md", "  \n\n "))
}

// Content-hash identity: identical chunks hash equal (idempotent upserts),
// any changed field hashes different.
func TestChunkHash(t *testing.T) {
	c := corpus.Chunk{DocPath: "resume.md", Section: "Experience", Content: "ten years of Go"}
	require.Equal(t, c.Hash(), corpus.Chunk{DocPath: "resume.md", Section: "Experience", Content: "ten years of Go"}.Hash())

	for _, other := range []corpus.Chunk{
		{DocPath: "resume2.md", Section: "Experience", Content: "ten years of Go"},
		{DocPath: "resume.md", Section: "Education", Content: "ten years of Go"},
		{DocPath: "resume.md", Section: "Experience", Content: "ten years of Rust"},
	} {
		require.NotEqual(t, c.Hash(), other.Hash())
	}
}
