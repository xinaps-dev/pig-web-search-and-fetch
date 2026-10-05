package websearch

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// Credential store layout. Keys live in the shared PiG credential store
// <agent-dir>/auth.json under the provider key with the standard format
// {"type": "api_key", "key": "..."}, so the extension stays interoperable
// with the TypeScript pi-web-search-and-fetch and pi-exa extensions.
const (
	authFileName   = "auth.json"
	exaProviderKey = "exa"
)

// storedCredential is one entry of auth.json.
type storedCredential struct {
	Type string `json:"type"`
	Key  string `json:"key"`
}

// authPath is the shared credential file.
func authPath() string {
	return filepath.Join(agentDir(), authFileName)
}

// readAuthStore reads the whole credential store. A missing, unreadable or
// malformed file reads as nil so callers fall back gracefully.
func readAuthStore() map[string]storedCredential {
	raw, err := os.ReadFile(authPath())
	if err != nil {
		return nil
	}
	var all map[string]storedCredential
	if err := json.Unmarshal(raw, &all); err != nil || all == nil {
		return nil
	}
	return all
}

// readStoredCredential returns the stored credential for a provider, or nil.
func readStoredCredential(provider string) *storedCredential {
	cred, ok := readAuthStore()[provider]
	if !ok || strings.TrimSpace(cred.Key) == "" {
		return nil
	}
	return &cred
}

// getExaAPIKey resolves the Exa API key:
//   - useAPIKey false → nil (public free mode requested, no key resolved)
//   - otherwise auth.json["exa"].key, then EXA_API_KEY.
//
// A nil return means no key is available.
func getExaAPIKey(useAPIKey bool) *string {
	if !useAPIKey {
		return nil
	}
	if stored := readStoredCredential(exaProviderKey); stored != nil {
		key := stored.Key
		return &key
	}
	if fromEnv := strings.TrimSpace(os.Getenv("EXA_API_KEY")); fromEnv != "" {
		return &fromEnv
	}
	return nil
}

// exaKeySource reports where the effective key comes from for status display.
func exaKeySource(useAPIKey bool) (key string, source string, ok bool) {
	if !useAPIKey {
		return "", "", false
	}
	if stored := readStoredCredential(exaProviderKey); stored != nil {
		return stored.Key, "auth.json", true
	}
	if fromEnv := strings.TrimSpace(os.Getenv("EXA_API_KEY")); fromEnv != "" {
		return fromEnv, "EXA_API_KEY (environment)", true
	}
	return "", "", false
}

// writeExaAPIKey stores the Exa key in auth.json (mode 0600), preserving
// every other provider entry.
func writeExaAPIKey(key string) error {
	all := readAuthStore()
	if all == nil {
		all = map[string]storedCredential{}
	}
	all[exaProviderKey] = storedCredential{Type: "api_key", Key: key}
	return writeAuthStore(all)
}

// removeExaAPIKey removes the Exa entry, preserving other providers.
func removeExaAPIKey() error {
	all := readAuthStore()
	if all == nil {
		return nil
	}
	if _, ok := all[exaProviderKey]; !ok {
		return nil
	}
	delete(all, exaProviderKey)
	return writeAuthStore(all)
}

// writeAuthStore replaces the credential file, owner-readable only. An empty
// store is written as {} so the file stays valid JSON.
func writeAuthStore(all map[string]storedCredential) error {
	if all == nil {
		all = map[string]storedCredential{}
	}
	raw, err := json.MarshalIndent(all, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	return writeFileAtomic(authPath(), raw, 0o600)
}

// maskKey keeps the first 4 and last 4 characters for status display.
func maskKey(key string) string {
	if len(key) <= 8 {
		return strings.Repeat("•", len(key))
	}
	return key[:4] + "••••" + key[len(key)-4:]
}
