package search_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/campoy/techcheck/internal/search"
)

// The Tavily client sends an authenticated search request and parses
// results into the provider-agnostic Result type. The fake server stands in
// for Tavily, so this stays hermetic: it tests our request/response
// handling, not Tavily itself (that's the live suite's job).
func TestTavilySearch(t *testing.T) {
	var gotMethod, gotAuth, gotQuery string
	var gotMax int
	var decodeErr error
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotAuth = r.Header.Get("Authorization")

		var body struct {
			Query      string `json:"query"`
			MaxResults int    `json:"max_results"`
		}
		decodeErr = json.NewDecoder(r.Body).Decode(&body)
		gotQuery, gotMax = body.Query, body.MaxResults

		_ = json.NewEncoder(w).Encode(map[string]any{
			"results": []map[string]any{
				{"url": "https://example.com/a", "title": "A", "content": "alpha"},
				{"url": "https://example.com/b", "title": "B", "content": "beta"},
			},
		})
	}))
	defer srv.Close()

	tavily := &search.Tavily{APIKey: "tvly-test", BaseURL: srv.URL}
	results, err := tavily.Search(t.Context(), "acme funding", 2)
	require.NoError(t, err)

	require.Equal(t, http.MethodPost, gotMethod)
	require.NoError(t, decodeErr)
	require.Equal(t, "Bearer tvly-test", gotAuth)
	require.Equal(t, "acme funding", gotQuery)
	require.Equal(t, 2, gotMax)

	require.Equal(t, []search.Result{
		{URL: "https://example.com/a", Title: "A", Content: "alpha"},
		{URL: "https://example.com/b", Title: "B", Content: "beta"},
	}, results)
}

// Non-200 responses surface as errors, not empty results.
func TestTavilySearchError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "rate limited", http.StatusTooManyRequests)
	}))
	defer srv.Close()

	tavily := &search.Tavily{APIKey: "tvly-test", BaseURL: srv.URL}
	_, err := tavily.Search(t.Context(), "acme", 1)
	require.Error(t, err)
	require.ErrorContains(t, err, "429")
}

// FetchPage extracts readable text: article content stays, scripts and
// navigation boilerplate go (FR-3.1).
func TestFetchPage(t *testing.T) {
	const page = `<!DOCTYPE html><html><head>
		<title>About Acme</title>
		<script>console.log("tracking-script-noise")</script>
		<style>.nav { color: red }</style>
	</head><body>
		<nav><a href="/">Home</a><a href="/pricing">Pricing</a></nav>
		<article>
			<h1>About Acme</h1>
			<p>Acme builds roadrunner countermeasures for enterprise customers.</p>
			<p>Founded in 2024, Acme raised a $5M seed round led by XYZ Ventures.</p>
		</article>
		<footer>© Acme. Cookie policy. Terms of service.</footer>
	</body></html>`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(page))
	}))
	defer srv.Close()

	text, err := search.FetchPage(t.Context(), srv.Client(), srv.URL)
	require.NoError(t, err)

	require.Contains(t, text, "roadrunner countermeasures")
	require.Contains(t, text, "$5M seed round")
	require.NotContains(t, text, "tracking-script-noise", "script content must be stripped")
	require.NotContains(t, text, ".nav { color: red }", "style content must be stripped")
}
