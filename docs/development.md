# Development

How to run kanarche.eu on your own machine and how to run its tests. For what the
subcommands do once it is running, how the server is put together and how the
production container is built, see [operations.md](operations.md). For every
configuration key, see [configuration.md](configuration.md).

## Prerequisites

- Go (the version in `go.mod`)
- Node.js and npm, for the frontend bundle in `web/`
- Docker, for the local database and for the integration and e2e test tiers

## Running locally

`docker-compose.yml` is for local development only — it publishes Postgres on
the host and carries none of the hardening a production deployment needs. Do
not use it in production.

No credential is hardcoded in it: every value comes from the environment with a
development-only fallback, so `docker compose up` works with no setup. Copy
`.env.example` to `.env` to override any of them. `.env` is gitignored.

```bash
docker compose up -d db
export AIRBG_DATABASE_URL='postgres://airbg:airbg@localhost:5432/airbg?sslmode=disable'
export AIRBG_CONFIG="$PWD/airbg.yaml"
go run ./cmd/airbg migrate
go run ./cmd/airbg import-areas data/boundaries/bulgaria.geojson country
go run ./cmd/airbg collect
```

`AIRBG_CONFIG` must name the path to `airbg.yaml`; there is no fallback path.
See [configuration.md](configuration.md) for the full configuration reference.

The `import-areas ... country` step is not optional. Until a `country`
boundary is imported, `collect` polls upstream successfully and stores
nothing, every cycle — see
[operations.md § The national boundary is a hard prerequisite](operations.md#the-national-boundary-is-a-hard-prerequisite).

The database must be PostgreSQL 18 with both PostGIS and TimescaleDB; the
compose file already uses the right image. See
[operations.md § Database](operations.md#database).

### Serving the site

Build the frontend once, then run the server directly:

```bash
cd web && npm ci --ignore-scripts && npm run build
cd ..
go run ./cmd/airbg serve
```

`serve` runs the poller and the HTTP server in one process, so it ingests on
start just as `collect` does. With the shipped `airbg.yaml` the public listener
is `http://localhost:8080` and `/metrics` and `/healthz` are on
`127.0.0.1:9090`.

Skipping the `npm run build` step is a supported, if degraded, mode: the
server starts fine and serves the same pages without the map or chart
islands (no JavaScript, no CSS from the bundle), and logs one line at
startup — `assets state="no manifest — serving without islands (run 'npm
run build' in web/)"` — so the gap is discoverable rather than silent.

`tiles.*` ships empty, so a local run has no basemap: sensor markers render
over `frontend.empty_basemap_colour`. That is a supported configuration; to
serve a real basemap, generate it per [tiles.md](tiles.md).

## Tests

There are four test tiers:

```bash
go test ./... -race                    # unit: no Docker, no Node
go test -tags integration ./... -race  # integration: real PostgreSQL via testcontainers
cd web && npm test                     # Vitest: frontend unit tests
go test -tags e2e ./internal/e2e/      # Playwright, driven from a build-tagged Go test
```

`go test ./... -race` alone does **not** start any containers — the
testcontainers-backed suite in `internal/server/e2e_test.go` is gated behind
the `integration` build tag specifically so the default `go test ./...` stays
fast and Docker-free. Run `go test -tags integration ./... -race` to include
it; Docker must be running, and the first run pulls
`timescale/timescaledb-ha:pg18`.

The `e2e` tier requires `npm run build` to have been run first in `web/` — the
Go test serves the embedded Vite bundle, so without a build every island is
missing and every spec fails — plus a Docker daemon (it starts the same
Postgres container as the `integration` tier) and Playwright's browsers
(`npx playwright install --with-deps chromium` in `web/`).

To check the live upstream contract:

```bash
AIRBG_LIVE_TEST=1 go test ./internal/ingest/ -run TestUpstreamContractLive
```

This test makes a real network call to data.sensor.community and is skipped
unless `AIRBG_LIVE_TEST=1` is set.

**The container test suite can flake under load.** A single transient failure
in `internal/store` has been observed during a full `-race` run, self-resolving
on rerun. Suspected testcontainers resource contention rather than a code
defect; worth watching if it recurs in CI.

## Where things live

| Path | What |
|---|---|
| `cmd/airbg` | The single binary and its subcommands |
| `internal/` | Ingest, storage, snapshot, API, pages, i18n |
| `internal/i18n/*.json` | All user-facing copy, one file per language |
| `web/` | Frontend islands (Vite build, embedded into the binary) |
| `airbg.yaml` | Every operational constant, with the reasoning as comments |
| `data/boundaries/` | Area boundary files and their provenance |
| `deploy/` | The production stack; see [deployment.md](deployment.md) |
| `docs/design/DESIGN.md` | The visual design contract |
