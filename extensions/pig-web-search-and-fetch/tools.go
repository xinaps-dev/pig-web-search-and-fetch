package websearch

import (
	"context"
	"fmt"
	"strings"

	sdk "github.com/MichaelKinsy/PiG/extensions/sdk"
)

// Prompt snippets and guidelines mirror the TypeScript originals so the
// system prompt stays identical across harnesses.
const (
	searchDescription = "Search the web for current information, news, facts, people, companies, or documentation about any topic. Returns clean search results with titles, URLs, highlights, and publish dates. For extracting full page content, follow up with web_fetch."
	searchSnippet     = "Search the web for current information, news, facts, people, companies, or documentation"
	fetchDescription  = "Read one or multiple webpage URLs and extract their full content as clean, readable markdown. Supports batch processing of multiple URLs in a single call. Use after web_search when highlights are insufficient or whenever you already have specific URLs to inspect."
	fetchSnippet      = "Fetch full clean markdown content from one or multiple known webpage URLs"
	deepDescription   = "Perform an in-depth web investigation and direct question-answering with synthesized answers grounded in authoritative sources. Use this for complex questions that require multi-source synthesis, deep verification, or direct comprehensive answers with citations."
	deepSnippet       = "In-depth web investigation with direct, source-grounded answers and citations"
)

var (
	searchGuidelines = []string{
		"Use web_search for discovering information, current events, recent documentation, or finding URLs beyond training data cutoff.",
		"Always use the current year when searching for recent developments or current versions.",
		"Use includeDomains / excludeDomains to restrict or filter results by domain, and startPublishedDate / endPublishedDate to filter by publication date.",
		"Tip: Use web_search to discover information, and web_fetch to retrieve full content from a specific known URL.",
	}
	fetchGuidelines = []string{
		"Use web_fetch to retrieve and analyze full content from one or multiple specific, known URLs (retrieval vs discovery).",
		"Pass 'urls' as a single string or an array of strings to batch-fetch multiple pages in a single call; each page is returned separately.",
	}
	deepGuidelines = []string{
		"Use web_deep_search for complex questions that require multi-source synthesis, deep verification, or direct comprehensive answers with citations.",
	}
)

// Schemas as plain JSON Schema maps (sdk.Schema is map[string]any).
func webSearchSchema() sdk.Schema {
	return sdk.Schema{
		"type": "object",
		"properties": map[string]any{
			"query":              map[string]any{"type": "string", "description": "Semantic search query or keywords"},
			"numResults":         map[string]any{"type": "number", "description": "Number of results to return (default: 10)"},
			"category":           map[string]any{"type": "string", "description": "Optional category filter: 'company', 'research_paper', 'news', 'pdf', 'github', 'tweet', 'personal_site'"},
			"includeDomains":     map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Optional list of domains to restrict the search to"},
			"excludeDomains":     map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Optional list of domains to exclude from the search"},
			"startPublishedDate": map[string]any{"type": "string", "description": "Optional ISO date: only return results published on or after this date"},
			"endPublishedDate":   map[string]any{"type": "string", "description": "Optional ISO date: only return results published on or before this date"},
			"similarUrl":         map[string]any{"type": "string", "description": "Optional URL: when provided, results similar to that URL are returned"},
		},
		"required": []any{"query"},
	}
}

func webFetchSchema() sdk.Schema {
	return sdk.Schema{
		"type": "object",
		"properties": map[string]any{
			"urls":          map[string]any{"description": "One or multiple webpage URLs to fetch"},
			"url":           map[string]any{"type": "string", "description": "Optional single URL for compatibility"},
			"maxCharacters": map[string]any{"type": "number", "description": "Maximum length of extracted content per page (default: 5000)"},
		},
	}
}

func webDeepSearchSchema() sdk.Schema {
	return sdk.Schema{
		"type": "object",
		"properties": map[string]any{
			"query":             map[string]any{"type": "string", "description": "In-depth research query or question to answer"},
			"numSources":        map[string]any{"type": "number", "description": "Number of reference sources to consult (default: 5)"},
			"includeText":       map[string]any{"type": "boolean", "description": "Whether to include text extracts from cited sources (default: true)"},
			"numResults":        map[string]any{"type": "number", "description": "Number of results to return per query (default: 10)"},
			"category":          map[string]any{"type": "string", "description": "Optional content category filter, e.g. 'company', 'research paper', 'news', 'github', 'pdf', 'tweet'"},
			"additionalQueries": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Optional complementary sub-queries executed in parallel for broader coverage"},
		},
		"required": []any{"query"},
	}
}

// --- formatting (mirrors the TypeScript format* helpers) ---

func formatSearchResults(resp SearchResponse) string {
	if len(resp.Results) == 0 {
		return securityNoticePrefix + "\n" + fmt.Sprintf("No web search results found for %q.", resp.Query)
	}
	var b strings.Builder
	b.WriteString(securityNoticePrefix + "\n")
	fmt.Fprintf(&b, "Web search results for %q (provider: %s, %d results):\n", resp.Query, resp.Provider, len(resp.Results))
	for i, item := range resp.Results {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString("\n")
		fmt.Fprintf(&b, "%d. %s\n", i+1, item.Title)
		fmt.Fprintf(&b, "   URL: %s\n", item.URL)
		var meta []string
		if item.PublishedDate != "" {
			meta = append(meta, "Published: "+item.PublishedDate)
		}
		if item.Author != "" {
			meta = append(meta, "Author: "+item.Author)
		}
		if len(meta) > 0 {
			b.WriteString("   " + strings.Join(meta, " | ") + "\n")
		}
		if item.Snippet != "" {
			b.WriteString(wrapWebContent(item.Snippet, item.URL, item.Title) + "\n")
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

func formatFetchResults(pages []FetchResult) string {
	if len(pages) == 0 {
		return securityNoticePrefix + "\nNo web fetch results returned."
	}
	var b strings.Builder
	b.WriteString(securityNoticePrefix)
	for _, page := range pages {
		b.WriteString("\n")
		if page.Content == "" {
			fmt.Fprintf(&b, "\nNo content could be extracted from %q.", page.URL)
			continue
		}
		b.WriteString("\n" + wrapWebContent(page.Content, page.URL, page.Title))
	}
	return b.String()
}

func formatDeepSearchResults(resp DeepSearchResponse) string {
	if len(resp.Results) == 0 {
		return securityNoticePrefix + "\n" + fmt.Sprintf("No deep search results found for %q.", resp.Query)
	}
	var b strings.Builder
	b.WriteString(securityNoticePrefix + "\n")
	fmt.Fprintf(&b, "Deep search results for %q (provider: %s, %d results):\n", resp.Query, resp.Provider, len(resp.Results))
	if len(resp.SubQueriesExecuted) > 0 {
		b.WriteString("Sub-queries executed: " + strings.Join(resp.SubQueriesExecuted, ", ") + "\n")
	}
	if resp.Answer != "" {
		b.WriteString("\n" + wrapWebContent(resp.Answer, "", "Synthesized Answer") + "\n")
	}
	b.WriteString("\n")
	for i, item := range resp.Results {
		fmt.Fprintf(&b, "%d. %s\n", i+1, item.Title)
		fmt.Fprintf(&b, "   URL: %s\n", item.URL)
		var meta []string
		if item.PublishedDate != "" {
			meta = append(meta, "Published: "+item.PublishedDate)
		}
		if item.Author != "" {
			meta = append(meta, "Author: "+item.Author)
		}
		if len(meta) > 0 {
			b.WriteString("   " + strings.Join(meta, " | ") + "\n")
		}
		var excerpt []string
		if len(item.Highlights) > 0 {
			excerpt = append(excerpt, "Highlights: "+strings.Join(item.Highlights, "; "))
		}
		if item.Text != "" {
			excerpt = append(excerpt, item.Text)
		}
		if len(excerpt) > 0 {
			b.WriteString(wrapWebContent(strings.Join(excerpt, "\n"), item.URL, item.Title) + "\n")
		}
		if i < len(resp.Results)-1 {
			b.WriteString("\n")
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

// --- param helpers ---

func stringParam(params map[string]any, key string) string {
	v, _ := params[key].(string)
	return strings.TrimSpace(v)
}

func numberParam(params map[string]any, key string, def float64) float64 {
	switch v := params[key].(type) {
	case float64:
		return v
	case int:
		return float64(v)
	case int64:
		return float64(v)
	default:
		return def
	}
}

func boolParam(params map[string]any, key string, def bool) bool {
	if v, ok := params[key].(bool); ok {
		return v
	}
	return def
}

func stringSliceParam(params map[string]any, key string) []string {
	raw, ok := params[key]
	if !ok || raw == nil {
		return nil
	}
	switch v := raw.(type) {
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	case []string:
		return v
	case string:
		return []string{v}
	default:
		return nil
	}
}

// toolContext combines the handler cancellation (ctx.Done) with the run
// signal into one context for outbound HTTP calls.
func toolContext(ctx sdk.Context) (context.Context, context.CancelFunc) {
	base := ctx.Signal()
	if base == nil {
		base = context.Background()
	}
	derived, cancel := context.WithCancel(base)
	done := ctx.Done()
	if done == nil {
		return derived, cancel
	}
	go func() {
		select {
		case <-done:
			cancel()
		case <-derived.Done():
		}
	}()
	return derived, cancel
}

// --- tool handlers ---

func webSearchHandler(ctx sdk.Context, params map[string]any) (any, error) {
	query := stringParam(params, "query")
	if query == "" {
		return nil, sdk.NewToolError("web_search requires a non-empty \"query\".")
	}
	cfg := getConfig()
	if cfg.Search.Provider != "" && cfg.Search.Provider != "exa" {
		return nil, sdk.NewToolError(fmt.Sprintf("Unknown provider %q for web_search. Registered providers: exa.", cfg.Search.Provider))
	}
	tctx, cancel := toolContext(ctx)
	defer cancel()
	opts := searchOptions{
		numResults:         int(numberParam(params, "numResults", 10)),
		category:           stringParam(params, "category"),
		includeDomains:     stringSliceParam(params, "includeDomains"),
		excludeDomains:     stringSliceParam(params, "excludeDomains"),
		startPublishedDate: stringParam(params, "startPublishedDate"),
		endPublishedDate:   stringParam(params, "endPublishedDate"),
		similarURL:         stringParam(params, "similarUrl"),
	}
	// Authenticated path: Exa REST. Keyless path: public MCP endpoint
	// (zero-config, global limits) — mirrors the TypeScript original.
	if apiKey, err := requireAPIKey(); err == nil {
		resp, err := exaSearch(tctx, apiKey, query, opts)
		if err != nil {
			return nil, sdk.NewToolError(maskCredentials(err.Error()))
		}
		return sdk.ToolResult{Content: formatSearchResults(resp), Details: resp}, nil
	}
	// No key: public free mode through MCP (global limits).
	resp, err := mcpSearch(tctx, query, opts)
	if err != nil {
		return nil, sdk.NewToolError(maskCredentials(err.Error()))
	}
	return sdk.ToolResult{Content: formatSearchResults(resp), Details: resp}, nil
}

func webFetchHandler(ctx sdk.Context, params map[string]any) (any, error) {
	cfg := getConfig()
	if cfg.Fetch.Provider != "" && cfg.Fetch.Provider != "exa" {
		return nil, sdk.NewToolError(fmt.Sprintf("Unknown provider %q for web_fetch. Registered providers: exa.", cfg.Fetch.Provider))
	}
	var urls []string
	if raw, ok := params["urls"]; ok && raw != nil {
		switch v := raw.(type) {
		case string:
			if strings.TrimSpace(v) != "" {
				urls = []string{v}
			}
		case []any:
			for _, item := range v {
				if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
					urls = append(urls, s)
				}
			}
		case []string:
			urls = append(urls, v...)
		}
	}
	if len(urls) == 0 {
		if single := stringParam(params, "url"); single != "" {
			urls = []string{single}
		}
	}
	if len(urls) == 0 {
		return nil, sdk.NewToolError("web_fetch requires \"urls\" (string or string[]) or \"url\".")
	}
	maxChars := int(numberParam(params, "maxCharacters", 5000))
	if maxChars <= 0 {
		maxChars = 5000
	}
	tctx, cancel := toolContext(ctx)
	defer cancel()
	// Authenticated path: Exa REST /contents. Keyless path: public MCP
	// endpoint — mirrors the TypeScript zero-config behaviour.
	if apiKey, err := requireAPIKey(); err == nil {
		pages, err := exaFetchContents(tctx, apiKey, urls, maxChars)
		if err != nil {
			// Batch failure degrades per URL, so one bad page never sinks the
			// whole batch (mirrors Promise.allSettled in the original).
			if len(urls) > 1 {
				fallback := make([]FetchResult, 0, len(urls))
				for _, u := range urls {
					single, ferr := exaFetchContents(tctx, apiKey, []string{u}, maxChars)
					if ferr != nil {
						fallback = append(fallback, FetchResult{URL: u, Title: "Error", Content: fmt.Sprintf("Failed to fetch %s: %s", u, maskCredentials(ferr.Error())), Provider: "exa"})
						continue
					}
					fallback = append(fallback, single...)
				}
				return sdk.ToolResult{Content: formatFetchResults(fallback), Details: fallback}, nil
			}
			return nil, sdk.NewToolError(maskCredentials(err.Error()))
		}
		return sdk.ToolResult{Content: formatFetchResults(pages), Details: pages}, nil
	}
	// No key: fetch each URL through the public MCP endpoint.
	pages := make([]FetchResult, 0, len(urls))
	for _, u := range urls {
		if tctx.Err() != nil {
			return nil, sdk.NewToolError("Exa fetch aborted")
		}
		page, err := mcpFetchOne(tctx, u, maxChars)
		if err != nil {
			normalized, nerr := normalizeFetchURL(u)
			if nerr != nil {
				return nil, sdk.NewToolError(nerr.Error())
			}
			pages = append(pages, FetchResult{URL: normalized, Title: "Error", Content: fmt.Sprintf("Failed to fetch %s: %s", u, maskCredentials(err.Error())), Provider: "exa"})
			continue
		}
		pages = append(pages, page)
	}
	return sdk.ToolResult{Content: formatFetchResults(pages), Details: pages}, nil
}

func webDeepSearchHandler(ctx sdk.Context, params map[string]any) (any, error) {
	query := stringParam(params, "query")
	if query == "" {
		return nil, sdk.NewToolError("web_deep_search requires a non-empty \"query\".")
	}
	cfg := getConfig()
	if cfg.DeepSearch.Provider != "" && cfg.DeepSearch.Provider != "exa" {
		return nil, sdk.NewToolError(fmt.Sprintf("Unknown provider %q for web_deep_search. Registered providers: exa.", cfg.DeepSearch.Provider))
	}
	apiKey, err := requireAPIKey()
	if err != nil {
		return nil, sdk.NewToolError(err.Error())
	}
	numSources := int(numberParam(params, "numSources", 5))
	if n := int(numberParam(params, "numResults", 0)); n > 0 && numSources <= 0 {
		numSources = n
	}
	if numSources <= 0 {
		numSources = 5
	}
	includeText := boolParam(params, "includeText", true)
	category := stringParam(params, "category")
	additional := stringSliceParam(params, "additionalQueries")
	tctx, cancel := toolContext(ctx)
	defer cancel()

	// Preferred path: synthesized answer with citations.
	if answer, aerr := exaAnswer(tctx, apiKey, query, numSources, includeText); aerr == nil {
		return sdk.ToolResult{Content: formatDeepSearchResults(answer), Details: answer}, nil
	} else if tctx.Err() != nil {
		return nil, sdk.NewToolError("Exa deep search aborted")
	} else {
		// Fallback: parallel multi-query search with URL dedup, mirroring
		// the TypeScript answer() → deepSearch() fallback.
		queries := buildDeepSearchQueries(query, additional)
		type outcome struct {
			results []SearchResultItem
			err     error
		}
		outcomes := make([]outcome, len(queries))
		done := make(chan struct{})
		go func() {
			defer close(done)
			sem := make(chan struct{}, 4)
			wait := make(chan int, len(queries))
			for i, q := range queries {
				i, q := i, q
				go func() {
					sem <- struct{}{}
					defer func() { <-sem; wait <- i }()
					r, err := exaSearch(tctx, apiKey, q, searchOptions{numResults: numSources, category: category})
					if err != nil {
						outcomes[i] = outcome{err: err}
						return
					}
					outcomes[i] = outcome{results: r.Results}
				}()
			}
			for range queries {
				<-wait
			}
		}()
		select {
		case <-tctx.Done():
			return nil, sdk.NewToolError("Exa deep search aborted")
		case <-done:
		}
		if outcomes[0].err != nil {
			return nil, sdk.NewToolError(maskCredentials(outcomes[0].err.Error()))
		}
		byURL := map[string]DeepSearchResult{}
		order := []string{}
		var executed []string
		for i, q := range queries {
			if outcomes[i].err != nil {
				continue
			}
			executed = append(executed, q)
			for _, hit := range outcomes[i].results {
				key := strings.ToLower(hit.URL)
				if _, ok := byURL[key]; !ok {
					order = append(order, key)
					byURL[key] = DeepSearchResult{Title: hit.Title, URL: hit.URL, Text: hit.Snippet, PublishedDate: hit.PublishedDate, Author: hit.Author}
				}
			}
		}
		results := make([]DeepSearchResult, 0, len(order))
		for _, key := range order {
			r := byURL[key]
			if !includeText {
				r.Text = ""
				r.Highlights = nil
			}
			results = append(results, r)
		}
		resp := DeepSearchResponse{Query: query, Results: results, SubQueriesExecuted: executed, Provider: "exa"}
		return sdk.ToolResult{Content: formatDeepSearchResults(resp), Details: resp}, nil
	}
}
