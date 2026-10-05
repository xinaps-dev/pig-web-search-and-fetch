# 🐷 pig-web-search-and-fetch

[![License: MIT](https://img.shields.io/badge/License-MIT-green.svg)](https://opensource.org/licenses/MIT)
[![pig Extension](https://img.shields.io/badge/pig-extension-purple.svg)](https://pi-in-go.dev/docs/latest/extensions/)

> **Give your [PiG](https://pi-in-go.dev) coding agent real-time web intelligence: neural search, clean Markdown fetch and deep research.** 🐽

`pig-web-search-and-fetch` brings live web tools to the **PiG** coding agent —
`web_search`, `web_fetch` and `web_deep_search` — powered by [Exa](https://exa.ai),
with a unified `/ws` control command and smart coordination with Requesty's
native search.

> 🐖 **Go port of [`pi-web-search-and-fetch`](https://github.com/xinaps-dev/pi-web-search-and-fetch)**
> (TypeScript, Pi) into a native PiG extension. Same tool surface, same `/ws`
> ergonomics — but compiled by PiG into a *runtime cell*, with no Node runtime
> and no `node_modules`.

---

## ⚡ Superpowers & Features

- 🔍 **Real-Time Neural Web Search (`web_search`)** — Semantic search with
  category, domain and date filters, plus transparent `similarUrl` discovery.
- 📄 **High-Fidelity Web Fetch (`web_fetch`)** — Clean Markdown extraction for
  one or many URLs in a single call, with per-page isolation blocks.
- 🧠 **Deep Research (`web_deep_search`)** — Synthesized answers with citations
  via Exa `/answer`, with automatic fallback to parallel multi-query synthesis.
- 🎛️ **Unified `/ws` Command** — Status report, `on|off` toggles, provider
  assignment wizard, Exa key configuration and an interactive hub loop.
- 🤝 **Requesty Synergy** — Reads the shared `pi-requesty.json` switch and the
  `requesty-models.json` catalog cache: when Requesty native search is active
  for a compatible non-Gemini model, `web_search` is suppressed to avoid
  duplication. `web_fetch` always stays on.
- 🔒 **Secure Credentials** — Exa key in `<agent-dir>/auth.json` (`0600`),
  preserving other providers' entries, with `EXA_API_KEY` fallback.

---

## 🚀 Quick Installation

### From npm

```bash
pig install npm:pig-web-search-and-fetch
```

> Until the package is published, install from GitHub or locally.

### From GitHub

```bash
pig install git:github.com/xinaps-dev/pig-web-search-and-fetch
```

### Local Development

```bash
git clone https://github.com/xinaps-dev/pig-web-search-and-fetch.git
cd pig-web-search-and-fetch

# Install the Package root (not the extension directory)
pig install .
```

Requirements: PiG 0.4.0 or later (`pig version`) and Go 1.27.1 to build from
source (`pig setup go`). PiG compiles the extension into a *runtime cell* on
first use and loads it from cache afterwards.

---

## 🎯 Getting Started

### 1. Store your Exa key

Inside a `pig` session:

```
/ws config exa
```

1. Choose `Yes` for *Use API Key*.
2. Paste your key (get one at [dashboard.exa.ai](https://dashboard.exa.ai)).

The key is saved to `<agent-dir>/auth.json` (`0600`). Alternatively, export
`EXA_API_KEY` — the stored key wins when both exist. Without any key, search
and fetch keep working through Exa's public endpoint under global limits
(zero-config, as in the original); deep research always needs a key.

### 2. Toggle tools

```
/ws status
/ws deep on
/ws search off
```

`web_search` and `web_fetch` are enabled by default, `web_deep_search` is
opt-in — exactly like the TypeScript original.

### 3. Ask the web

With an Exa key configured, just ask. The model calls the tools directly; use
`/ws status` to confirm which tools the next turn sees.

---

## 🛠️ Command Reference

`/ws` ships with autocomplete. Without arguments it opens the interactive hub
(a `Select` loop with the same actions as the text commands).

| Command | Description |
|---|---|
| `/ws` | Interactive control hub (toggle, assign, configure, status) |
| `/ws status` | Detailed report: tools, Exa credential source, Requesty state |
| `/ws search on\|off` | Enable/disable `web_search` |
| `/ws fetch on\|off` | Enable/disable `web_fetch` |
| `/ws deep on\|off` | Enable/disable `web_deep_search` |
| `/ws provider <tool> <id\|none>` | Assign a provider (`tool`: search\|fetch\|deep; `none` disables, keeping the id) |
| `/ws provider` | Interactive 3-step assignment wizard |
| `/ws config [exa]` | Configure the Exa API key / mode |
| `/ws help` | Usage help |

Only `exa` is registered today; `none` disables the tool while remembering
its provider id.

---

## 🧰 Tools

| Tool | Params | Behaviour |
|---|---|---|
| `web_search` | `query` (required), `numResults`, `category`, `includeDomains`, `excludeDomains`, `startPublishedDate`, `endPublishedDate`, `similarUrl` | Exa `/search` (or `/findSimilar` when `similarUrl` is set). Output is wrapped in `<web_content>` isolation blocks with a security notice. |
| `web_fetch` | `urls` (string or string[]), `url` (alias), `maxCharacters` (default 5000) | Exa `/contents` with Markdown truncation at natural boundaries. Batch failures degrade per URL. |
| `web_deep_search` | `query` (required), `numSources` (default 5), `includeText` (default true), `numResults`, `category`, `additionalQueries` | Exa `/answer` first; on failure, parallel `/search` synthesis with URL dedup. Disabled by default. |

---

## ⚙️ Environment Variables

| Variable | Description | Default |
|---|---|---|
| `EXA_API_KEY` | Exa API key fallback when no stored credential exists | _(none)_ |

Credential precedence: **`auth.json["exa"]` → `EXA_API_KEY`**. When
`providers.exa.useApiKey` is `false`, no key is resolved and the tools report
how to re-enable it.

### Files the extension owns

All inside PiG's agent dir (`~/.pig/agent` by default, or whatever
`PIG_CODING_AGENT_DIR` sets):

| File | Contents |
|---|---|
| `pi-web-search-and-fetch.json` | Tool toggles (`search`/`fetch`/`deepSearch`) and `providers.exa.useApiKey`. Same name as the TypeScript extension, so configs carry over. |
| `auth.json` | The `exa` credential (`{"type":"api_key","key":"..."}`). Shared with PiG; other providers' entries are preserved. Mode `0600`. |

### Files the extension reads (Requesty compatibility)

| File | Contents |
|---|---|
| `pi-requesty.json` | The **shared** Requesty switch (`nativeSearch`). Same file used by `pi-requesty-provider` and `pig-requesty-provider` — install either one and this extension stays compatible. |
| `requesty-models.json` | Requesty catalog cache, consulted for `supports_web_search` before suppressing `web_search`. Absent/unknown ⇒ no suppression (safe default). |

---

## 🔄 Porting notes

Differences from the TypeScript `pi-web-search-and-fetch` that are
intentional in this Go port:

- **Exa via REST with key, MCP without.** With an API key the port calls
  `https://api.exa.ai/{search,findSimilar,contents,answer}` directly, staying
  stateless inside PiG runtime cells (no persistent connection, nothing to
  close on `session_shutdown`). Without a key, search and fetch fall back to
  the public `https://mcp.exa.ai/mcp` endpoint — the same server the
  TypeScript original uses — so zero-config mode keeps working under global
  limits. Deep research requires a key in both implementations.
- **Hub without custom TUI.** The original dashboard is a `ui.custom`
  component; this port implements `/ws` as text subcommands plus a blocking
  `Select` loop with the same actions (toggle, wizard, configure, status).
- **Suppression via catalog cache.** The original inspects the live model
  object for `supportsWebSearch`; the Go SDK's `ModelInfo` has no such field,
  so this port reads the cached `requesty-models.json` written by either
  Requesty extension. Behaviour is otherwise identical: suppress only on
  Requesty provider + `nativeSearch: true` + non-Gemini + catalog support.

---

## ✅ Verification

```bash
# Manifest + registration + unit tests + /ws surface
scripts/verify.sh

# Unit tests only
cd extensions/pig-web-search-and-fetch && go test ./...
```

`scripts/verify.sh` uses a throwaway agent dir and never touches the real
`~/.pig` configuration.

---

## 📄 License

MIT — see [LICENSE](./LICENSE).
