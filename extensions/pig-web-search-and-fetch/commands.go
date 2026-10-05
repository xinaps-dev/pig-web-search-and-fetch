package websearch

import (
	"fmt"
	"strings"

	sdk "github.com/MichaelKinsy/PiG/extensions/sdk"
)

// wsUsage and wsHelp mirror the TypeScript router text.
const wsUsage = "/ws"

var wsHelpText = strings.Join([]string{
	"Web Search and Fetch — usage:",
	"  /ws                            Open the interactive control hub",
	"  /ws status                     Show the detailed status report",
	"  /ws search on|off              Enable/disable web_search",
	"  /ws fetch on|off               Enable/disable web_fetch",
	"  /ws deep on|off                Enable/disable web_deep_search",
	"  /ws provider <tool> <id|none>  Assign a provider (tool: search|fetch|deep)",
	"  /ws provider                   Interactive provider assignment wizard",
	"  /ws config [providerId]        Configure a provider (e.g. Exa API key)",
	"  /ws help                       Show this help",
}, "\n")

// subcommand is one /ws action.
type subcommand struct {
	name        string
	description string
	category    string
	completions func(rest string) []sdk.AutocompleteItem
	handler     func(ctx sdk.Context, args []string) error
}

// router dispatches /ws subcommands and derives help/completion.
type wsRouter struct {
	subs []subcommand
}

func (r *wsRouter) add(s subcommand) {
	r.subs = append(r.subs, s)
}

func (r *wsRouter) lookup(name string) *subcommand {
	for i := range r.subs {
		if r.subs[i].name == name {
			return &r.subs[i]
		}
	}
	return nil
}

func (r *wsRouter) run(ctx sdk.Context, args string) error {
	fields := strings.Fields(args)
	if len(fields) == 0 {
		return handleWsHub(ctx)
	}
	if fields[0] == "help" || fields[0] == "--help" || fields[0] == "-h" {
		ctx.Notify(wsHelpText, "info")
		return nil
	}
	sub := r.lookup(strings.ToLower(fields[0]))
	if sub == nil {
		// search|fetch|deep are handled as toggle aliases for ergonomics.
		if toolKey, ok := parseToolKey(fields[0]); ok {
			return handleToggle(ctx, toolKey, fields[1:])
		}
		ctx.Notify(fmt.Sprintf("Unknown subcommand %q.\n\n%s", fields[0], wsHelpText), "error")
		return nil
	}
	return sub.handler(ctx, fields[1:])
}

func (r *wsRouter) complete(prefix string) ([]sdk.AutocompleteItem, error) {
	tokens := strings.Fields(prefix)
	trailingSpace := strings.HasSuffix(prefix, " ") && len(prefix) > 0
	if len(tokens) > 1 || (trailingSpace && len(tokens) == 1) {
		sub := r.lookup(tokens[0])
		if sub == nil || sub.completions == nil {
			// Toggle aliases complete on|off.
			if _, ok := parseToolKey(tokens[0]); ok {
				return completeOnOff(strings.Join(tokens[1:], " ")), nil
			}
			return nil, nil
		}
		rest := strings.Join(tokens[1:], " ")
		if trailingSpace {
			rest += " "
		}
		base := strings.Join(tokens, " ")
		if !trailingSpace {
			base = strings.Join(tokens[:len(tokens)-1], " ")
		}
		items := sub.completions(rest)
		for i := range items {
			items[i].Value = base + " " + items[i].Value
		}
		return items, nil
	}
	var items []sdk.AutocompleteItem
	for _, sub := range r.subs {
		if strings.HasPrefix(sub.name, prefix) {
			items = append(items, sdk.AutocompleteItem{Value: sub.name, Label: sub.name, Description: sub.description})
		}
	}
	for _, alias := range []string{"search", "fetch", "deep"} {
		if strings.HasPrefix(alias, prefix) {
			items = append(items, sdk.AutocompleteItem{Value: alias, Label: alias, Description: "Enable/disable " + alias})
		}
	}
	return items, nil
}

func completeOnOff(rest string) []sdk.AutocompleteItem {
	var out []sdk.AutocompleteItem
	for _, v := range []string{"on", "off"} {
		if strings.HasPrefix(v, strings.TrimSpace(rest)) {
			out = append(out, sdk.AutocompleteItem{Value: v, Label: v})
		}
	}
	return out
}

func parseToolKey(raw string) (string, bool) {
	switch strings.ToLower(raw) {
	case "search":
		return "search", true
	case "fetch":
		return "fetch", true
	case "deep", "deepsearch", "deep-search":
		return "deep", true
	}
	return "", false
}

func parseState(raw string) (bool, bool) {
	switch strings.ToLower(raw) {
	case "on", "true", "1":
		return true, true
	case "off", "false", "0":
		return false, true
	}
	return false, false
}

// wsSubcommands builds the /ws surface (status, provider, config + hub entry).
func wsSubcommands() []subcommand {
	return []subcommand{
		{name: "status", description: "Show the detailed status report", category: "General",
			handler: func(ctx sdk.Context, _ []string) error { return handleWsStatus(ctx) }},
		{name: "provider", description: "Assign a provider (provider <tool> <id|none>)", category: "Providers",
			completions: completeProvider,
			handler:     handleProvider},
		{name: "config", description: "Configure a provider (config [exa])", category: "Providers",
			completions: completeConfig,
			handler:     handleConfig},
	}
}

func completeProvider(rest string) []sdk.AutocompleteItem {
	fields := strings.Fields(rest)
	if len(fields) == 0 || (len(fields) == 1 && !strings.HasSuffix(rest, " ")) {
		prefix := ""
		if len(fields) == 1 {
			prefix = fields[0]
		}
		var out []sdk.AutocompleteItem
		for _, t := range []string{"search", "fetch", "deep"} {
			if strings.HasPrefix(t, prefix) {
				out = append(out, sdk.AutocompleteItem{Value: t, Label: t})
			}
		}
		return out
	}
	prefix := ""
	if len(fields) >= 2 {
		prefix = fields[1]
	}
	var out []sdk.AutocompleteItem
	for _, p := range []string{"exa", "none"} {
		if strings.HasPrefix(p, prefix) {
			out = append(out, sdk.AutocompleteItem{Value: p, Label: p})
		}
	}
	return out
}

func completeConfig(rest string) []sdk.AutocompleteItem {
	if strings.HasPrefix("exa", strings.TrimSpace(rest)) {
		return []sdk.AutocompleteItem{{Value: "exa", Label: "exa", Description: "Configure the Exa API key"}}
	}
	return nil
}

func handleToggle(ctx sdk.Context, toolKey string, rest []string) error {
	if len(rest) == 0 {
		ctx.Notify(fmt.Sprintf("Usage: %s %s on|off", wsUsage, toolKey), "error")
		return nil
	}
	enabled, ok := parseState(rest[0])
	if !ok {
		ctx.Notify(fmt.Sprintf("Usage: %s %s on|off", wsUsage, toolKey), "error")
		return nil
	}
	var patch configPatch
	var label string
	switch toolKey {
	case "search":
		patch = configPatch{Search: &WsToolConfig{Enabled: enabled, Provider: getConfig().Search.Provider}}
		label = toolSearch
	case "fetch":
		patch = configPatch{Fetch: &WsToolConfig{Enabled: enabled, Provider: getConfig().Fetch.Provider}}
		label = toolFetch
	default:
		patch = configPatch{DeepSearch: &WsToolConfig{Enabled: enabled, Provider: getConfig().DeepSearch.Provider}}
		label = toolDeep
	}
	// Preserve the provider when toggling: read-modify-write keeps the id.
	if _, err := updateConfig(patch); err != nil {
		ctx.Notify("Could not save the config: "+err.Error(), "error")
		return nil
	}
	syncActiveTools(ctx)
	if enabled {
		ctx.Notify(label+" enabled.", "info")
	} else {
		ctx.Notify(label+" disabled.", "info")
	}
	return nil
}

func handleProvider(ctx sdk.Context, args []string) error {
	if len(args) == 0 {
		return runProviderWizard(ctx)
	}
	toolKey, ok := parseToolKey(args[0])
	if !ok {
		ctx.Notify(fmt.Sprintf("Unknown tool %q. Use search|fetch|deep.", args[0]), "error")
		return nil
	}
	if len(args) < 2 {
		ctx.Notify(fmt.Sprintf("Usage: %s provider %s <id|none>", wsUsage, args[0]), "error")
		return nil
	}
	providerArg := strings.ToLower(args[1])
	if providerArg != "exa" && providerArg != "none" {
		ctx.Notify(fmt.Sprintf("Unknown provider %q for %s. Registered providers: exa, none.", args[1], toolKey), "error")
		return nil
	}
	cfg := getConfig()
	var patch configPatch
	var label string
	switch toolKey {
	case "search":
		label = toolSearch
		if providerArg == "none" {
			patch = configPatch{Search: &WsToolConfig{Enabled: false, Provider: cfg.Search.Provider}}
		} else {
			patch = configPatch{Search: &WsToolConfig{Enabled: true, Provider: "exa"}}
		}
	case "fetch":
		label = toolFetch
		if providerArg == "none" {
			patch = configPatch{Fetch: &WsToolConfig{Enabled: false, Provider: cfg.Fetch.Provider}}
		} else {
			patch = configPatch{Fetch: &WsToolConfig{Enabled: true, Provider: "exa"}}
		}
	default:
		label = toolDeep
		if providerArg == "none" {
			patch = configPatch{DeepSearch: &WsToolConfig{Enabled: false, Provider: cfg.DeepSearch.Provider}}
		} else {
			patch = configPatch{DeepSearch: &WsToolConfig{Enabled: true, Provider: "exa"}}
		}
	}
	if _, err := updateConfig(patch); err != nil {
		ctx.Notify("Could not save the config: "+err.Error(), "error")
		return nil
	}
	syncActiveTools(ctx)
	if providerArg == "none" {
		ctx.Notify(label+" disabled (provider kept).", "info")
	} else {
		ctx.Notify(label+" assigned to provider \"exa\".", "info")
	}
	return nil
}

func handleConfig(ctx sdk.Context, args []string) error {
	if len(args) == 0 {
		return runProviderConfigSelector(ctx)
	}
	if strings.ToLower(args[0]) != "exa" {
		ctx.Notify(fmt.Sprintf("Unknown provider %q. Known providers: exa.", args[0]), "error")
		return nil
	}
	return runExaConfigFlow(ctx)
}

// runProviderWizard is the interactive 3-step assignment: tool → provider.
func runProviderWizard(ctx sdk.Context) error {
	tool, ok, err := ctx.Select("Assign provider — tool", []string{"search (web_search)", "fetch (web_fetch)", "deep (web_deep_search)"})
	if err != nil || !ok {
		return nil
	}
	toolKey := "search"
	switch {
	case strings.HasPrefix(tool, "fetch"):
		toolKey = "fetch"
	case strings.HasPrefix(tool, "deep"):
		toolKey = "deep"
	}
	provider, ok, err := ctx.Select("Assign provider — provider for "+toolKey, []string{"exa", "none (disable tool)"})
	if err != nil || !ok {
		return nil
	}
	arg := "exa"
	if strings.HasPrefix(provider, "none") {
		arg = "none"
	}
	return handleProvider(ctx, []string{toolKey, arg})
}

// runProviderConfigSelector picks a provider to configure (only exa today).
func runProviderConfigSelector(ctx sdk.Context) error {
	provider, ok, err := ctx.Select("Configure provider", []string{"exa"})
	if err != nil || !ok {
		return nil
	}
	_ = provider
	return runExaConfigFlow(ctx)
}

// runExaConfigFlow mirrors the TypeScript Exa modal with blocking dialogs:
// use-key toggle, then an API-key input, persisted to auth.json (0600).
func runExaConfigFlow(ctx sdk.Context) error {
	cfg := getConfig()
	current := "No"
	if cfg.Providers.Exa.UseAPIKey {
		current = "Yes"
	}
	choice, ok, err := ctx.Select("Exa — Use API Key (current: "+current+")", []string{"Yes", "No"})
	if err != nil {
		return nil
	}
	if !ok {
		return nil
	}
	useKey := strings.EqualFold(choice, "Yes")
	if useKey {
		var stored string
		if cred := readStoredCredential(exaProviderKey); cred != nil {
			stored = cred.Key
		}
		hint := "sk-... (leave empty to keep the stored key)"
		if stored == "" {
			hint = "Paste your Exa API key (https://dashboard.exa.ai)"
		}
		input, ok, err := ctx.Input("Exa API Key", hint)
		if err != nil || !ok {
			return nil
		}
		key := strings.TrimSpace(input)
		if _, err := updateConfig(configPatch{UseAPIKey: &useKey}); err != nil {
			ctx.Notify("Could not save the config: "+err.Error(), "error")
			return nil
		}
		if key != "" {
			if err := writeExaAPIKey(key); err != nil {
				ctx.Notify("Could not save the API key: "+err.Error(), "error")
				return nil
			}
			ctx.Notify("Exa configured: API key saved to auth.json (0600).", "info")
		} else if stored != "" {
			ctx.Notify("Exa configured: kept the stored key.", "info")
		} else {
			ctx.Notify("Exa will use EXA_API_KEY when it is set.", "info")
		}
	} else {
		if _, err := updateConfig(configPatch{UseAPIKey: &useKey}); err != nil {
			ctx.Notify("Could not save the config: "+err.Error(), "error")
			return nil
		}
		if err := removeExaAPIKey(); err != nil {
			ctx.Notify("Could not remove the stored key: "+err.Error(), "error")
			return nil
		}
		ctx.Notify("Exa configured: public free mode (global limits). Search and fetch work without a key; deep research still needs one.", "info")
	}
	syncActiveTools(ctx)
	return nil
}

// handleWsHub is the interactive hub shown by plain /ws: a Select loop over
// the same actions the TypeScript dashboard offers.
func handleWsHub(ctx sdk.Context) error {
	for {
		if ctx.Err() != nil {
			return nil
		}
		cfg := getConfig()
		supp := shouldSuppressWebSearch(ctx.ModelProvider(), ctx.Model())
		statuses := hubToolStatuses(cfg, supp)
		lines := []string{"=== Web Search and Fetch — Control Panel ==="}
		for _, st := range statuses {
			lines = append(lines, renderToolLine(st))
		}
		ctx.Notify(strings.Join(lines, "\n"), "info")
		choice, ok, err := ctx.Select("Web Search and Fetch — action", []string{
			"Toggle search (web_search)",
			"Toggle fetch (web_fetch)",
			"Toggle deep search (web_deep_search)",
			"Assign providers (wizard)",
			"Configure Exa (API key / mode)",
			"View detailed status",
			"Exit",
		})
		if err != nil || !ok {
			return nil
		}
		switch {
		case strings.HasPrefix(choice, "Toggle search"):
			_ = handleToggle(ctx, "search", []string{onOff(!cfg.Search.Enabled)})
		case strings.HasPrefix(choice, "Toggle fetch"):
			_ = handleToggle(ctx, "fetch", []string{onOff(!cfg.Fetch.Enabled)})
		case strings.HasPrefix(choice, "Toggle deep"):
			_ = handleToggle(ctx, "deep", []string{onOff(!cfg.DeepSearch.Enabled)})
		case strings.HasPrefix(choice, "Assign"):
			if err := runProviderWizard(ctx); err != nil {
				return nil
			}
		case strings.HasPrefix(choice, "Configure"):
			if err := runExaConfigFlow(ctx); err != nil {
				return nil
			}
		case strings.HasPrefix(choice, "View"):
			_ = handleWsStatus(ctx)
		default:
			return nil
		}
	}
}

func onOff(enabled bool) string {
	if enabled {
		return "on"
	}
	return "off"
}
