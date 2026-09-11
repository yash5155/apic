# Contributing to apic

Thanks for your interest in improving **apic** 🎉 — a terminal client for any
OpenAPI/Swagger service. Contributions of every size are welcome: bug reports,
docs fixes, new features, and questions.

## Ways to contribute

- **Report a bug** — open a [Bug report](https://github.com/yash5155/apic/issues/new?template=bug_report.yml).
- **Request a feature** — open a [Feature request](https://github.com/yash5155/apic/issues/new?template=feature_request.yml).
- **Ask a question / share an idea** — use [Discussions](https://github.com/yash5155/apic/discussions) if enabled, otherwise open an issue.
- **Send a pull request** — see below.

## Development setup

You need **Go 1.22+**.

```bash
git clone https://github.com/yash5155/apic
cd apic
go build -o apic .            # build
go run . testdata/petstore.json   # run without building
```

## Before you open a pull request

Run the same checks CI runs — a PR must pass all of them:

```bash
gofmt -l .          # must print nothing (run `gofmt -w .` to fix)
go vet ./...
go build ./...
go test -race ./...
```

Please also:

- **Add or update tests** for any behaviour you change. Tests live next to the
  code (`*_test.go`) and run without a terminal — the UI model is driven with
  synthetic key messages.
- **Keep files under 500 lines** and match the surrounding style (the codebase
  favours small, well-commented functions).
- **Update the docs** (`README.md` and/or `docs/DOCUMENTATION.md`) when you add
  a flag, key binding, or user-visible behaviour.

## Pull request process

1. Fork the repo and create a branch from `main`:
   `git checkout -b fix/short-description`
2. Make your change with tests and docs.
3. Ensure the checks above pass locally.
4. Push and open a PR against `main`. Fill in the PR template.
5. CI runs automatically. A maintainer will review; address any feedback.
6. Once approved and green, a maintainer merges it. Thank you!

Small, focused PRs are reviewed fastest. If you're planning something large,
open an issue first so we can agree on the approach.

## Commit messages

Write clear, imperative commit subjects ("Add YAML spec support", not "added
stuff"). A short body explaining *why* is appreciated for non-trivial changes.

## Project layout

```
main.go                 entry point
cmd/root.go             CLI flags, config, launches the TUI
internal/spec/          load + convert any spec (OpenAPI 3.x, Swagger 2.0, YAML)
internal/httpx/         build and send requests (no UI dependency)
internal/ui/            Bubble Tea TUI (model, form, styles)
testdata/               sample specs used by tests
docs/DOCUMENTATION.md   full user documentation
```

See [`docs/DOCUMENTATION.md`](docs/DOCUMENTATION.md) §12 for architecture notes.

## Code of Conduct

By participating you agree to abide by our
[Code of Conduct](CODE_OF_CONDUCT.md).

## License

By contributing, you agree that your contributions will be licensed under the
[MIT License](LICENSE) that covers this project.
