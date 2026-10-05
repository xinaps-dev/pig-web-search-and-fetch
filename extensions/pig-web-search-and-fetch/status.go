package websearch

import (
	"fmt"
	"strings"

	sdk "github.com/MichaelKinsy/PiG/extensions/sdk"
)

// toolStatus is one tool snapshot for status and hub displays.
type toolStatus struct {
	toolID     string
	enabled    bool
	providerID string
	suppressed bool
	reason     string
}

// hubToolStatuses mirrors the TypeScript buildHubToolStatuses: the three
// tools in stable order with the Requesty suppression applied to search.
func hubToolStatuses(cfg Config, supp suppression) []toolStatus {
	searchSuppressed := supp.shouldSuppress
	return []toolStatus{
		{toolID: toolSearch, enabled: cfg.Search.Enabled && !searchSuppressed, providerID: cfg.Search.Provider, suppressed: searchSuppressed, reason: supp.reason},
		{toolID: toolFetch, enabled: cfg.Fetch.Enabled, providerID: cfg.Fetch.Provider},
		{toolID: toolDeep, enabled: cfg.DeepSearch.Enabled, providerID: cfg.DeepSearch.Provider},
	}
}

func toolLabel(id string) string {
	switch id {
	case toolSearch:
		return "Search (web_search)"
	case toolFetch:
		return "Fetch (web_fetch)"
	case toolDeep:
		return "Deep Search (web_deep_search)"
	default:
		return id
	}
}

func renderToolLine(st toolStatus) string {
	box, state := "[ ]", "OFF"
	if st.enabled {
		box, state = "[✓]", "ON"
	}
	line := fmt.Sprintf("  %s %s : %s (Provider: %s)", box, toolLabel(st.toolID), state, st.providerID)
	if st.suppressed {
		line += " (suppressed"
		if st.reason != "" {
			line += ": " + st.reason
		}
		line += ")"
	}
	return line
}

// buildStatusReport assembles the /ws status text.
func buildStatusReport(modelProvider, modelID string) string {
	cfg := getConfig()
	supp := shouldSuppressWebSearch(modelProvider, modelID)
	nativeEnabled := isRequestyNativeSearchEnabled()

	toolLines := []string{}
	for _, st := range hubToolStatuses(cfg, supp) {
		toolLines = append(toolLines, renderToolLine(st))
	}

	var credentialsLine string
	if !cfg.Providers.Exa.UseAPIKey {
		credentialsLine = "  Exa: public mode without API Key (useApiKey: No, global limits)"
	} else if key, source, ok := exaKeySource(true); ok {
		credentialsLine = fmt.Sprintf("  Exa: API Key detected in %s (%s)", source, maskKey(key))
	} else {
		credentialsLine = "  Exa: without API Key (free public mode, global limits)"
	}

	nativeLine := "  nativeSearch: disabled"
	if nativeEnabled {
		nativeLine = "  nativeSearch: enabled"
	}
	suppLine := "  web_search: not suppressed"
	if supp.shouldSuppress {
		suppLine = "  web_search: suppressed"
	}
	if supp.reason != "" {
		suppLine += " — " + supp.reason
	} else if !supp.shouldSuppress {
		suppLine += " — " + supp.reason
	}

	return strings.Join(append([]string{
		"🌐 Web Search and Fetch — Current Status",
		"",
		"Tools:",
	}, append(toolLines, "", "Credentials:", credentialsLine, "", "pi-requesty:", nativeLine, suppLine)...), "\n")
}

// handleWsStatus delivers the status report as a notification.
func handleWsStatus(ctx sdk.Context) error {
	ctx.Notify(buildStatusReport(ctx.ModelProvider(), ctx.Model()), "info")
	return nil
}
