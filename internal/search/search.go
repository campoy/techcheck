// Package search provides web search (Tavily) and page fetching with
// readability extraction (FR-3.1).
package search

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"golang.org/x/net/html"
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

const defaultTavilyURL = "https://api.tavily.com"

// Tavily is a Searcher backed by the Tavily search API.
type Tavily struct {
	APIKey  string
	BaseURL string // defaults to the public API; tests override it
	HTTP    *http.Client
}

// Search implements Searcher.
func (t *Tavily) Search(ctx context.Context, query string, max int) ([]Result, error) {
	base := t.BaseURL
	if base == "" {
		base = defaultTavilyURL
	}
	httpc := t.HTTP
	if httpc == nil {
		httpc = http.DefaultClient
	}

	body, err := json.Marshal(map[string]any{
		"query":       query,
		"max_results": max,
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/search", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+t.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("tavily: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("tavily: unexpected status %s", resp.Status)
	}

	var parsed struct {
		Results []Result `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("tavily: decoding response: %w", err)
	}
	if len(parsed.Results) > max {
		parsed.Results = parsed.Results[:max]
	}
	return parsed.Results, nil
}

// skippedElements never contribute readable text.
var skippedElements = map[string]bool{
	"script": true, "style": true, "noscript": true, "template": true,
	"nav": true, "header": true, "footer": true, "aside": true,
	"iframe": true, "svg": true, "form": true, "button": true,
}

// FetchPage retrieves a URL and returns its readable text content, stripping
// navigation, scripts, and boilerplate.
func FetchPage(ctx context.Context, httpc *http.Client, url string) (string, error) {
	if httpc == nil {
		httpc = http.DefaultClient
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := httpc.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetching %s: %w", url, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("fetching %s: unexpected status %s", url, resp.Status)
	}

	root, err := html.Parse(io.LimitReader(resp.Body, 2<<20)) // 2MB cap
	if err != nil {
		return "", fmt.Errorf("parsing %s: %w", url, err)
	}

	var lines []string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && skippedElements[n.Data] {
			return
		}
		if n.Type == html.TextNode {
			if text := strings.TrimSpace(n.Data); text != "" {
				lines = append(lines, text)
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	return strings.Join(lines, "\n"), nil
}
