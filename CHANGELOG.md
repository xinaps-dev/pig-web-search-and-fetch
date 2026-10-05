# Changelog

All notable changes to this project are documented here.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.0] - 2026-10-05

### Added

- Go extension `pig-web-search-and-fetch`: `web_search`, `web_fetch` and
  `web_deep_search` tools for PiG via the Exa REST API (keyed) with a public
  MCP fallback (keyless) for search and fetch, plus a unified `/ws` command
  and `session_start` / `model_select` / `before_agent_start`
  synchronization.
- `/ws` command surface: `status`, `search|fetch|deep on|off`,
  `provider <tool> <id|none>` with a 3-step wizard, `config [exa]` and `help`,
  with argument completion and an interactive hub loop built on blocking
  `Select` dialogs.
- Exa credential flow: `auth.json["exa"]` (mode `0600`) then `EXA_API_KEY`,
  toggled by `providers.exa.useApiKey`; deep research always requires a key.
- `pi-requesty` compatibility: reads the shared `<agent-dir>/pi-requesty.json`
  (`nativeSearch`) and the `requesty-models.json` catalog cache, suppressing
  `web_search` only for Requesty non-Gemini models with `supports_web_search`,
  so the TypeScript `pi-requesty-provider` and `pig-requesty-provider` stay
  interchangeable.
- `package.json` Package manifest exposing the extension through
  `pi.extensions`.
- `scripts/verify.sh`: manifest validation, `go test`, `/ws` command surface
  and config-persistence checks against a throwaway agent dir.
