// Package websearch implements pig-web-search-and-fetch: real-time web
// search, clean Markdown fetch and deep research for PiG via Exa, with a
// unified /ws command and pi-requesty compatibility.
//
// The provider is reached through Exa's REST API (https://api.exa.ai), so
// the extension is stateless: no MCP connection is kept across calls. Tool
// activation is synchronized through setActiveTools on session_start,
// model_select and before_agent_start, mirroring the TypeScript original.
package websearch

// Extension identity. It must match the extension directory name for runtime
// registration to be accepted.
const extensionID = "pig-web-search-and-fetch"

// Tool identifiers exposed to the LLM, keyed by config section.
const (
	toolSearch = "web_search"
	toolFetch  = "web_fetch"
	toolDeep   = "web_deep_search"
)
