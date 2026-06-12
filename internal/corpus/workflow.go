package corpus

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

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
	Chunks    int `json:"chunks"` // newly indexed; 0 when the corpus is unchanged
}

// Activities holds the ingestion activities' dependencies.
type Activities struct {
	Corpus Indexer
}

// corpusExtensions are the document types ingestion picks up.
var corpusExtensions = map[string]bool{
	".md": true, ".markdown": true, ".txt": true, ".pdf": true,
}

// ListDocuments returns the corpus documents under dir as paths relative to
// it: Markdown, plain text, and PDF files (FR-4.1).
func (a *Activities) ListDocuments(ctx context.Context, dir string) ([]string, error) {
	var out []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !corpusExtensions[strings.ToLower(filepath.Ext(path))] {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		out = append(out, rel)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walking corpus dir %s: %w", dir, err)
	}
	sort.Strings(out)
	return out, nil
}

// IngestRequest identifies one document to ingest.
type IngestRequest struct {
	Dir  string
	Path string // relative to Dir
}

// IngestDocument reads one document (extracting text from PDFs), splits it,
// and indexes its chunks, returning the newly indexed chunk count.
func (a *Activities) IngestDocument(ctx context.Context, req IngestRequest) (int, error) {
	full := filepath.Join(req.Dir, req.Path)

	var content string
	if strings.EqualFold(filepath.Ext(full), ".pdf") {
		text, err := ExtractPDF(full)
		if err != nil {
			return 0, err
		}
		content = text
	} else {
		raw, err := os.ReadFile(full)
		if err != nil {
			return 0, fmt.Errorf("reading %s: %w", full, err)
		}
		content = string(raw)
	}

	return a.Corpus.IndexDocument(ctx, req.Path, content)
}

// IngestCorpus walks the corpus directory and ingests every document.
// Content-hash keyed upserts make re-runs idempotent, so it can be
// triggered freely whenever the corpus changes.
func IngestCorpus(ctx workflow.Context, in IngestInput) (IngestResult, error) {
	if in.Dir == "" {
		in.Dir = DefaultDir
	}
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 5 * time.Minute,
	})

	var a *Activities // method references only; the worker provides the instance

	var docs []string
	if err := workflow.ExecuteActivity(ctx, a.ListDocuments, in.Dir).Get(ctx, &docs); err != nil {
		return IngestResult{}, err
	}

	result := IngestResult{Documents: len(docs)}
	for _, doc := range docs {
		var chunks int
		if err := workflow.ExecuteActivity(ctx, a.IngestDocument, IngestRequest{
			Dir:  in.Dir,
			Path: doc,
		}).Get(ctx, &chunks); err != nil {
			return IngestResult{}, err
		}
		result.Chunks += chunks
	}
	return result, nil
}
