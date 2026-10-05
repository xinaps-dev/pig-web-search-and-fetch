package websearch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strings"
	"time"
)

// Exa REST endpoints. The Go port talks to Exa's REST API directly when an
// API key is available, and falls back to the public MCP endpoint
// (https://mcp.exa.ai/mcp, same server the TypeScript original uses for
// everything) when no key is resolved, preserving zero-config public mode
// for search and fetch. Deep research still requires a key, exactly as
// upstream.
var (
	exaBaseURL      = "https://api.exa.ai"
	exaAPIKeyHeader = "x-api-key"
)

const (
	exaSearchPath      = "/search"
	exaSimilarPath     = "/findSimilar"
	exaContentsPath    = "/contents"
	exaAnswerPath      = "/answer"
	defaultHTTPTimeout = 70 * time.Second
)

// Tunables, overridable via environment in tests.
const (
	searchTimeoutMs     = 15_000
	fetchTimeoutMs      = 20_000
	deepSearchTimeoutMs = 60_000
	maxRetries          = 3
)

// SearchResultItem is one normalized search hit.
type SearchResultItem struct {
	Title         string  `json:"title"`
	URL           string  `json:"url"`
	Snippet       string  `json:"snippet,omitempty"`
	PublishedDate string  `json:"publishedDate,omitempty"`
	Author        string  `json:"author,omitempty"`
	Score         float64 `json:"score,omitempty"`
}

// SearchResponse is the normalized search payload returned to tools.
type SearchResponse struct {
	Query    string             `json:"query"`
	Results  []SearchResultItem `json:"results"`
	Provider string             `json:"provider"`
}

// FetchResult is one normalized page.
type FetchResult struct {
	URL      string `json:"url"`
	Title    string `json:"title,omitempty"`
	Content  string `json:"content"`
	Provider string `json:"provider"`
}

// DeepSearchResult is one cited source.
type DeepSearchResult struct {
	Title         string   `json:"title"`
	URL           string   `json:"url"`
	Text          string   `json:"text,omitempty"`
	Highlights    []string `json:"highlights,omitempty"`
	PublishedDate string   `json:"publishedDate,omitempty"`
	Author        string   `json:"author,omitempty"`
}

// DeepSearchResponse is the normalized deep-research payload.
type DeepSearchResponse struct {
	Query              string             `json:"query"`
	Results            []DeepSearchResult `json:"results"`
	SubQueriesExecuted []string           `json:"subQueriesExecuted,omitempty"`
	Provider           string             `json:"provider"`
	Answer             string             `json:"answer,omitempty"`
}

// exaSearchHit is the REST shape of one hit.
type exaSearchHit struct {
	Title         string   `json:"title"`
	URL           string   `json:"url"`
	Text          string   `json:"text"`
	Highlights    []string `json:"highlights"`
	PublishedDate string   `json:"publishedDate"`
	Author        string   `json:"author"`
	Score         float64  `json:"score"`
}

// normalizeCategory maps the tool-level category to Exa's REST vocabulary.
func normalizeCategory(category string) string {
	switch strings.ToLower(strings.TrimSpace(category)) {
	case "research_paper", "research paper", "publication":
		return "research paper"
	case "personal_site", "personal site":
		return "personal site"
	case "financial_report", "financial report":
		return "financial report"
	default:
		return category
	}
}

// exaHTTPClient is shared across calls; per-request deadlines come from ctx.
var exaHTTPClient = &http.Client{Timeout: defaultHTTPTimeout}

// doExaPOST posts a JSON body with the API key and decodes the JSON reply,
// retrying transient failures with exponential backoff and jitter.
func doExaPOST(ctx context.Context, apiKey, path string, body any) ([]byte, error) {
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	url := strings.TrimSuffix(exaBaseURL, "/") + path
	var lastErr error
	delay := time.Second
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(raw))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set(exaAPIKeyHeader, apiKey)
		resp, err := exaHTTPClient.Do(req)
		if err != nil {
			lastErr = err
		} else {
			respBody, readErr := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
			resp.Body.Close()
			if readErr != nil {
				lastErr = readErr
			} else if resp.StatusCode == 429 || (resp.StatusCode >= 500 && resp.StatusCode <= 599) {
				lastErr = fmt.Errorf("Exa API error (HTTP %d): %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
			} else if resp.StatusCode < 200 || resp.StatusCode > 299 {
				return nil, fmt.Errorf("Exa API error (HTTP %d): %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
			} else {
				return respBody, nil
			}
		}
		if attempt == maxRetries {
			break
		}
		jitter := time.Duration(rand.Int63n(int64(delay) / 5))
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(delay + jitter):
		}
		delay *= 2
		if delay > 10*time.Second {
			delay = 10 * time.Second
		}
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("Exa request failed")
	}
	return nil, fmt.Errorf("%w", maskCredentialsError(lastErr))
}

func maskCredentialsError(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s", maskCredentials(err.Error()))
}

// requireAPIKey resolves the key or returns the guided error shown when the
// extension runs without one. The message points at /ws config and EXA_API_KEY.
func requireAPIKey() (string, error) {
	cfg := getConfig()
	key := getExaAPIKey(cfg.Providers.Exa.UseAPIKey)
	if key == nil || strings.TrimSpace(*key) == "" {
		if !cfg.Providers.Exa.UseAPIKey {
			return "", fmt.Errorf("Exa API key is disabled (useApiKey: false). Enable it with `/ws config exa` or `/ws config`, or store a key first")
		}
		return "", fmt.Errorf("no Exa API key found. Store one with `/ws config exa` (saved to auth.json, 0600) or set EXA_API_KEY")
	}
	return *key, nil
}

// withTimeout derives the per-request context from the caller's cancellation.
func withTimeout(parent context.Context, ms int) (context.Context, context.CancelFunc) {
	if parent == nil {
		parent = context.Background()
	}
	return context.WithTimeout(parent, time.Duration(ms)*time.Millisecond)
}

// exaSearch runs POST /search (or /findSimilar when similarURL is set).
func exaSearch(ctx context.Context, apiKey, query string, opts searchOptions) (SearchResponse, error) {
	timeoutCtx, cancel := withTimeout(ctx, searchTimeoutMs)
	defer cancel()
	if opts.similarURL != "" {
		return exaFindSimilar(timeoutCtx, apiKey, opts.similarURL, opts)
	}
	numResults := opts.numResults
	if numResults <= 0 {
		numResults = 10
	}
	body := map[string]any{"query": query, "numResults": numResults}
	if opts.category != "" {
		body["category"] = normalizeCategory(opts.category)
	}
	if len(opts.includeDomains) > 0 {
		body["includeDomains"] = opts.includeDomains
	}
	if len(opts.excludeDomains) > 0 {
		body["excludeDomains"] = opts.excludeDomains
	}
	if opts.startPublishedDate != "" {
		body["startPublishedDate"] = opts.startPublishedDate
	}
	if opts.endPublishedDate != "" {
		body["endPublishedDate"] = opts.endPublishedDate
	}
	raw, err := doExaPOST(timeoutCtx, apiKey, exaSearchPath, body)
	if err != nil {
		return SearchResponse{}, err
	}
	var parsed struct {
		Results []exaSearchHit `json:"results"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return SearchResponse{}, fmt.Errorf("decode Exa search response: %w", err)
	}
	items := make([]SearchResultItem, 0, len(parsed.Results))
	for _, hit := range parsed.Results {
		snippet := hit.Text
		if snippet == "" && len(hit.Highlights) > 0 {
			snippet = strings.Join(hit.Highlights, "\n")
		}
		items = append(items, SearchResultItem{
			Title:         hit.Title,
			URL:           hit.URL,
			Snippet:       snippet,
			PublishedDate: hit.PublishedDate,
			Author:        hit.Author,
			Score:         hit.Score,
		})
	}
	return SearchResponse{Query: query, Results: items, Provider: "exa"}, nil
}

// searchOptions mirrors the web_search tool parameters.
type searchOptions struct {
	numResults         int
	category           string
	includeDomains     []string
	excludeDomains     []string
	startPublishedDate string
	endPublishedDate   string
	similarURL         string
}

// exaFindSimilar runs POST /findSimilar.
func exaFindSimilar(ctx context.Context, apiKey, url string, opts searchOptions) (SearchResponse, error) {
	numResults := opts.numResults
	if numResults <= 0 {
		numResults = 10
	}
	body := map[string]any{"url": url, "numResults": numResults}
	if len(opts.includeDomains) > 0 {
		body["includeDomains"] = opts.includeDomains
	}
	if len(opts.excludeDomains) > 0 {
		body["excludeDomains"] = opts.excludeDomains
	}
	if opts.startPublishedDate != "" {
		body["startPublishedDate"] = opts.startPublishedDate
	}
	if opts.endPublishedDate != "" {
		body["endPublishedDate"] = opts.endPublishedDate
	}
	if opts.category != "" {
		body["category"] = normalizeCategory(opts.category)
	}
	raw, err := doExaPOST(ctx, apiKey, exaSimilarPath, body)
	if err != nil {
		return SearchResponse{}, err
	}
	var parsed struct {
		Results []exaSearchHit `json:"results"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return SearchResponse{}, fmt.Errorf("decode Exa find-similar response: %w", err)
	}
	items := make([]SearchResultItem, 0, len(parsed.Results))
	for _, hit := range parsed.Results {
		snippet := hit.Text
		if snippet == "" && len(hit.Highlights) > 0 {
			snippet = strings.Join(hit.Highlights, "\n")
		}
		items = append(items, SearchResultItem{
			Title:         hit.Title,
			URL:           hit.URL,
			Snippet:       snippet,
			PublishedDate: hit.PublishedDate,
			Author:        hit.Author,
			Score:         hit.Score,
		})
	}
	return SearchResponse{Query: url, Results: items, Provider: "exa"}, nil
}

// normalizeFetchURL validates the URL and upgrades http to https.
func normalizeFetchURL(rawURL string) (string, error) {
	trimmed := strings.TrimSpace(rawURL)
	if strings.HasPrefix(trimmed, "http://") {
		return "https://" + strings.TrimPrefix(trimmed, "http://"), nil
	}
	if strings.HasPrefix(trimmed, "https://") {
		return trimmed, nil
	}
	return "", fmt.Errorf("Exa fetch requires a fully-formed http(s) URL, got: %q", rawURL)
}

// exaFetchContents runs POST /contents for a batch of URLs.
func exaFetchContents(ctx context.Context, apiKey string, urls []string, maxCharacters int) ([]FetchResult, error) {
	if maxCharacters <= 0 {
		maxCharacters = 5000
	}
	timeoutCtx, cancel := withTimeout(ctx, fetchTimeoutMs)
	defer cancel()
	normalized := make([]string, 0, len(urls))
	for _, u := range urls {
		n, err := normalizeFetchURL(u)
		if err != nil {
			return nil, err
		}
		normalized = append(normalized, n)
	}
	body := map[string]any{
		"urls": normalized,
		"text": map[string]any{"maxCharacters": maxCharacters},
	}
	raw, err := doExaPOST(timeoutCtx, apiKey, exaContentsPath, body)
	if err != nil {
		return nil, err
	}
	var parsed struct {
		Results []struct {
			URL   string `json:"url"`
			Title string `json:"title"`
			Text  string `json:"text"`
		} `json:"results"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("decode Exa contents response: %w", err)
	}
	out := make([]FetchResult, 0, len(parsed.Results))
	for i, r := range parsed.Results {
		u := r.URL
		if u == "" && i < len(normalized) {
			u = normalized[i]
		}
		content := truncateMarkdown(r.Text, maxCharacters)
		out = append(out, FetchResult{URL: u, Title: r.Title, Content: content, Provider: "exa"})
	}
	return out, nil
}

// exaAnswer runs POST /answer for deep research synthesis.
func exaAnswer(ctx context.Context, apiKey, query string, numSources int, includeText bool) (DeepSearchResponse, error) {
	if numSources <= 0 {
		numSources = 5
	}
	timeoutCtx, cancel := withTimeout(ctx, deepSearchTimeoutMs)
	defer cancel()
	body := map[string]any{"query": query, "text": includeText, "numSources": numSources}
	raw, err := doExaPOST(timeoutCtx, apiKey, exaAnswerPath, body)
	if err != nil {
		return DeepSearchResponse{}, err
	}
	var parsed struct {
		Answer    string `json:"answer"`
		Citations []struct {
			URL   string `json:"url"`
			Title string `json:"title"`
			Text  string `json:"text"`
		} `json:"citations"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return DeepSearchResponse{}, fmt.Errorf("decode Exa answer response: %w", err)
	}
	results := make([]DeepSearchResult, 0, len(parsed.Citations))
	for _, c := range parsed.Citations {
		r := DeepSearchResult{Title: c.Title, URL: c.URL}
		if includeText && c.Text != "" {
			r.Text = c.Text
		}
		results = append(results, r)
	}
	return DeepSearchResponse{Query: query, Results: results, Provider: "exa", Answer: parsed.Answer}, nil
}

// buildDeepSearchQueries returns the main query plus deduplicated extras.
func buildDeepSearchQueries(query string, additional []string) []string {
	main := strings.TrimSpace(query)
	seen := map[string]bool{strings.ToLower(main): true}
	queries := []string{main}
	for _, extra := range additional {
		trimmed := strings.TrimSpace(extra)
		key := strings.ToLower(trimmed)
		if trimmed == "" || seen[key] {
			continue
		}
		seen[key] = true
		queries = append(queries, trimmed)
	}
	return queries
}
