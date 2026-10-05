package websearch

import (
	sdk "github.com/MichaelKinsy/PiG/extensions/sdk"
)

// syncActiveTools recomputes which extension tools the model sees next turn
// and applies the list through SetActiveTools:
//
//   - a tool is active when its config section has enabled true;
//   - web_search is additionally suppressed by the pi-requesty compatibility
//     rules (nativeSearch + compatible non-Gemini model with web support);
//   - web_fetch is always kept when enabled: Requesty never fetches pages;
//   - non-extension tools (PiG built-ins, other extensions) are preserved.
func syncActiveTools(ctx sdk.Context) {
	cfg := getConfig()
	supp := shouldSuppressWebSearch(ctx.ModelProvider(), ctx.Model())

	var active []string
	if cfg.Search.Enabled && !supp.shouldSuppress {
		active = append(active, toolSearch)
	}
	if cfg.Fetch.Enabled {
		active = append(active, toolFetch)
	}
	if cfg.DeepSearch.Enabled {
		active = append(active, toolDeep)
	}

	ours := map[string]bool{toolSearch: true, toolFetch: true, toolDeep: true}
	current, err := ctx.GetActiveTools()
	if err != nil || current == nil {
		ctx.SetActiveTools(active)
		return
	}
	merged := make([]string, 0, len(current)+len(active))
	for _, name := range current {
		if !ours[name] {
			merged = append(merged, name)
		}
	}
	merged = append(merged, active...)
	ctx.SetActiveTools(merged)
}

// syncEvent is the shared handler for session_start, model_select and
// before_agent_start: re-evaluate the config + Requesty suppression.
func syncEvent(ctx sdk.Context, _ map[string]any) (any, error) {
	syncActiveTools(ctx)
	return nil, nil
}
