package websearch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Exa MCP endpoint used for keyless public mode. The TypeScript original
// talks to this server for every tool; the Go port uses REST when an API
// key is available and falls back to this MCP endpoint only when no key is
// resolved, preserving the zero-config behaviour for search and fetch.
// (Deep research still requires a key, exactly as upstream.)
const (
	exaMCPEndpoint          = "https://mcp.exa.ai/mcp"
	exaMCPProtocolVersion   = "2024-11-05"
	exaMCPClientName        = "pig-web-search-and-fetch"
	exaMCPClientVersion     = "0.1.0"
	exaMCPSearchTool        = "web_search_exa"
	exaMCPAdvancedTool      = "web_search_advanced_exa"
	exaMCPFetchTool         = "web_fetch_exa"
	exaMCPToolsParam        = "web_search_exa,web_search_advanced_exa,web_fetch_exa"
	exaMCPAPIKeyParam       = "exaApiKey"
	mcpHandshakeTimeoutSecs = 20
)

// mcpURL builds the endpoint URL, attaching the API key as the exaApiKey
// query parameter when one is available (absence selects public free mode).
func mcpURL(apiKey *string) string {
	u, _ := url.Parse(exaMCPEndpoint)
	q := u.Query()
	if q.Get("tools") == "" {
		q.Set("tools", exaMCPToolsParam)
	}
	if apiKey != nil && strings.TrimSpace(*apiKey) != "" {
		q.Set(exaMCPAPIKeyParam, strings.TrimSpace(*apiKey))
	}
	u.RawQuery = q.Encode()
	return u.String()
}

// mcpEnvelope is the JSON-RPC envelope carried inside the SSE data field
// (or as a bare JSON body for 202 responses).
type mcpEnvelope struct {
	Result *mcpCallResult `json:"result"`
	Error  *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
	ID any `json:"id"`
}

type mcpCallResult struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	IsError bool `json:"isError"`
}

// mcpPost sends one JSON-RPC message and returns the session id (from the
// response header, when present) plus the decoded envelope.
func mcpPost(ctx context.Context, endpoint, sessionID string, payload any) (newSession string, env *mcpEnvelope, rawBody []byte, err error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return "", nil, nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", nil, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	if sessionID != "" {
		req.Header.Set("Mcp-Session-Id", sessionID)
	}
	resp, err := exaHTTPClient.Do(req)
	if err != nil {
		return "", nil, nil, err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return "", nil, nil, err
	}
	newSession = resp.Header.Get("Mcp-Session-Id")
	if resp.StatusCode == http.StatusAccepted {
		return newSession, nil, respBody, nil
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return newSession, nil, respBody, fmt.Errorf("Exa MCP error (HTTP %d): %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	decoded, derr := decodeSSEEnvelope(respBody)
	if derr != nil {
		return newSession, nil, respBody, derr
	}
	return newSession, decoded, respBody, nil
}

// decodeSSEEnvelope extracts the JSON payload from a StreamableHTTP body:
// either bare JSON or one "data: {...}" SSE field.
func decodeSSEEnvelope(body []byte) (*mcpEnvelope, error) {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return nil, fmt.Errorf("empty Exa MCP response")
	}
	if trimmed[0] == '{' {
		var env mcpEnvelope
		if err := json.Unmarshal(trimmed, &env); err != nil {
			return nil, fmt.Errorf("decode Exa MCP response: %w", err)
		}
		return &env, nil
	}
	for _, line := range strings.Split(string(body), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "data:") {
			payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if payload == "" || payload == "[DONE]" {
				continue
			}
			var env mcpEnvelope
			if err := json.Unmarshal([]byte(payload), &env); err != nil {
				continue
			}
			return &env, nil
		}
	}
	return nil, fmt.Errorf("Exa MCP returned no parseable result")
}

// mcpSession opens a session: initialize → notifications/initialized.
func mcpSession(ctx context.Context, endpoint string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, mcpHandshakeTimeoutSecs*time.Second)
	defer cancel()
	session, env, _, err := mcpPost(ctx, endpoint, "", map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]any{
			"protocolVersion": exaMCPProtocolVersion,
			"capabilities":    map[string]any{},
			"clientInfo":      map[string]any{"name": exaMCPClientName, "version": exaMCPClientVersion},
		},
	})
	if err != nil {
		return "", err
	}
	if env != nil && env.Error != nil {
		return "", fmt.Errorf("Exa MCP initialize failed: %s", env.Error.Message)
	}
	if session == "" {
		return "", fmt.Errorf("Exa MCP initialize returned no session")
	}
	// Fire-and-forget per the protocol; a failure here must not fail the call,
	// the server proceeds without it in practice.
	_, _, _, _ = mcpPost(ctx, endpoint, session, map[string]any{
		"jsonrpc": "2.0",
		"method":  "notifications/initialized",
		"params":  map[string]any{},
	})
	return session, nil
}

// mcpCallTool invokes one MCP tool in a fresh session and returns its text
// blocks. A fresh session per call keeps the client stateless (no shared
// connection across runtime-cell reloads).
func mcpCallTool(ctx context.Context, apiKey *string, tool string, args map[string]any) ([]string, error) {
	endpoint := mcpURL(apiKey)
	session, err := mcpSession(ctx, endpoint)
	if err != nil {
		return nil, err
	}
	_, env, _, err := mcpPost(ctx, endpoint, session, map[string]any{
		"jsonrpc": "2.0",
		"id":      3,
		"method":  "tools/call",
		"params":  map[string]any{"name": tool, "arguments": args},
	})
	if err != nil {
		return nil, err
	}
	if env == nil {
		return nil, fmt.Errorf("Exa MCP %s returned no result", tool)
	}
	if env.Error != nil {
		return nil, fmt.Errorf("Exa %s failed: %s", tool, env.Error.Message)
	}
	if env.Result == nil {
		return nil, fmt.Errorf("Exa %s returned no parseable result", tool)
	}
	if env.Result.IsError {
		var texts []string
		for _, b := range env.Result.Content {
			if b.Type == "text" && b.Text != "" {
				texts = append(texts, b.Text)
			}
		}
		msg := strings.TrimSpace(strings.Join(texts, "\n"))
		if msg == "" {
			msg = "unknown tool error"
		}
		return nil, fmt.Errorf("Exa %s failed: %s", tool, msg)
	}
	var out []string
	for _, b := range env.Result.Content {
		if b.Type == "text" && b.Text != "" {
			out = append(out, b.Text)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("Exa %s returned no parseable result", tool)
	}
	return out, nil
}

// parseMCPTextSearch parses the human-readable blocks returned by
// web_search_exa / web_search_advanced_exa: blocks separated by --- with
// Title:, URL:, Published:, Author: and Highlights: sections.
func parseMCPTextSearch(text string) []exaSearchHit {
	blocks := splitMCPBlocks(text)
	var out []exaSearchHit
	for _, block := range blocks {
		var hit exaSearchHit
		var snippet []string
		inHighlights := false
		for _, line := range strings.Split(block, "\n") {
			if !inHighlights {
				switch {
				case strings.HasPrefix(line, "Title: "):
					hit.Title = strings.TrimSpace(strings.TrimPrefix(line, "Title: "))
					continue
				case strings.HasPrefix(line, "URL: "):
					hit.URL = strings.TrimSpace(strings.TrimPrefix(line, "URL: "))
					continue
				case strings.HasPrefix(line, "Published: "):
					val := strings.TrimSpace(strings.TrimPrefix(line, "Published: "))
					if val != "" && val != "N/A" {
						hit.PublishedDate = val
					}
					continue
				case strings.HasPrefix(line, "Author: "):
					val := strings.TrimSpace(strings.TrimPrefix(line, "Author: "))
					if val != "" && val != "N/A" {
						hit.Author = val
					}
					continue
				case strings.HasPrefix(line, "Highlights:"):
					inHighlights = true
					continue
				}
			}
			if inHighlights {
				snippet = append(snippet, line)
			}
		}
		if hit.URL != "" || hit.Title != "" {
			if hit.Title == "" {
				hit.Title = hit.URL
			}
			if t := strings.TrimSpace(strings.Join(snippet, "\n")); t != "" {
				hit.Text = t
			}
			out = append(out, hit)
		}
	}
	return out
}

func splitMCPBlocks(text string) []string {
	var out []string
	for _, chunk := range strings.Split(text, "\n---\n") {
		if t := strings.TrimSpace(chunk); t != "" {
			out = append(out, t)
		}
	}
	if len(out) == 0 {
		if t := strings.TrimSpace(text); t != "" {
			return []string{t}
		}
	}
	return out
}

// extractMCPSearchHits accepts both JSON payloads ({results:[...]} or bare
// array) and the plain text format above.
func extractMCPSearchHits(blocks []string) ([]exaSearchHit, error) {
	for _, text := range blocks {
		if parsed := tryParseSearchJSON(text); parsed != nil {
			return parsed, nil
		}
		if hits := parseMCPTextSearch(text); len(hits) > 0 {
			return hits, nil
		}
	}
	return nil, fmt.Errorf("Exa search returned no parseable results")
}

func tryParseSearchJSON(text string) []exaSearchHit {
	var asArray []exaSearchHit
	if err := json.Unmarshal([]byte(text), &asArray); err == nil && len(asArray) > 0 {
		return asArray
	}
	var wrapped struct {
		Results []exaSearchHit `json:"results"`
	}
	if err := json.Unmarshal([]byte(text), &wrapped); err == nil && len(wrapped.Results) > 0 {
		return wrapped.Results
	}
	return nil
}

// mcpSearch runs a search through the keyless MCP endpoint.
func mcpSearch(ctx context.Context, query string, opts searchOptions) (SearchResponse, error) {
	// Transparent dispatch: similarUrl goes to find-similar first, falling
	// back to a plain search when the tool is not offered (the default
	// ?tools filter only advertises search/fetch).
	if opts.similarURL != "" {
		if resp, err := mcpFindSimilar(ctx, opts.similarURL, opts); err == nil {
			return resp, nil
		}
	}
	hasAdvanced := opts.category != "" || len(opts.includeDomains) > 0 || len(opts.excludeDomains) > 0 ||
		opts.startPublishedDate != "" || opts.endPublishedDate != ""
	tool := exaMCPSearchTool
	args := map[string]any{"query": query, "numResults": positiveOr(opts.numResults, 10)}
	if hasAdvanced {
		tool = exaMCPAdvancedTool
		if opts.category != "" {
			args["category"] = normalizeCategory(opts.category)
		}
		if len(opts.includeDomains) > 0 {
			args["includeDomains"] = opts.includeDomains
		}
		if len(opts.excludeDomains) > 0 {
			args["excludeDomains"] = opts.excludeDomains
		}
		if opts.startPublishedDate != "" {
			args["startPublishedDate"] = opts.startPublishedDate
		}
		if opts.endPublishedDate != "" {
			args["endPublishedDate"] = opts.endPublishedDate
		}
	}
	blocks, err := mcpCallTool(ctx, nil, tool, args)
	if err != nil {
		return SearchResponse{}, err
	}
	hits, err := extractMCPSearchHits(blocks)
	if err != nil {
		return SearchResponse{}, err
	}
	items := make([]SearchResultItem, 0, len(hits))
	for _, hit := range hits {
		snippet := hit.Text
		if snippet == "" && len(hit.Highlights) > 0 {
			snippet = strings.Join(hit.Highlights, "\n")
		}
		items = append(items, SearchResultItem{
			Title: hit.Title, URL: hit.URL, Snippet: snippet,
			PublishedDate: hit.PublishedDate, Author: hit.Author, Score: hit.Score,
		})
	}
	return SearchResponse{Query: query, Results: items, Provider: "exa"}, nil
}

// mcpFetchOne fetches a single URL through the keyless MCP endpoint.
func mcpFetchOne(ctx context.Context, rawURL string, maxCharacters int) (FetchResult, error) {
	normalized, err := normalizeFetchURL(rawURL)
	if err != nil {
		return FetchResult{}, err
	}
	if maxCharacters <= 0 {
		maxCharacters = 5000
	}
	blocks, err := mcpCallTool(ctx, nil, exaMCPFetchTool, map[string]any{
		"urls": []string{normalized}, "maxCharacters": maxCharacters,
	})
	if err != nil {
		return FetchResult{}, err
	}
	for _, text := range blocks {
		if fr := parseMCPFetchText(text, normalized, maxCharacters); fr != nil {
			return *fr, nil
		}
	}
	return FetchResult{}, fmt.Errorf("Exa fetch returned no parseable content")
}

// parseMCPFetchText parses "# Title\nURL: ...\n\ncontent" plus the JSON
// shapes the tool may return instead.
func parseMCPFetchText(text, fallbackURL string, maxCharacters int) *FetchResult {
	trimmed := strings.TrimSpace(text)
	if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
		var obj struct {
			URL     string `json:"url"`
			Title   string `json:"title"`
			Content string `json:"content"`
			Text    string `json:"text"`
		}
		if err := json.Unmarshal([]byte(trimmed), &obj); err == nil {
			content := obj.Content
			if content == "" {
				content = obj.Text
			}
			if content != "" {
				u := obj.URL
				if u == "" {
					u = fallbackURL
				}
				return &FetchResult{URL: u, Title: obj.Title, Content: truncateMarkdown(content, maxCharacters), Provider: "exa"}
			}
		}
		return nil
	}
	lines := strings.Split(trimmed, "\n")
	var title string
	u := fallbackURL
	i := 0
	if len(lines) > 0 && strings.HasPrefix(lines[0], "# ") {
		title = strings.TrimSpace(strings.TrimPrefix(lines[0], "# "))
		i = 1
	}
	if len(lines) > i && strings.HasPrefix(lines[i], "URL: ") {
		if v := strings.TrimSpace(strings.TrimPrefix(lines[i], "URL: ")); v != "" {
			u = v
		}
		i++
	}
	content := strings.TrimSpace(strings.Join(lines[i:], "\n"))
	if content == "" && title == "" {
		return nil
	}
	if content == "" {
		content = trimmed
	}
	return &FetchResult{URL: u, Title: title, Content: truncateMarkdown(content, maxCharacters), Provider: "exa"}
}

func positiveOr(v, def int) int {
	if v > 0 {
		return v
	}
	return def
}

// mcpFindSimilar tries the find-similar MCP tool for keyless similarUrl
// dispatch. The default tools filter does not advertise it, so a rejection
// falls back to the caller (plain search).
func mcpFindSimilar(ctx context.Context, rawURL string, opts searchOptions) (SearchResponse, error) {
	args := map[string]any{"url": rawURL, "numResults": positiveOr(opts.numResults, 10)}
	if len(opts.includeDomains) > 0 {
		args["includeDomains"] = opts.includeDomains
	}
	if len(opts.excludeDomains) > 0 {
		args["excludeDomains"] = opts.excludeDomains
	}
	if opts.startPublishedDate != "" {
		args["startPublishedDate"] = opts.startPublishedDate
	}
	if opts.endPublishedDate != "" {
		args["endPublishedDate"] = opts.endPublishedDate
	}
	blocks, err := mcpCallTool(ctx, nil, "web_find_similar_exa", args)
	if err != nil {
		return SearchResponse{}, err
	}
	hits, err := extractMCPSearchHits(blocks)
	if err != nil {
		return SearchResponse{}, err
	}
	items := make([]SearchResultItem, 0, len(hits))
	for _, hit := range hits {
		snippet := hit.Text
		if snippet == "" && len(hit.Highlights) > 0 {
			snippet = strings.Join(hit.Highlights, "\n")
		}
		items = append(items, SearchResultItem{
			Title: hit.Title, URL: hit.URL, Snippet: snippet,
			PublishedDate: hit.PublishedDate, Author: hit.Author, Score: hit.Score,
		})
	}
	return SearchResponse{Query: rawURL, Results: items, Provider: "exa"}, nil
}
