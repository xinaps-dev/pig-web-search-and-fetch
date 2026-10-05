package websearch

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Config file name of this extension: <agent-dir>/pi-web-search-and-fetch.json.
// The name intentionally keeps the "pi-" prefix so a config written by the
// TypeScript pi-web-search-and-fetch extension is reused as-is.
const configFileName = "pi-web-search-and-fetch.json"

// WsToolConfig is one per-tool config section (search, fetch, deepSearch).
type WsToolConfig struct {
	Enabled  bool   `json:"enabled"`
	Provider string `json:"provider"`
}

// ExaProviderConfig holds the Exa-specific options. useApiKey false means
// public free mode is requested (no key is resolved).
type ExaProviderConfig struct {
	UseAPIKey bool `json:"useApiKey"`
}

// ProvidersConfig groups provider-specific options.
type ProvidersConfig struct {
	Exa ExaProviderConfig `json:"exa"`
}

// Config is the root structure of pi-web-search-and-fetch.json.
type Config struct {
	Search     WsToolConfig   `json:"search"`
	Fetch      WsToolConfig   `json:"fetch"`
	DeepSearch WsToolConfig   `json:"deepSearch"`
	Providers  ProvidersConfig `json:"providers"`
}

// defaultConfig mirrors the TypeScript DEFAULT_CONFIG: search and fetch
// enabled with exa, deep search disabled, Exa key usage on.
func defaultConfig() Config {
	return Config{
		Search:     WsToolConfig{Enabled: true, Provider: "exa"},
		Fetch:      WsToolConfig{Enabled: true, Provider: "exa"},
		DeepSearch: WsToolConfig{Enabled: false, Provider: "exa"},
		Providers:  ProvidersConfig{Exa: ExaProviderConfig{UseAPIKey: true}},
	}
}

// configPath is the full path of the extension config file.
func configPath() string {
	return filepath.Join(agentDir(), configFileName)
}

// mergeConfig overlays a partial decoded map over base, section by section,
// so partial or missing files always yield a complete Config.
func mergeConfig(base Config, partial map[string]any) Config {
	out := base
	if sec, ok := partial["search"].(map[string]any); ok {
		if v, ok := sec["enabled"].(bool); ok {
			out.Search.Enabled = v
		}
		if v, ok := sec["provider"].(string); ok && v != "" {
			out.Search.Provider = v
		}
	}
	if sec, ok := partial["fetch"].(map[string]any); ok {
		if v, ok := sec["enabled"].(bool); ok {
			out.Fetch.Enabled = v
		}
		if v, ok := sec["provider"].(string); ok && v != "" {
			out.Fetch.Provider = v
		}
	}
	if sec, ok := partial["deepSearch"].(map[string]any); ok {
		if v, ok := sec["enabled"].(bool); ok {
			out.DeepSearch.Enabled = v
		}
		if v, ok := sec["provider"].(string); ok && v != "" {
			out.DeepSearch.Provider = v
		}
	}
	if prov, ok := partial["providers"].(map[string]any); ok {
		if exa, ok := prov["exa"].(map[string]any); ok {
			if v, ok := exa["useApiKey"].(bool); ok {
				out.Providers.Exa.UseAPIKey = v
			}
		}
	}
	return out
}

// getConfig reads the on-disk config merged over defaults. A missing,
// unreadable or malformed file yields defaults, never an error.
func getConfig() Config {
	raw, err := os.ReadFile(configPath())
	if err != nil {
		return defaultConfig()
	}
	var parsed map[string]any
	if err := json.Unmarshal(raw, &parsed); err != nil || parsed == nil {
		return defaultConfig()
	}
	return mergeConfig(defaultConfig(), parsed)
}

// configPatch is a sparse update: nil sections are left untouched.
type configPatch struct {
	Search     *WsToolConfig
	Fetch      *WsToolConfig
	DeepSearch *WsToolConfig
	UseAPIKey  *bool
}

// updateConfig merges the patch over the current on-disk state (or defaults)
// and persists it atomically (write-to-temp + rename).
func updateConfig(patch configPatch) (Config, error) {
	current := getConfig()
	if patch.Search != nil {
		current.Search = *patch.Search
	}
	if patch.Fetch != nil {
		current.Fetch = *patch.Fetch
	}
	if patch.DeepSearch != nil {
		current.DeepSearch = *patch.DeepSearch
	}
	if patch.UseAPIKey != nil {
		current.Providers.Exa.UseAPIKey = *patch.UseAPIKey
	}
	raw, err := json.MarshalIndent(current, "", "  ")
	if err != nil {
		return current, err
	}
	raw = append(raw, '\n')
	if err := writeFileAtomic(configPath(), raw, 0o600); err != nil {
		return current, err
	}
	return current, nil
}

// writeFileAtomic writes data through a temp file in the destination
// directory so an interrupted write cannot truncate the previous content.
func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Chmod(tmpName, perm); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName)
		return err
	}
	return nil
}
