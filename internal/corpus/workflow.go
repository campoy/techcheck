package corpus

import (
	"context"

	"go.temporal.io/sdk/workflow"
)

// DefaultDir is where the corpus lives when no directory is given; it is
// gitignored — the repo is public and the corpus is personal.
const DefaultDir = "corpus"

// IngestInput starts a corpus ingestion run.
type IngestInput struct {
	Dir string // defaults to DefaultDir
}

// IngestResult summarizes an ingestion run.
type IngestResult struct {
	Documents int `json:"documents"`
	Chunks    int `json:"chunks"`
}

// Activities holds the ingestion activities' dependencies.
type Activities struct {
	Corpus Indexer
}

// ListDocuments returns the corpus documents under dir as paths relative to
// it: Markdown, plain text, and PDF files (FR-4.1).
func (a *Activities) ListDocuments(ctx context.Context, dir string) ([]string, error) {
	return nil, errNotImplemented
}

// IngestRequest identifies one document to ingest.
type IngestRequest struct {
	Dir  string
	Path string // relative to Dir
}

// IngestDocument reads one document (extracting text from PDFs), splits it,
// and indexes its chunks, returning the chunk count.
func (a *Activities) IngestDocument(ctx context.Context, req IngestRequest) (int, error) {
	return 0, errNotImplemented
}

// IngestCorpus walks the corpus directory and ingests every document.
// Content-hash keyed upserts make re-runs idempotent, so it can be
// triggered freely whenever the corpus changes.
func IngestCorpus(ctx workflow.Context, in IngestInput) (IngestResult, error) {
	return IngestResult{}, errNotImplemented
}
