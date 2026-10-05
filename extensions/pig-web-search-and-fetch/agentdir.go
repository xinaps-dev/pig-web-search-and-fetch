package websearch

import (
	"os"
	"path/filepath"
)

// agentDir returns PiG's agent directory.
//
// Context.ConfigHome reports the configuration root and ignores
// PIG_CODING_AGENT_DIR, so the extension resolves the agent directory itself
// with the precedence PiG documents. This mirrors pig-requesty-provider so
// both extensions share auth.json, pi-requesty.json and the catalog cache.
func agentDir() string {
	if dir := os.Getenv("PIG_CODING_AGENT_DIR"); dir != "" {
		return dir
	}
	if home := os.Getenv("PIG_HOME"); home != "" {
		return filepath.Join(home, "agent")
	}
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "pig", "agent")
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return filepath.Join(".pig", "agent")
	}
	return filepath.Join(home, ".pig", "agent")
}
