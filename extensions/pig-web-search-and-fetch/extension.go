package websearch

import (
	sdk "github.com/MichaelKinsy/PiG/extensions/sdk"
)

// Extension is the factory PiG loads.
func Extension() *sdk.Extension {
	ext := sdk.New(extensionID)

	ext.RegisterTool(sdk.ToolDefinition{
		Name:             toolSearch,
		Label:            toolSearch,
		Description:      searchDescription,
		Parameters:       webSearchSchema(),
		PromptSnippet:    searchSnippet,
		PromptGuidelines: searchGuidelines,
		Execute:          webSearchHandler,
	})
	ext.RegisterTool(sdk.ToolDefinition{
		Name:             toolFetch,
		Label:            toolFetch,
		Description:      fetchDescription,
		Parameters:       webFetchSchema(),
		PromptSnippet:    fetchSnippet,
		PromptGuidelines: fetchGuidelines,
		Execute:          webFetchHandler,
	})
	ext.RegisterTool(sdk.ToolDefinition{
		Name:             toolDeep,
		Label:            toolDeep,
		Description:      deepDescription,
		Parameters:       webDeepSearchSchema(),
		PromptSnippet:    deepSnippet,
		PromptGuidelines: deepGuidelines,
		Execute:          webDeepSearchHandler,
	})

	r := &wsRouter{}
	for _, sub := range wsSubcommands() {
		r.add(sub)
	}
	ext.RegisterCommand("ws", sdk.CommandOptions{
		Description:            "Open the Web Search and Fetch control panel and provider settings",
		GetArgumentCompletions: r.complete,
		Handler:                r.run,
	})

	// Keep the active tool set in sync with the config and the Requesty
	// compatibility rules, mirroring the TypeScript lifecycle listeners.
	ext.OnEvent(sdk.EventSessionStart, syncEvent)
	ext.OnEvent(sdk.EventModelSelect, syncEvent)
	ext.OnEvent(sdk.EventBeforeAgentStart, syncEvent)

	return ext
}
