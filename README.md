# apic

[![CI](https://github.com/yash5155/apic/actions/workflows/ci.yml/badge.svg)](https://github.com/yash5155/apic/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/yash5155/apic?sort=semver)](https://github.com/yash5155/apic/releases)
[![Go Reference](https://pkg.go.dev/badge/github.com/yash5155/apic.svg)](https://pkg.go.dev/github.com/yash5155/apic)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![PRs welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg)](CONTRIBUTING.md)

A terminal client for any **OpenAPI 3.x or Swagger 2.0** service, in JSON or
YAML, local or remote. It reads the spec, lists the endpoints, and builds an
input form from each endpoint's declared parameters — so you get correct field
names, required markers, enum hints and a prefilled JSON body without typing any
of it yourself.

Generic HTTP clients exist everywhere. A schema-driven one in the terminal
doesn't, and that's the whole point of the project.

> **Full documentation:** see [`docs/DOCUMENTATION.md`](docs/DOCUMENTATION.md)
> for every flag, every use case, the security model, and troubleshooting.

## Install

One line — downloads the right prebuilt binary for your OS/arch, no Go needed:

```bash
curl -fsSL https://raw.githubusercontent.com/yash5155/apic/main/install.sh | bash
```

Then just run:

```bash
apic openapi.json
```

Other ways:

```bash
go install github.com/yash5155/apic@latest        # if you have Go
# or grab a binary from https://github.com/yash5155/apic/releases
```

The installer drops `apic` in `~/.local/bin` by default (override with
`APIC_INSTALL_DIR`). If that's not on your PATH it prints the one line to add it.

## Build from source

```bash
git clone https://github.com/yash5155/apic && cd apic
go build -o apic .

./apic --list testdata/petstore.json   # parser check, no UI
./apic testdata/petstore.json          # the real thing
./apic https://petstore3.swagger.io/api/v3/openapi.json
./apic testdata/swagger2.json          # Swagger 2.0, auto-converted
./apic testdata/mini.yaml              # YAML spec
```

Against a real API you'll usually want to override the base URL and pass auth:

```bash
./apic openapi.json --server http://localhost:8080
./apic openapi.json --server https://api.example.com -H "Authorization: Bearer $TOKEN"
```

## Flags

`--server` base URL · `-H/--header` global header (repeatable) · `--env`
environment · `--var key=value` set a variable (repeatable) · `--list` print
and exit · `--timeout` per-request timeout · `-k/--insecure` skip TLS verify ·
`--max-body` response cap (MB) · `-v/--version`. Run `apic --help` for details.

## Keys

**Endpoint list:** `j`/`k` move · `/` filter · `ctrl+e` switch server · `enter` open · `q` quit
**Detail:** `tab`/`shift+tab` field · `←`/`→` cycle enum · `ctrl+s` send ·
`ctrl+b` validate body · `ctrl+y` copy as curl · `ctrl+f` filter response ·
`ctrl+k` capture a value into a variable · `ctrl+p` reload last request ·
`ctrl+e` switch server · `ctrl+n` switch env · `ctrl+r` headers ·
`ctrl+d`/`ctrl+u` (or `pgdn`/`pgup`, or mouse wheel) scroll · `ctrl+g`/`ctrl+t`
end/top · `ctrl+o` save full response · `esc` back

## Features

Schema-driven forms · **enum dropdowns** (arrow-key selectors) · **body
validation** against the request schema before sending · **security-scheme
auto-fill** (api-key / bearer fields from the spec) · **response filtering**
with a `.data.items[0]` dot-path · **save as curl** to the clipboard · **request
history** per endpoint (secrets never written to disk) · **runtime server
switching** across the spec's servers · **environments & variables** —
`~/.config/apic/config.json` holds named environments with `{{variables}}` you
can use anywhere, plus OpenAPI `{scheme}://{host}` server-template expansion ·
**request chaining** — capture a value from one response and reuse it in later
requests.

### Environments & variables

Create `~/.config/apic/config.json`:

```json
{
  "active": "staging",
  "environments": {
    "prod":    { "base_url": "https://api.example.com", "vars": { "token": "{{env.PROD_TOKEN}}" } },
    "staging": { "base_url": "https://staging.example.com", "headers": { "X-Env": "staging" }, "vars": { "token": "abc" } }
  }
}
```

Then use `{{token}}` in any field, URL, header, or body. Switch environments at
runtime with `ctrl+n`, or per-run with `--env prod --var token=xyz`. Unresolved
`{{…}}` blocks the send with a clear message. Prefer `{{env.NAME}}` to keep real
secrets out of the file (which is written `0600`).

## How it's put together

```
cmd/root.go            Cobra: flags, spec loading, base URL resolution
internal/spec/spec.go  OpenAPI document -> our own Endpoint/Param structs
internal/httpx/        builds and sends the request. No UI knowledge.
internal/ui/form.go    the form generated at runtime from a schema
internal/ui/ui.go      root model, both screens, async handling
```

### The important design decision

`spec.Load` converts kin-openapi's types into plain `Endpoint` and `Param`
structs immediately, and nothing else in the program ever imports
kin-openapi.

Those library types are deeply nested and full of `*Ref` indirection —
`op.Parameters[0].Value.Schema.Value.Type` is a normal access path, and
every link can be nil. If that leaks into the UI, every `View` function
becomes six levels of nil checks. Pay the cost once at the boundary, and
the rest of the code stays readable.

Same reasoning for `httpx`: `Send` is a plain blocking function that knows
nothing about Bubble Tea, which is exactly why it's easy to test.

## The two new concepts (compared to a simple TUI)

**1. Forms you don't know at compile time.** You can't declare the inputs
as struct fields, because the number of them depends on which endpoint the
user picked. So they live in a slice and focus is an index into it:

```go
type form struct {
    inputs []textinput.Model
    body   textarea.Model
    focus  int   // 0..len(inputs)-1 = a param, len(inputs) = the body
}
```

`focusCurrent` blurs everything and re-focuses whatever `focus` points at.
Blurring all of them every time is slightly wasteful and much simpler than
tracking the previous field.

**2. Async without freezing.** A `tea.Cmd` is just a function returning a
message. Bubble Tea runs it on its own goroutine and feeds the result back
into `Update` like any other event:

```go
func sendRequest(r httpx.Request) tea.Cmd {
    return func() tea.Msg {
        return responseMsg(httpx.Send(r))   // blocking, off the main loop
    }
}
```

`Update` returns immediately with `sending = true`, so the spinner keeps
animating while the request is in flight. When it lands, `responseMsg`
arrives and you clear the flag.

`tea.Batch(m.spin.Tick, sendRequest(req))` starts both at once.

**Never call a blocking function directly inside `Update`.** That is the
single biggest mistake in Bubble Tea apps — the whole UI locks up until it
returns.

## Contributing

Contributions are welcome — bug reports, features, docs, questions.

- 🐛 [Open a bug report](https://github.com/yash5155/apic/issues/new?template=bug_report.yml)
- ✨ [Request a feature](https://github.com/yash5155/apic/issues/new?template=feature_request.yml)
- 🔧 Send a pull request — see [CONTRIBUTING.md](CONTRIBUTING.md)

Every PR is checked by CI (gofmt, vet, build, `test -race`). Please read the
[Contributing guide](CONTRIBUTING.md) and our [Code of Conduct](CODE_OF_CONDUCT.md).
Security issues? See [SECURITY.md](SECURITY.md).

## Tests

```bash
go test ./...
```

No terminal needed. The model is a pure function, so the UI tests
drive it with synthetic `tea.KeyMsg` values and assert on the resulting
state. `TestFullRequestRoundTrip` goes end to end: presses ctrl+s, runs the
command Bubble Tea would have run, unwraps the `tea.BatchMsg`, feeds the
`responseMsg` back into `Update`, and checks the status line renders — all
against a real `httptest` server.

## Ideas for later

The big feature set (auth auto-fill, history, enum selectors, response
filtering, multi-server, body validation, save-as-curl) is now built — see
[Features](#features). Still open:

- **OAuth2 flows.** Bearer/basic/api-key work today; a full OAuth2 token dance
  (authorization-code / client-credentials) would be a nice addition.
- **Real jq.** The response filter is a lightweight dot-path; swapping in a jq
  engine behind a flag would unlock pipes and `select()`.
- **Enum multi-select** for array-of-enum query params.
- **Named history** — keep more than the last request per endpoint.

## Notes

Versions in `go.mod` are pinned to what this was verified against. Newer
kin-openapi releases are fine, but note that `Schema.Type` changed from
`string` to `*Types` around v0.121 — if you upgrade and see type errors,
that's why.
