# Testing

How to run the provider test suite locally, what each Make target does, and
how to reproduce the CI pipeline before pushing.

## Table of Contents

- [Prerequisites](#prerequisites)
- [Test Layout](#test-layout)
- [Running Tests](#running-tests)
  - [Unit Tests Only (fast, no Docker)](#unit-tests-only-fast-no-docker)
  - [Acceptance Tests (Docker + PostgreSQL)](#acceptance-tests-docker--postgresql)
  - [Everything (the full `make all` pipeline)](#everything-the-full-make-all-pipeline)
- [CI Parity](#ci-parity)
  - [Target Map](#target-map)
  - [Reproducing a CI Failure Locally](#reproducing-a-ci-failure-locally)
- [Environment Variables](#environment-variables)
- [How Acceptance Tests Work](#how-acceptance-tests-work)
- [Troubleshooting](#troubleshooting)

## Prerequisites

- **Go** ≥ 1.26.2 (the toolchain pinned in `go.mod:3` —
  `GOTOOLCHAIN=auto`, the default since Go 1.21, will auto-fetch it if your
  system has an older version).
- **Docker** (acceptance tests spin up a PostgreSQL container via
  [testcontainers-go](https://github.com/testcontainers/testcontainers-go)).
- **Terraform** ≥ 1.0 on `$PATH` (acceptance tests shell out via
  `TF_ACC_TERRAFORM_PATH=$(which terraform)`, set automatically by the
  Makefile).

All Go tooling (`golangci-lint`, `goimports`, `tfplugindocs`, `govulncheck`)
is declared in `go.mod` and resolved via `go tool` / the Make targets —
nothing to install manually. `make ci-vuln` will install `govulncheck` into
`$GOBIN` on first use if it isn't already present
(`Makefile:84-89`).

## Test Layout

Tests live under `test/` as external test packages (`package xxx_test`)
so they can only touch the exported API of `internal/`:

```
test/
├── acctest/              # Shared helpers for acceptance tests
├── mocks/                # GoMock mocks (DBTX, Scanner, Rows, Tx)
├── unit/
│   ├── common/           # helpers, retry, allowlist, option builders
│   ├── datasource/       # data-source Read paths with GoMock
│   ├── provider/         # Configure + env-var fallbacks
│   └── resource/         # CRUD flows for every resource
└── integration/          # Acceptance tests (//go:build integration)
    ├── datasource/
    ├── provider/
    └── resource/
```

Unit tests use `go.uber.org/mock` (GoMock) for the DB surface. Acceptance
tests use real PostgreSQL via testcontainers and are compiled only with
`-tags integration` — this keeps unit-test builds lean and keeps
`govulncheck` clear of test-only dependencies.

Mock regeneration (rarely needed):

```bash
mockgen -destination=test/mocks/mock_db.go -package=mocks \
  github.com/DiegoBulhoes/terraform-provider-postgresql/internal/common \
  DBTX,Scanner,Rows,Tx
```

## Running Tests

### Unit Tests Only (fast, no Docker)

```bash
make ci-unit                               # with race + coverage
go test -race ./test/unit/...              # same, without the Make wrapper
go test -run TestReadRole ./test/unit/...  # a single test
```

`make ci-unit` writes `coverage-unit.out` and prints the total at the end.
Target definition: `Makefile:92-94`.

### Acceptance Tests (Docker + PostgreSQL)

```bash
# Full matrix (PG 14, 15, 16, 17)
make ci-acceptance

# Subset — override PG_VERSIONS
make ci-acceptance PG_VERSIONS="16 17"

# Single version, explicit
POSTGRES_IMAGE=postgres:17-alpine TF_ACC=1 \
  TF_ACC_TERRAFORM_PATH=$(which terraform) \
  go test -tags integration -timeout 600s -count=1 ./test/integration/...
```

`make ci-acceptance` iterates over `$(PG_VERSIONS)` (default `14 15 16 17`,
`Makefile:5`) and stops at the first failing version via
`|| exit 1` (`Makefile:103-110`).

### Everything (the full `make all` pipeline)

```bash
make all   # tidy → lint → security → test → build → docs
```

Note: `make test` (`Makefile:37-47`) runs both unit **and** the full
acceptance matrix — it requires Docker + Terraform. If you just want the
fast feedback loop, prefer `make ci` (unit-only) or `make ci-unit`.

## CI Parity

`make ci` mirrors the `.github/workflows/test.yml` `checks` and `unit`
jobs. It runs in order and fails on the first problem, just like CI:

```bash
make ci
```

On success you'll see:

```
✓ CI checks + unit tests pass (mirrors .github/workflows/test.yml 'checks' + 'unit' jobs)
  Acceptance tests require Docker + PG — run 'make ci-acceptance' for full parity.
```

### Target Map

| `make` target      | CI step in `.github/workflows/test.yml`               | Modifies files? |
|--------------------|-------------------------------------------------------|-----------------|
| `ci-vet`           | `go vet ./...` (`test.yml:28-29`)                     | no |
| `ci-fmt-check`     | `gofmt -l .` + fail-if-nonempty (`test.yml:31-34`)    | no |
| `ci-lint`          | `golangci-lint run ./...` (`test.yml:36-37`)          | no |
| `ci-docs`          | `tfplugindocs validate` (`test.yml:39-40`)            | no |
| `ci-vuln`          | install + run `govulncheck` (`test.yml:42-46`)        | no |
| `ci-unit`          | `go test -race -coverpkg ./test/unit/...` (`test.yml:59-60`) | writes `coverage-unit.out` |
| `ci`               | all of the above in sequence                          | no source changes |
| `ci-acceptance`    | `acceptance` job matrix (`test.yml:62-96`)            | — |

The lint+format check is **non-modifying** — unlike `make lint`, which
runs `go fmt` first. If you want CI-style behavior (fail fast on
unformatted code) use `make ci-fmt-check`. If you want to actually
format the code use `make fmt`.

### Selecting PostgreSQL Versions in CI

Both workflows take an optional `pg_versions` input on
`workflow_dispatch`, so the acceptance matrix is built at runtime instead
of being hardcoded:

```yaml
postgres_version: ${{ fromJSON(github.event.inputs.pg_versions || '["14","15","16","17"]') }}
```

- **Pull requests** get the full `14 15 16 17` matrix (the input is
  absent, so the fallback applies).
- **Manual runs** (Actions → *Tests* / *Release* → *Run workflow*) accept
  a JSON array — e.g. `["16","17"]` for a quick two-version run. It must
  be valid JSON, otherwise `fromJSON` fails before the matrix expands.

The `run-name` reflects the choice, so a manual run shows up as
`Tests - my-branch (PG ["16","17"])` in the Actions list.

This is the CI equivalent of `make ci-acceptance PG_VERSIONS="16 17"`
locally. Note the two are **not** wired to the same source: the Makefile
default lives in `Makefile:5` and the workflow default in the `fromJSON`
fallback above — when you add a PostgreSQL version, update both.

Caveat: the Codecov upload is gated on `matrix.postgres_version == '17'`
(`test.yml:91`), so a manual run that excludes `17` uploads no coverage.

### Reproducing a CI Failure Locally

When CI fails, run the specific target that corresponds to the failing
step:

```bash
# CI said: "gofmt -l . returned files"
make ci-fmt-check       # see the list
make fmt                # fix

# CI said: "golangci-lint exited non-zero"
make ci-lint

# CI said: "govulncheck exited 3"
make ci-vuln            # confirm; usually means Go stdlib CVE → bump go.mod

# CI said: "tfplugindocs validate failed"
make docs               # regenerate from templates/
make ci-docs            # re-validate
```

## Environment Variables

| Variable                | Default                  | Description |
|-------------------------|--------------------------|-------------|
| `TF_ACC`                | —                        | Required for acceptance tests to actually run (Terraform SDK convention). The Makefile sets this automatically. |
| `PG_VERSIONS`           | `14 15 16 17`            | PostgreSQL versions to test (matrix variable; `Makefile:5`). |
| `POSTGRES_IMAGE`        | `postgres:<v>-alpine`    | Container image override (`Makefile:42,106`). |
| `TF_ACC_TERRAFORM_PATH` | `$(which terraform)`     | Path to the Terraform binary the SDK invokes. |
| `PGHOST`                | (testcontainer)          | If set, acceptance tests skip the container and use an external DB. |
| `PGPORT`                | (testcontainer)          | Port override when `PGHOST` is set. |
| `PGUSER`                | `postgres`               | Username for the external DB case. |
| `PGPASSWORD`            | `postgres`               | Password for the external DB case. |
| `PGDATABASE`            | `postgres`               | Default database. |
| `PGSSLMODE`             | `disable`                | SSL mode for the external DB case. |
| `GOTOOLCHAIN`           | `auto`                   | Go ≥ 1.21 auto-fetches the toolchain declared in `go.mod:3`. Set to `local` to disable. |

## How Acceptance Tests Work

Tests in `test/integration/` are guarded by `//go:build integration`, so
they compile only when `-tags integration` is passed. Each test calls into
`test/acctest/acctest.go` to:

1. Start a PostgreSQL container (or reuse an external DB if `PGHOST` is
   set).
2. Expose connection info via the standard `PG*` environment variables
   before the `terraform-plugin-framework` test harness boots the provider.
3. Execute `terraform plan` / `apply` / `import` / `destroy` steps defined
   by the test via `resource.TestStep`.

Because the container is shared between test steps within a single test,
keep `-parallel 1` (the default) unless you know what you're doing —
otherwise you can exhaust `max_connections` quickly.

## Troubleshooting

**`pq: sorry, too many clients already`** — add `-parallel 1` or
reduce the number of concurrent `resource.TestStep`s in your test. The
container starts with `max_connections=500`, but each step opens its
own pool.

**`Error: Cannot connect to the Docker daemon`** — run `docker info`;
start Docker and retry.

**`govulncheck` reports stdlib CVEs** — these are fixed in newer Go
patch releases. The fix is to bump the version in `go.mod:3` (the CI
runner reads it via `setup-go@v6 with: go-version-file: go.mod`,
`test.yml:24-26`); your local Go ≥ 1.21 will auto-fetch the required
toolchain.

**`tfplugindocs validate` fails after editing templates** — regenerate:
`make docs` (`Makefile:51-53`), then commit the regenerated `docs/`.
Do **not** hand-edit files in `docs/` — they will be overwritten by the
next `make docs` run (see `CLAUDE.md:64`).
