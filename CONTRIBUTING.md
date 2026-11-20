# Contributing

Thanks for looking at Kanarche. This project is a Go backend + Svelte/Vite
frontend for an air quality map of Bulgaria.

## Setup

See the [README](README.md) for running it locally and
[`docs/configuration.md`](docs/configuration.md) for the environment variables.

## Building and testing

```bash
go build ./...
go test ./... -race                    # unit: no Docker, no Node
go test -tags integration ./... -race  # integration: real Postgres via testcontainers
cd web && npx vitest run               # frontend unit tests
cd web && npm run build                # frontend build
```

The integration tier starts its own Postgres container via testcontainers. On
macOS with Colima, that needs
`DOCKER_HOST=unix://$HOME/.colima/default/docker.sock` and
`TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE=/var/run/docker.sock`.

The e2e tier is a Go test that serves the built frontend, so it must run in
this order (`-count=1` stops Go reusing a cached pass against an old bundle):

```bash
cd web && VITE_E2E_MAP_HANDLE=1 npm run build
go test -tags e2e -count=1 ./internal/e2e/
cd web && npm run build   # rebuild without the E2E flag before committing/serving normally
```

## Pre-commit hooks

```bash
brew install pre-commit gitleaks   # or: pipx install pre-commit
pre-commit install
pre-commit run --all-files
```

On each commit the hooks check whitespace, YAML and JSON syntax, merge markers, large files, secrets (gitleaks), `gofmt`, `go vet`, the i18n copy rule and private paths. Tests, staticcheck, govulncheck and e2e run only in CI.

## Pull requests

- Open a PR against `master`. CI (`.github/workflows/ci.yml`) must be green:
  Go build/vet/staticcheck/govulncheck, `go test ./... -race`, the web unit
  tests and build, the integration tier, and the e2e tier.
- Keep commits focused: a short imperative subject line (e.g. `fix: reject
  empty geometry on import`), and comments limited to 1-2 lines that explain
  *why*, not what.

## Reporting bugs or data problems

Use the issue templates under **Issues > New issue**. See
[`SECURITY.md`](SECURITY.md) for vulnerabilities instead of a public issue.
