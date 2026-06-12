package corpus_test

import (
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/testsuite"

	"github.com/campoy/techcheck/internal/corpus"
)

// IngestCorpus lists the corpus directory and ingests every document,
// reporting totals (FR-4.1).
func TestIngestCorpusWorkflow(t *testing.T) {
	var ts testsuite.WorkflowTestSuite
	env := ts.NewTestWorkflowEnvironment()

	a := &corpus.Activities{}
	env.RegisterActivity(a.ListDocuments)
	env.RegisterActivity(a.IngestDocument)

	env.OnActivity(a.ListDocuments, mock.Anything, corpus.DefaultDir).
		Return([]string{"resume.md", "briefs/scale-ai.md"}, nil).Once()
	env.OnActivity(a.IngestDocument, mock.Anything, corpus.IngestRequest{Dir: corpus.DefaultDir, Path: "resume.md"}).
		Return(3, nil).Once()
	env.OnActivity(a.IngestDocument, mock.Anything, corpus.IngestRequest{Dir: corpus.DefaultDir, Path: "briefs/scale-ai.md"}).
		Return(2, nil).Once()

	env.ExecuteWorkflow(corpus.IngestCorpus, corpus.IngestInput{}) // empty dir → default

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var got corpus.IngestResult
	require.NoError(t, env.GetWorkflowResult(&got))
	require.Equal(t, corpus.IngestResult{Documents: 2, Chunks: 5}, got)
	env.AssertExpectations(t)
}

// An explicit directory overrides the default.
func TestIngestCorpusCustomDir(t *testing.T) {
	var ts testsuite.WorkflowTestSuite
	env := ts.NewTestWorkflowEnvironment()

	a := &corpus.Activities{}
	env.RegisterActivity(a.ListDocuments)
	env.RegisterActivity(a.IngestDocument)

	env.OnActivity(a.ListDocuments, mock.Anything, "/data/corpus").
		Return([]string{"criteria.md"}, nil).Once()
	env.OnActivity(a.IngestDocument, mock.Anything, corpus.IngestRequest{Dir: "/data/corpus", Path: "criteria.md"}).
		Return(4, nil).Once()

	env.ExecuteWorkflow(corpus.IngestCorpus, corpus.IngestInput{Dir: "/data/corpus"})

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var got corpus.IngestResult
	require.NoError(t, env.GetWorkflowResult(&got))
	require.Equal(t, corpus.IngestResult{Documents: 1, Chunks: 4}, got)
	env.AssertExpectations(t)
}
