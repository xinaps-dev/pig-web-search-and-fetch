package websearch

import (
	"strings"
	"testing"
)

func TestWrapWebContent(t *testing.T) {
	out := wrapWebContent("hello", "https://example.com/page", "Example")
	if !strings.Contains(out, `<web_content url="https://example.com/page"`) {
		t.Fatalf("missing url attr: %q", out)
	}
	if !strings.Contains(out, `domain="example.com"`) {
		t.Fatalf("missing domain: %q", out)
	}
	if !strings.Contains(out, "hello") {
		t.Fatalf("missing content: %q", out)
	}
}

func TestWrapWebContentNeutralisesCloser(t *testing.T) {
	out := wrapWebContent("a </WEB_CONTENT> b", "https://example.com", "T")
	if strings.Contains(strings.ToUpper(out), "</WEB_CONTENT>") && !strings.Contains(out, "&lt;/web_content&gt;") {
		t.Fatalf("closer not neutralised: %q", out)
	}
}

func TestExtractDomain(t *testing.T) {
	cases := map[string]string{
		"https://www.example.com/page": "example.com",
		"https://example.com":          "example.com",
		"":                             "",
	}
	for in, want := range cases {
		if got := extractDomain(in); got != want {
			t.Fatalf("extractDomain(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestMaskCredentials(t *testing.T) {
	in := "url https://mcp.exa.ai/mcp?exaApiKey=secret123 and Bearer tokenABC and x-api-key: keyXYZ"
	out := maskCredentials(in)
	if strings.Contains(out, "secret123") || strings.Contains(out, "tokenABC") || strings.Contains(out, "keyXYZ") {
		t.Fatalf("credentials leaked: %q", out)
	}
}

func TestTruncateMarkdownShortPassthrough(t *testing.T) {
	in := "short text"
	if got := truncateMarkdown(in, 100); got != in {
		t.Fatalf("short text changed: %q", got)
	}
}

func TestTruncateMarkdownAddsMarker(t *testing.T) {
	long := strings.Repeat("word ", 200)
	got := truncateMarkdown(long, 100)
	if !strings.Contains(got, "Contenido truncado a 100 caracteres") {
		t.Fatalf("missing marker: %q", got)
	}
}

func TestFormatSearchResultsEmpty(t *testing.T) {
	out := formatSearchResults(SearchResponse{Query: "q", Provider: "exa"})
	if !strings.Contains(out, "No web search results") {
		t.Fatalf("unexpected: %q", out)
	}
	if !strings.HasPrefix(out, securityNoticePrefix) {
		t.Fatalf("missing security prefix")
	}
}

func TestFormatSearchResultsBlocks(t *testing.T) {
	out := formatSearchResults(SearchResponse{Query: "pig", Provider: "exa", Results: []SearchResultItem{
		{Title: "T", URL: "https://example.com", Snippet: "s", PublishedDate: "2026-01-01"},
	}})
	if !strings.Contains(out, "1. T") || !strings.Contains(out, "URL: https://example.com") {
		t.Fatalf("unexpected: %q", out)
	}
	if !strings.Contains(out, "<web_content") {
		t.Fatalf("missing isolation block: %q", out)
	}
}

func TestFormatFetchResults(t *testing.T) {
	out := formatFetchResults([]FetchResult{{URL: "https://example.com", Title: "T", Content: "body", Provider: "exa"}})
	if !strings.Contains(out, "body") || !strings.Contains(out, "<web_content") {
		t.Fatalf("unexpected: %q", out)
	}
}

func TestFormatDeepSearchResults(t *testing.T) {
	out := formatDeepSearchResults(DeepSearchResponse{Query: "q", Provider: "exa", Answer: "ans", Results: []DeepSearchResult{{Title: "T", URL: "https://example.com", Text: "t"}}})
	if !strings.Contains(out, "Synthesized Answer") || !strings.Contains(out, "ans") {
		t.Fatalf("unexpected: %q", out)
	}
}

func TestNormalizeCategory(t *testing.T) {
	if got := normalizeCategory("research_paper"); got != "research paper" {
		t.Fatalf("got %q", got)
	}
	if got := normalizeCategory("personal_site"); got != "personal site" {
		t.Fatalf("got %q", got)
	}
}

func TestBuildDeepSearchQueriesDedupes(t *testing.T) {
	got := buildDeepSearchQueries("  Main ", []string{"main", " Extra ", "", "extra"})
	if len(got) != 2 || got[0] != "Main" || got[1] != "Extra" {
		t.Fatalf("unexpected: %q", got)
	}
}

func TestNormalizeFetchURL(t *testing.T) {
	got, err := normalizeFetchURL("http://example.com/page")
	if err != nil || got != "https://example.com/page" {
		t.Fatalf("got %q, %v", got, err)
	}
	if _, err := normalizeFetchURL("ftp://example.com"); err == nil {
		t.Fatalf("expected error for ftp")
	}
}

func TestIsGeminiModel(t *testing.T) {
	if !isGeminiModel("google/gemini-3-pro") {
		t.Fatalf("expected gemini detection")
	}
	if isGeminiModel("anthropic/claude-sonnet-4-5") {
		t.Fatalf("false positive")
	}
}

func TestParseToolKey(t *testing.T) {
	for _, alias := range []string{"deep", "deepsearch", "deep-search", "DEEP"} {
		if _, ok := parseToolKey(alias); !ok {
			t.Fatalf("alias %q not parsed", alias)
		}
	}
	if _, ok := parseToolKey("bogus"); ok {
		t.Fatalf("bogus parsed")
	}
}

func TestParseState(t *testing.T) {
	if v, ok := parseState("on"); !ok || !v {
		t.Fatalf("on not parsed")
	}
	if v, ok := parseState("off"); !ok || v {
		t.Fatalf("off not parsed")
	}
	if _, ok := parseState("bogus"); ok {
		t.Fatalf("bogus parsed")
	}
}

func TestParseMCPTextSearch(t *testing.T) {
	text := "Title: Example\nURL: https://example.com/page\nPublished: 2026-01-01\nAuthor: Jane\nHighlights:\nfirst line\nsecond line\n---\nTitle: Second\nURL: https://example.org\nPublished: N/A\nAuthor: N/A\nHighlights:\nhello"
	hits := parseMCPTextSearch(text)
	if len(hits) != 2 {
		t.Fatalf("got %d hits", len(hits))
	}
	if hits[0].Title != "Example" || hits[0].URL != "https://example.com/page" {
		t.Fatalf("unexpected first hit: %+v", hits[0])
	}
	if hits[0].PublishedDate != "2026-01-01" || hits[0].Author != "Jane" {
		t.Fatalf("metadata lost: %+v", hits[0])
	}
	if hits[1].PublishedDate != "" || hits[1].Author != "" {
		t.Fatalf("N/A should map to empty: %+v", hits[1])
	}
}

func TestExtractMCPSearchHitsJSON(t *testing.T) {
	blocks := []string{`{"results": [{"title": "T", "url": "https://example.com"}]}`}
	hits, err := extractMCPSearchHits(blocks)
	if err != nil || len(hits) != 1 || hits[0].URL != "https://example.com" {
		t.Fatalf("got %+v, %v", hits, err)
	}
}

func TestParseMCPFetchText(t *testing.T) {
	fr := parseMCPFetchText("# Hello\nURL: https://example.com\n\nSome content here", "https://example.com", 5000)
	if fr == nil || fr.Title != "Hello" || fr.URL != "https://example.com" {
		t.Fatalf("unexpected: %+v", fr)
	}
	if fr.Content != "Some content here" {
		t.Fatalf("unexpected content: %q", fr.Content)
	}
}

func TestDecodeSSEEnvelope(t *testing.T) {
	env, err := decodeSSEEnvelope([]byte("event: message\ndata: {\"result\": {\"content\": [{\"type\": \"text\", \"text\": \"hi\"}]}}\n\n"))
	if err != nil || env == nil || env.Result == nil || len(env.Result.Content) != 1 {
		t.Fatalf("got %+v, %v", env, err)
	}
	bare, err := decodeSSEEnvelope([]byte(`{"result": {"content": []}}`))
	if err != nil || bare == nil || bare.Result == nil {
		t.Fatalf("bare JSON failed: %+v, %v", bare, err)
	}
}

func TestMCPURLKeylessAndKeyed(t *testing.T) {
	keyless := mcpURL(nil)
	if !strings.Contains(keyless, "tools=") || strings.Contains(keyless, "exaApiKey") {
		t.Fatalf("keyless URL wrong: %q", keyless)
	}
	key := "secret"
	keyed := mcpURL(&key)
	if !strings.Contains(keyed, "exaApiKey=secret") {
		t.Fatalf("keyed URL wrong: %q", keyed)
	}
}
