# apic — Complete Documentation

`apic` is a terminal client for any OpenAPI / Swagger service. Point it at a
spec, it lists every endpoint, builds an input form from each endpoint's
declared parameters, and sends real requests — all without leaving the
terminal.

- **Reads everything:** OpenAPI 3.0, OpenAPI 3.1, and Swagger 2.0, in **JSON or
  YAML**, from a **local file or an http(s) URL**.
- **Interactive:** filterable endpoint list, auto-generated forms, pretty-printed
  responses.
- **Scriptable:** `--list` prints endpoints and exits.
- **Production-minded:** bounded timeouts, capped response sizes, TLS
  verification on by default, credential-safe redirects, and no chasing of
  external `$ref`s over the network.

---

## Table of contents

1. [Install](#1-install)
2. [Quick start](#2-quick-start)
3. [Supported spec formats](#3-supported-spec-formats)
4. [Command-line reference](#4-command-line-reference)
5. [Choosing the server (`--server`)](#5-choosing-the-server---server)
6. [Authentication and global headers](#6-authentication-and-global-headers)
7. [Getting the spec out of a Swagger UI page](#7-getting-the-spec-out-of-a-swagger-ui-page)
8. [The interactive UI](#8-the-interactive-ui)
9. [Saving and reading large responses](#9-saving-and-reading-large-responses)
10. [Use-case recipes](#10-use-case-recipes)
11. [Security model](#11-security-model)
12. [Architecture](#12-architecture)
13. [Troubleshooting](#13-troubleshooting)
14. [Development](#14-development)

---

## 1. Install

**One line (recommended)** — downloads the right prebuilt binary for your OS and
architecture from the latest GitHub release. No Go toolchain required:

```bash
curl -fsSL https://raw.githubusercontent.com/yash5155/apic/main/install.sh | bash
```

It installs to `~/.local/bin` by default. Override with `APIC_INSTALL_DIR`, or
pin a version with `APIC_VERSION=v0.1.0`. If the install dir isn't on your PATH,
the script prints the exact line to add it.

**With Go:**

```bash
go install github.com/yash5155/apic@latest   # installs to ~/go/bin/apic
```

If `~/go/bin` is not on your `PATH`:

```bash
echo 'export PATH=$PATH:$HOME/go/bin' >> ~/.bashrc && source ~/.bashrc
```

**From source** (requires Go 1.22+):

```bash
git clone https://github.com/yash5155/apic && cd apic
go build -o apic .
go run . testdata/petstore.json   # or run without building
```

---

## 2. Quick start

```bash
# Local spec, use the server declared in the spec
apic openapi.json

# Remote spec on a live public API
apic https://petstore3.swagger.io/api/v3/openapi.json

# Just print the endpoints, no UI
apic openapi.json --list

# Real-world: a Swagger 2.0 YAML spec behind bearer auth
apic swagger.yaml --server https://api.example.com -H "Authorization: Bearer $TOKEN"
```

---

## 3. Supported spec formats

| Format | Supported | Notes |
|---|---|---|
| OpenAPI 3.0 | ✅ | Native. |
| OpenAPI 3.1 | ✅ | Parsed by the same loader; the common fields used here (paths, params, schemas) work. |
| Swagger 2.0 | ✅ | Auto-detected and converted to OpenAPI 3 in memory. `host` + `basePath` + `schemes` become the server URL. |
| JSON | ✅ | Local or remote. |
| YAML | ✅ | Local or remote; detected automatically. |
| Local file | ✅ | Relative external `$ref`s to sibling files resolve. |
| http(s) URL | ✅ | Fetched with a timeout and size cap. External `$ref`s are **not** followed over the network (SSRF-safe). |

The spec flavour is detected from the document itself — you never have to tell
`apic` which version or format you have. If a document has no recognisable
`openapi` or `swagger` version field, `apic` still attempts to parse it and
reports a clear error if it cannot.

**How Swagger 2.0 conversion works:** when `apic` sees `"swagger": "2.0"`, it
converts the document to OpenAPI 3 before doing anything else. Parameters,
enums, defaults, `$ref` bodies, and path-level parameters all carry across. You
do **not** need `swagger2openapi` or any external tool.

---

## 4. Command-line reference

```
apic <spec-file-or-url> [flags]
```

| Flag | Default | Description |
|---|---|---|
| `--server <url>` | first server in spec | Base URL requests are sent to. |
| `--list` | off | Print endpoints and exit; no TUI. |
| `-H, --header "Name: value"` | none | Header added to **every** request. Repeatable. |
| `--timeout <duration>` | `30s` | Per-request timeout, and the timeout for fetching a remote spec. Accepts Go durations: `500ms`, `10s`, `2m`. |
| `-k, --insecure` | off | Skip TLS certificate verification. Use only for trusted self-signed hosts. |
| `--max-body <MB>` | `2` | Maximum response body read into memory, in megabytes. Bodies larger than this are shown truncated and flagged. |
| `-v, --version` | — | Print version and exit. |
| `-h, --help` | — | Show help. |

### Exit codes

- `0` — success.
- `1` — any error (spec failed to load, no endpoints, no server, bad flag).

---

## 5. Choosing the server (`--server`)

The base URL is resolved in this order:

1. `--server` if you pass it.
2. Otherwise, the **first** `servers` entry in the spec.
3. If neither exists, `apic` exits and asks you to pass `--server`.

**Why you often need `--server`.** Specs frequently declare a placeholder or
templated server such as `{scheme}://{host}/{basePath}`, or a server for a
different environment than the one you want. In those cases pass the real base
URL yourself.

**The double-prefix trap.** The server URL and the spec's paths are
concatenated. If the spec's paths already start with a prefix (e.g.
`/content/v1/...`) and you also put that prefix in `--server`
(`https://host/content`), you get `/content/content/...` and a 404. Rule of
thumb: if the paths already include the prefix, set `--server` to the host
only.

```bash
# paths are /content/v1/... in the spec  → host only
apic spec.json --server https://myvurt.com

# paths are /v1/... in the spec          → include the prefix
apic spec.json --server https://myvurt.com/content
```

Verify quickly with `--list`: the header line shows exactly what base URL will
be used.

---

## 6. Authentication and global headers

Most real APIs require an auth token on every call. Pass it once with `-H` and
it is attached to every request:

```bash
apic openapi.json --server https://api.example.com \
  -H "Authorization: Bearer eyJhbGci..."
```

`-H` is repeatable, so you can send several global headers:

```bash
apic openapi.json \
  -H "Authorization: Bearer $TOKEN" \
  -H "X-Tenant-Id: acme" \
  -H "Accept-Language: en"
```

**Per-endpoint override.** If an endpoint declares its own header parameter
(for example a different `Authorization` for one call), the value you type in
the form **wins** over the global header for that request. Everything else
still comes from `-H`.

**Precedence, highest to lowest:**

1. A header parameter you fill in on the form.
2. A global `-H` header.
3. `apic`'s defaults (`Accept: application/json`, and `Content-Type:
   application/json` when a body is present).

**Keep tokens out of your shell history.** Prefer an environment variable
(`-H "Authorization: Bearer $TOKEN"`) over pasting the literal token, and
consider a leading space before the command if your shell is configured to skip
space-prefixed lines from history.

---

## 7. Getting the spec out of a Swagger UI page

A URL like `https://example.com/api/docs` is usually the **Swagger UI web
page**, not the raw spec. `apic` needs the raw JSON/YAML. There are three common
cases:

**a) A separate spec file.** View the page source or the network tab and look
for a `.json`/`.yaml` URL (often `/openapi.json`, `/swagger.json`,
`/v3/api-docs`, or `/api-docs`). Point `apic` at that:

```bash
apic https://example.com/v3/api-docs --server https://example.com
```

**b) The spec is inlined in the page's JavaScript.** Some servers
(swagger-ui-express) embed the whole spec inside `swagger-ui-init.js` as a
`swaggerDoc` object. Download that file and extract the object into a `.json`
file, then load the file:

```bash
curl -s -L "https://example.com/api/docs/swagger-ui-init.js" -o swui.js
# extract the swaggerDoc object into spec.json (any JSON tool works), then:
apic spec.json --server https://example.com
```

**c) You already have the file.** Just pass it:

```bash
apic spec.json --server https://example.com
```

If you point `apic` directly at an HTML page it will report that the content is
not a valid spec rather than silently showing nothing.

---

## 8. The interactive UI

Two screens: the **endpoint list** and the **detail** view.

### Endpoint list

| Key | Action |
|---|---|
| `j` / `↓` | Move down |
| `k` / `↑` | Move up |
| `/` | Filter (matches method, path, and summary) |
| `ctrl+e` | Switch the active server (see [Switching servers](#switching-servers)) |
| `enter` | Open the selected endpoint |
| `esc` | Clear the active filter |
| `q` | Quit |

Filtering is a case-insensitive substring match against `"METHOD /path
summary"`. Press `/`, type, then `enter` to keep the filter or `esc` to clear
it.

### Detail view

The left pane is the form generated from the endpoint's parameters (path,
query, and header params, plus a JSON body editor when the endpoint takes one).
Required fields are marked with `*`. Defaults are pre-filled. Parameters with an
enum become an **arrow-key selector** you cannot type an invalid value into; and
any auth the spec declares for the endpoint gets an auto-added field marked
`(auth)`.

| Key | Action |
|---|---|
| `tab` / `shift+tab` | Move between form fields |
| `←` / `→` | Cycle the value of the focused **enum** field |
| `ctrl+s` | Send the request (validates the body first) |
| `ctrl+b` | Validate the JSON body against the schema on demand |
| `ctrl+y` | Copy the equivalent **curl** command to the clipboard |
| `ctrl+f` | Filter the response with a dot-path (see [Filtering the response](#filtering-the-response)) |
| `ctrl+p` | Reload the last request sent to this endpoint |
| `ctrl+e` | Switch the active server |
| `ctrl+r` | Toggle response **headers** view |
| `ctrl+o` | Save the full response to a file |
| `ctrl+d` / `ctrl+u` | Scroll the response down / up (half page) |
| `pgdn` / `pgup` | Same as `ctrl+d` / `ctrl+u` |
| mouse wheel | Scroll the response |
| `ctrl+g` / `ctrl+t` | Jump to bottom / top of the response |
| `esc` | Back to the endpoint list |

`ctrl+d`/`ctrl+u` are the reliable scroll keys — they work on laptops without a
dedicated `PgDn` key. While the JSON body editor is focused, `ctrl+b`/`ctrl+e`/
`ctrl+f` act as textarea editing keys instead of shortcuts, so body editing is
never blocked.

The response header line shows the status (colour-coded: green 2xx, orange
3xx/4xx, red 5xx) and the round-trip time. If the body was larger than
`--max-body`, it shows `(truncated — ctrl+o to save full)`.

### Enum selectors

When a parameter declares an enum, its field becomes a selector: focus it and
use `←`/`→` to cycle the allowed values. You cannot type a value the API doesn't
accept. A required enum starts unset (shown as `(choose)`) so it's caught before
sending; an optional one can be left as `(skip)`.

### Auth auto-fill

`apic` reads the spec's `securitySchemes` and the security a given operation
requires. When you open a secured endpoint it adds the right field automatically,
marked `(auth)`:

- **API key** (`type: apiKey`) → a header or query field with the scheme's name.
- **Bearer / basic** (`type: http`) → an `Authorization` field, hinted
  `Bearer <token>` / `Basic <base64>`.

If you passed a global `-H "Authorization: …"`, the bearer field is pre-filled
from it. A value you type on the form always overrides the global.

### Validate the body

Before sending, `apic` validates the JSON body against the endpoint's
request-body schema and refuses to send an invalid one, naming the offending
field (e.g. `body invalid: /name: property "name" is missing`). Press `ctrl+b`
to validate on demand without sending.

### Save as curl

Press `ctrl+y` to copy the exact equivalent `curl` command — resolved URL,
headers (including auth and defaults), and body — to your clipboard. If no
clipboard is available (e.g. a bare SSH session), it writes `apic-curl.sh`
instead and tells you.

### Filtering the response

Press `ctrl+f` to open a filter box and type a dot-path to narrow a large
response:

```
.               whole document
.data.items     a nested field
.pets[0].name   an array element's field
.meta.total
```

The filter only changes what's displayed — `ctrl+o` (save) and `ctrl+y` (curl)
still use the full raw body. Press `enter` to apply, `esc` to clear.

### Reload a previous request

After a successful (2xx) request, `apic` remembers what you sent for that
endpoint in `~/.config/apic/history.json`. Reopen the endpoint and press
`ctrl+p` to refill the form. **Secrets are never written to disk** —
`Authorization`, `Cookie`, and any api-key values are stripped before saving, so
you re-enter those.

### Switching servers

If the spec lists several servers (prod/staging/local), press `ctrl+e` on either
screen to cycle the active base URL among them (plus any `--server` you passed).
The current base URL is shown in the header.

### Terminal size

The UI draws two panes side by side. Give it **~100 columns or more**; a
narrower window compresses the panes and can look cramped. Responses are wrapped
to the pane width, so long lines (like image URLs) stay inside the border.

---

## 9. Saving and reading large responses

The entire response body (up to `--max-body`, default 2 MB) is loaded into the
response pane and is scrollable — nothing is lost when it looks like only the
top is showing. For anything big, `ctrl+o` is the fastest way to see it all:

- Press `ctrl+o` — the **full** body is written to `apic-response.json` in the
  current directory (auto-numbered `apic-response-1.json`, `-2`, … so repeated
  saves never clobber each other). The footer confirms the path.
- Open it in any tool: `less apic-response.json`, `code apic-response.json`,
  `jq . apic-response.json`.

If a response exceeds `--max-body`, raise the cap:

```bash
apic openapi.json --max-body 16   # allow up to 16 MB responses
```

---

## 10. Use-case recipes

**Explore a public API interactively**

```bash
apic https://petstore3.swagger.io/api/v3/openapi.json
# open GET /pet/findByStatus, set status = available, ctrl+s
```

**Call your local service during development**

```bash
apic openapi.json --server http://localhost:8080
```

**A Swagger 2.0 service**

```bash
apic swagger.json                 # auto-converted, no extra tooling
apic swagger.yaml --server https://api.legacy.example.com
```

**Authenticated admin API**

```bash
apic openapi.json --server https://api.example.com \
  -H "Authorization: Bearer $ADMIN_TOKEN"
```

**Quickly audit which endpoints exist (scripting)**

```bash
apic openapi.json --list
apic openapi.json --list | grep DELETE      # every destructive endpoint
apic openapi.json --list | wc -l            # rough endpoint count
```

**A slow or flaky upstream**

```bash
apic openapi.json --timeout 2m
```

**A staging box with a self-signed certificate**

```bash
apic https://staging.internal/openapi.json -k --server https://staging.internal
```

**A big list/report endpoint**

```bash
apic openapi.json --max-body 32
# send, then ctrl+o to dump the full body to a file and inspect with jq
```

---

## 11. Security model

`apic` sends whatever you tell it to; these are the guardrails around that.

- **TLS verification is on by default.** Certificates are validated. `--insecure`
  turns verification off for both the spec fetch and requests — use it only for
  hosts you trust (self-signed staging).
- **Credential-safe redirects.** Redirects are capped at 10, and when a redirect
  crosses to a different host the `Authorization` and `Cookie` headers are
  stripped, so a redirect cannot leak your token to a third party.
- **No SSRF via `$ref`.** For remote specs, external `$ref`s are not fetched over
  the network. (Local files may resolve relative `$ref`s to sibling files, which
  is a deliberate convenience for local specs.)
- **Bounded resource use.** Remote specs are capped at 32 MB with a fetch
  timeout; response bodies are capped at `--max-body` (default 2 MB). Neither a
  giant spec nor a giant response can exhaust memory.
- **Proxies honoured.** `HTTP_PROXY` / `HTTPS_PROXY` / `NO_PROXY` are respected.
- **Saved files.** `ctrl+o` writes the response body verbatim to the current
  directory. Responses can contain secrets (tokens, PII) — treat saved files
  accordingly.
- **Your token is visible.** Header values you type into the form are shown in
  the clear, as in any API client. Prefer `-H "…: $TOKEN"` over literals.

---

## 12. Architecture

```
main.go
└── cmd/root.go            flag parsing, config wiring, --list, launches the TUI
    ├── internal/spec      load + convert + flatten any spec into simple structs
    │   ├── load.go        source reading, version sniffing, 2.0→3.0, YAML, fetch
    │   ├── spec.go        API/Endpoint/Param types + flatten + body skeletons
    │   └── url.go         remote URL validation
    ├── internal/httpx     build and send requests (no UI dependency)
    │   └── client.go      Config, redirect/TLS policy, size cap, pretty-print
    └── internal/ui        Bubble Tea layer
        ├── ui.go          model, screens, update loop, response pane
        ├── form.go        runtime form built from an endpoint's params
        └── styles.go      colours and method/status styling
```

**Design notes**

- **One translation boundary.** `spec` flattens kin-openapi's deeply nested,
  `$ref`-laden types into flat `Endpoint`/`Param` structs exactly once, so the
  UI never does six-level nil checks. Version conversion (Swagger 2.0 → 3) also
  happens here, so the rest of the program only ever sees OpenAPI 3.
- **`httpx` is UI-agnostic.** `Send` is a plain blocking function returning a
  `Result`; the UI wraps it in a `tea.Cmd`. That keeps it trivially testable.
- **Async without freezing.** Requests run in a goroutine and deliver a
  `responseMsg` back into the Bubble Tea update loop, so the UI never blocks.

---

## 13. Troubleshooting

| Symptom | Cause & fix |
|---|---|
| `no endpoints found` | You pointed at a Swagger UI HTML page, not the raw spec. See [§7](#7-getting-the-spec-out-of-a-swagger-ui-page). |
| `unrecognised spec: no "openapi" or "swagger" version field` | The file isn't an OpenAPI/Swagger document (or is HTML/an error page). |
| 404 with a doubled path prefix (`/content/content/...`) | `--server` duplicates a prefix already in the paths. Use host only. See [§5](#5-choosing-the-server---server). |
| `spec declares no server; pass --server …` | The spec has no usable `servers` entry (often a `{template}` placeholder). Pass `--server`. |
| 401 / 403 on every call | Missing or wrong auth. Add `-H "Authorization: …"`. See [§6](#6-authentication-and-global-headers). |
| `(truncated — ctrl+o to save full)` | Response exceeded `--max-body`. Raise it (`--max-body 16`) or `ctrl+o` to save. |
| Response looks cut off, but not flagged truncated | It's just scrolled to the top. Use `ctrl+d` / mouse wheel / `ctrl+g`, or `ctrl+o` to save and open elsewhere. |
| TLS / certificate error on a trusted host | Self-signed cert. Add `-k` / `--insecure`. |
| `could not fetch spec` / timeout | Host unreachable or slow. Check the URL; raise `--timeout`. |
| Cramped or overlapping panes | Terminal too narrow. Widen to ~100+ columns. |

Paste the exact error message when asking for help — the messages are written
to point at the specific cause.

---

## 14. Development

```bash
go build ./...            # compile everything
go vet ./...              # static checks
go test ./...             # run the suite (spec, httpx, ui)
go test -race ./...       # run with the race detector
go run . testdata/petstore.json   # run without installing
```

**Test data** lives in `testdata/`:

- `petstore.json` — OpenAPI 3 sample used by most tests. The spec tests load it
  by the relative path `../../testdata/petstore.json`, so it must stay there.
- `swagger2.json` — Swagger 2.0 sample proving the conversion path.
- `mini.yaml` — a YAML OpenAPI 3 sample proving YAML support.

**Module name.** `go.mod` uses `github.com/yash5155/apic`, matching the public
repository, so `go install github.com/yash5155/apic@latest` works. If you fork
it under a different name, update the imports in `main.go`, `cmd/root.go`,
`internal/ui/*.go`, and the test files to match.

**Pinning kin-openapi.** The converter and loader rely on kin-openapi. If a
`go mod tidy` pulls a newer version that breaks the build, pin it:

```bash
go get github.com/getkin/kin-openapi@v0.127.0
```
