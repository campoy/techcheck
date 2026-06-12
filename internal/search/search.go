// Package search provides web search (Tavily) and page fetching with
// readability extraction (FR-3.1).
package search

import (
	"context"
	"errors"
	"net/http"
)

// Result is one web search hit.
type Result struct {
	URL     string `json:"url"`
	Title   string `json:"title"`
	Content string `json:"content"`
}

// Searcher runs a web search, returning at most max results.
type Searcher interface {
	Search(ctx context.Context, query string, max int) ([]Result, error)
}

// Tavily is a Searcher backed by the Tavily search API.
type Tavily struct {
	APIKey  string
	BaseURL string // defaults to the public API; tests override it
	HTTP    *http.Client
}

// Search implements Searcher.
func (t *Tavily) Search(ctx context.Context, query string, max int) ([]Result, error) {
	return nil, errors.New("not implemented")
}

// FetchPage retrieves a URL and returns its readable text content, stripping
// navigation, scripts, and boilerplate.
func FetchPage(ctx context.Context, httpc *http.Client, url string) (string, error) {
	return "", errors.New("not implemented")
}
