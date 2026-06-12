//go:build integration

package integration

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"

	"github.com/campoy/techcheck/internal/corpus"
	"github.com/campoy/techcheck/internal/server"
	"github.com/campoy/techcheck/internal/workflows"
)

// newTestCorpus connects a corpus.Corpus to the research database with the
// hermetic lexical embedder.
func newTestCorpus(t *testing.T) *corpus.Corpus {
	t.Helper()
	ctx, cancel := contextWithTimeout(t, 30*time.Second)
	defer cancel()

	store, err := corpus.NewStore(ctx, pgDSN())
	require.NoError(t, err)
	t.Cleanup(store.Close)
	require.NoError(t, store.Migrate(ctx))

	return &corpus.Corpus{Store: store, Embedder: corpus.LexicalEmbedder{}}
}

// uniqueDoc namespaces fixture paths per run so reruns against a persistent
// database stay meaningful.
func uniqueDoc(prefix, path string) string {
	return fmt.Sprintf("%s/%s", prefix, path)
}

// FR-4.1, FR-8.3 (indexing side): documents index into pgvector and
// re-indexing identical content is a no-op.
func TestCorpusIndexIdempotent(t *testing.T) {
	c := newTestCorpus(t)
	ctx, cancel := contextWithTimeout(t, 30*time.Second)
	defer cancel()

	prefix := fmt.Sprintf("it-%d", time.Now().UnixNano())
	doc := uniqueDoc(prefix, "resume.md")
	content := `# Resume

## Experience

Staff engineer: Go services, Kubernetes platforms, developer tooling.

## Interests

Durable execution, developer productivity, open source.
`
	n, err := c.IndexDocument(ctx, doc, content)
	require.NoError(t, err)
	require.Positive(t, n, "first ingestion indexes chunks")

	again, err := c.IndexDocument(ctx, doc, content)
	require.NoError(t, err)
	require.Zero(t, again, "re-ingesting unchanged content must be a no-op (idempotent upserts)")
}

// FR-4.2, FR-4.3: retrieval over a synthetic personal corpus surfaces the
// applicable precedent — a values rule-out for defense-adjacent companies,
// a stage-mismatch for very early companies.
func TestCorpusRetrievesPrecedents(t *testing.T) {
	c := newTestCorpus(t)
	ctx, cancel := contextWithTimeout(t, 60*time.Second)
	defer cancel()

	prefix := fmt.Sprintf("it-%d", time.Now().UnixNano())
	docs := map[string]string{
		"resume.md": "# Resume\n\nStaff engineer, Go and distributed systems.\n",
		"criteria.md": "# Criteria\n\n## Values\n\nNo defense or surveillance work.\n\n" +
			"## Stage\n\nSeries A or later, post product-market fit.\n",
		"briefs/scale-ai.md": "# Scale AI\n\n## Decision\n\nRuled out on values grounds: " +
			"military contracts, defense work, surveillance programs.\n",
		"briefs/tribeoroi.md": "# TribeROI\n\n## Decision\n\nRuled out as stage mismatch: " +
			"pre-seed, no revenue, extremely early stage company.\n",
	}
	for path, content := range docs {
		_, err := c.IndexDocument(ctx, uniqueDoc(prefix, path), content)
		require.NoError(t, err)
	}

	values, err := c.Retrieve(ctx, []string{"military defense contracts surveillance vendor"}, 3)
	require.NoError(t, err)
	require.NotEmpty(t, values)
	requireDocSurfaced(t, values, "scale-ai", "the values precedent must surface for defense cues (FR-4.3)")

	stage, err := c.Retrieve(ctx, []string{"pre-seed no revenue extremely early stage startup"}, 3)
	require.NoError(t, err)
	requireDocSurfaced(t, stage, "tribeoroi", "the stage precedent must surface for stage cues (FR-4.3)")
}

func requireDocSurfaced(t *testing.T, excerpts []corpus.Excerpt, fragment, msg string) {
	t.Helper()
	for _, e := range excerpts {
		if bytes.Contains([]byte(e.DocPath), []byte(fragment)) {
			return
		}
	}
	t.Fatalf("%s: %q not in retrieved docs %v", msg, fragment, excerpts)
}

// FR-4.4: near-duplicate sections must not crowd out a relevant excerpt
// from another document.
func TestCorpusRetrievalDiversity(t *testing.T) {
	c := newTestCorpus(t)
	ctx, cancel := contextWithTimeout(t, 60*time.Second)
	defer cancel()

	prefix := fmt.Sprintf("it-%d", time.Now().UnixNano())
	var dup strings.Builder
	dup.WriteString("# Notes\n")
	for i := range 5 {
		fmt.Fprintf(&dup, "\n## Take %d\n\nThe rust compiler toolchain is interesting.\n", i)
	}
	_, err := c.IndexDocument(ctx, uniqueDoc(prefix, "dup.md"), dup.String())
	require.NoError(t, err)
	_, err = c.IndexDocument(ctx, uniqueDoc(prefix, "other.md"),
		"# Other\n\nNotes on rust compiler toolchain performance and build speed.\n")
	require.NoError(t, err)

	got, err := c.Retrieve(ctx, []string{"rust compiler toolchain"}, 2)
	require.NoError(t, err)
	require.Len(t, got, 2)
	requireDocSurfaced(t, got, "other.md", "diverse retrieval must not return only near-duplicates (FR-4.4)")
}

// End to end: POST /corpus/ingest walks a fixture corpus (Markdown and
// PDF), the IngestCorpus workflow completes on a worker, and a re-run is
// idempotent (FR-4.1).
func TestCorpusIngestEndToEnd(t *testing.T) {
	cor := newTestCorpus(t)

	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "briefs"), 0o755))
	prefix := time.Now().UnixNano()
	files := map[string]string{
		"resume.md":           fmt.Sprintf("# Resume %d\n\nGo, Temporal, distributed systems.\n", prefix),
		"criteria.md":         fmt.Sprintf("# Criteria %d\n\n## Values\n\nNo surveillance work.\n", prefix),
		"briefs/scale-ai.md":  fmt.Sprintf("# Scale AI %d\n\nRuled out on values grounds.\n", prefix),
		"briefs/tribeoroi.md": fmt.Sprintf("# TribeROI %d\n\nRuled out as stage mismatch.\n", prefix),
	}
	for path, content := range files {
		require.NoError(t, os.WriteFile(filepath.Join(dir, path), []byte(content), 0o644))
	}
	pdf, err := os.ReadFile("../../internal/corpus/testdata/hello.pdf")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "notes.pdf"), pdf, 0o644))

	c, err := client.Dial(client.Options{HostPort: temporalHostPort()})
	require.NoError(t, err)
	defer c.Close()

	w := worker.New(c, workflows.TaskQueue, worker.Options{})
	w.RegisterWorkflow(corpus.IngestCorpus)
	w.RegisterActivity(&corpus.Activities{Corpus: cor})
	require.NoError(t, w.Start())
	defer w.Stop()

	api := httptest.NewServer(server.New(c))
	defer api.Close()

	ingest := func() corpus.IngestResult {
		t.Helper()
		req, err := http.NewRequestWithContext(t.Context(), http.MethodPost,
			api.URL+"/corpus/ingest", bytes.NewReader([]byte(fmt.Sprintf(`{"dir":%q}`, dir))))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()
		require.Equal(t, http.StatusAccepted, resp.StatusCode)

		var body struct {
			WorkflowID string `json:"workflow_id"`
			RunID      string `json:"run_id"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))

		ctx, cancel := contextWithTimeout(t, 60*time.Second)
		defer cancel()
		var result corpus.IngestResult
		require.NoError(t, c.GetWorkflow(ctx, body.WorkflowID, body.RunID).Get(ctx, &result))
		return result
	}

	first := ingest()
	require.Equal(t, 5, first.Documents, "all Markdown and PDF fixtures are ingested (FR-4.1)")
	require.Positive(t, first.Chunks)

	second := ingest()
	require.Equal(t, 5, second.Documents)
	require.Zero(t, second.Chunks, "re-ingesting an unchanged corpus indexes nothing new")
}

// The corpus schema lives in the research database (FR-8.3 storage side).
func TestCorpusSchema(t *testing.T) {
	newTestCorpus(t) // runs Migrate

	db, err := sql.Open("pgx", pgDSN())
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	ctx, cancel := contextWithTimeout(t, 10*time.Second)
	defer cancel()

	var n int
	require.NoError(t, db.QueryRowContext(ctx,
		"SELECT count(*) FROM information_schema.tables WHERE table_name = 'corpus_chunks'").Scan(&n))
	require.Equal(t, 1, n, "Migrate must create the corpus_chunks table")
}
