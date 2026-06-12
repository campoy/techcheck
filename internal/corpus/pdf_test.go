package corpus_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/campoy/techcheck/internal/corpus"
)

// FR-4.1: PDFs in the corpus (exported notes, documents) yield their text.
func TestExtractPDF(t *testing.T) {
	text, err := corpus.ExtractPDF("testdata/hello.pdf")
	require.NoError(t, err)
	require.Contains(t, text, "Hello corpus PDF")
}

func TestExtractPDFMissing(t *testing.T) {
	_, err := corpus.ExtractPDF("testdata/does-not-exist.pdf")
	require.Error(t, err)
}
