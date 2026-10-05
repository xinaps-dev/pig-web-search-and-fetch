package websearch

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// Compatibility with pi-requesty / pig-requesty-provider.
//
// Both Requesty extensions share one common file, <agent-dir>/pi-requesty.json
// with {"nativeSearch": bool}. When Requesty's native server-side search is
// active for a compatible non-Gemini model with web-search support, this
// extension suppresses web_search to avoid duplication. web_fetch is always
// kept: Requesty never fetches full pages.
//
// The Go port reads the same file name from the PiG agent dir, so installing
// the TypeScript pi-requesty-provider under PiG stays compatible.
const (
	requestyConfigFileName  = "pi-requesty.json"
	requestyCatalogFileName = "requesty-models.json"
	requestyProviderID      = "requesty"
)

// suppression is the outcome of the compatibility evaluation.
type suppression struct {
	shouldSuppress bool
	reason         string
}

// requestyConfigPath is <agent-dir>/pi-requesty.json.
func requestyConfigPath() string {
	return filepath.Join(agentDir(), requestyConfigFileName)
}

// requestyCatalogPath is <agent-dir>/requesty-models.json, the catalog cache
// written by pig-requesty-provider (and pi-requesty-provider).
func requestyCatalogPath() string {
	return filepath.Join(agentDir(), requestyCatalogFileName)
}

// isRequestyNativeSearchEnabled reports the nativeSearch flag. A missing,
// unreadable or malformed file means disabled, never an error.
func isRequestyNativeSearchEnabled() bool {
	raw, err := os.ReadFile(requestyConfigPath())
	if err != nil {
		return false
	}
	var parsed map[string]any
	if err := json.Unmarshal(raw, &parsed); err != nil || parsed == nil {
		return false
	}
	enabled, _ := parsed["nativeSearch"].(bool)
	return enabled
}

// isGeminiModel reports whether a model id belongs to Gemini. Gemini rejects
// requests combining its built-in search with function calling, so native
// search is never preferred for it.
func isGeminiModel(modelID string) bool {
	return strings.Contains(strings.ToLower(modelID), "gemini")
}

// catalogSupportsWebSearch reports whether the cached Requesty catalog marks
// modelID as supporting native web search. An absent or unreadable cache, or
// an unknown model, yields false: suppression requires positive evidence,
// mirroring the TypeScript integration (model.supportsWebSearch === true).
func catalogSupportsWebSearch(modelID string) bool {
	if modelID == "" {
		return false
	}
	raw, err := os.ReadFile(requestyCatalogPath())
	if err != nil {
		return false
	}
	// The cache is either a bare array (pig-requesty-provider) or the
	// {"data": [...]} envelope (Requesty API shape); accept both.
	var envelope struct {
		Data []map[string]any `json:"data"`
	}
	var list []map[string]any
	if err := json.Unmarshal(raw, &list); err != nil || list == nil {
		if err := json.Unmarshal(raw, &envelope); err != nil {
			return false
		}
		list = envelope.Data
	}
	for _, entry := range list {
		id, _ := entry["id"].(string)
		if id != modelID {
			continue
		}
		supports, _ := entry["supports_web_search"].(bool)
		return supports
	}
	return false
}

// shouldSuppressWebSearch evaluates the compatibility rules for the active
// model. Suppression happens only when ALL hold:
//  1. provider is requesty,
//  2. pi-requesty.json has nativeSearch true,
//  3. the model is not a Gemini model,
//  4. the cached catalog marks the model with supports_web_search true.
//
// Every other case keeps web_search active per the local config.
func shouldSuppressWebSearch(modelProvider, modelID string) suppression {
	if !strings.EqualFold(strings.TrimSpace(modelProvider), requestyProviderID) {
		if strings.TrimSpace(modelProvider) == "" {
			return suppression{reason: "current model is not a Requesty model"}
		}
		return suppression{reason: "current model provider is not requesty"}
	}
	if !isRequestyNativeSearchEnabled() {
		return suppression{reason: "pi-requesty nativeSearch is disabled"}
	}
	if isGeminiModel(modelID) {
		return suppression{reason: "Gemini models do not support combining function calling with native search"}
	}
	if !catalogSupportsWebSearch(modelID) {
		return suppression{reason: "current model does not support native web search"}
	}
	return suppression{
		shouldSuppress: true,
		reason:         "Requesty native search is active for the current model; web_search is suppressed to avoid duplication",
	}
}
