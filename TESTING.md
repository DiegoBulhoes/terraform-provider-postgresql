# Testing

How to run the test suite locally, what each Make target does, and how to
reproduce the CI pipeline before you push.

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

- **Go** ≥ 1.26.5, the toolchain pinned in `go.mod`. If your system has an
  older version, `GOTOOLCHAIN=auto`, the default since Go 1.21, fetches the
  right one.
- **Docker** (acceptance tests start a PostgreSQL container with
  [testcontainers-go](https://github.com/testcontainers/testcontainers-go)).
- **Terraform** ≥ 1.0 on `$PATH`. Acceptance tests call it through
  `TF_ACC_TERRAFORM_PATH=$(which terraform)`, which the Makefile sets for you.

The Go tools (`golangci-lint`, `goimports`, `tfplugindocs`, `govulncheck`)
are declared in `go.mod` and resolved by `go tool` and the Make targets, so
there is nothing to install by hand. On first use, `make ci-vuln` installs
`govulncheck` into `$GOBIN` if it is missing.

## Test Layout

Tests live under `test/` as external test packages (`package xxx_test`), so
they can only reach the exported API of `internal/`:

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

Unit tests mock the database with `go.uber.org/mock` (GoMock). Acceptance
tests run against a real PostgreSQL through testcontainers and only compile
with `-tags integration`. That keeps unit-test builds small and keeps
`govulncheck` away from test-only dependencies.

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

`make ci-acceptance` iterates over `$(PG_VERSIONS)` (default `14 15 16 17`)
and stops at the first failing version via `|| exit 1`.

### Everything (the full `make all` pipeline)

```bash
make all   # tidy → lint → security → test → build → docs
```

Note: `make test` runs the unit tests **and** the full acceptance matrix, so
it needs Docker and Terraform. For a fast loop, use `make ci` or
`make ci-unit`, which run unit tests only.

## CI Parity

`make ci` mirrors the `checks` and `unit` jobs in
`.github/workflows/test.yml`. It runs them in order and stops at the first
problem, like CI does:

```bash
make ci
```

On success you'll see:

```
✓ CI checks + unit tests pass (mirrors .github/workflows/test.yml 'checks' + 'unit' jobs)
  Acceptance tests require Docker + PG — run 'make ci-acceptance' for full parity.
```

### Target Map

| `make` target   | CI step in `.github/workflows/test.yml`        | Modifies files? |
|-----------------|------------------------------------------------|-----------------|
| `ci-vet`        | `Run go vet` step                              | no |
| `ci-fmt-check`  | `Check formatting` step                        | no |
| `ci-lint`       | `Run golangci-lint` step                       | no |
| `ci-docs`       | `Validate documentation` step                  | no |
| `ci-vuln`       | `Install govulncheck` + `Run govulncheck`      | no |
| `ci-unit`       | `unit` job                                     | writes `coverage-unit.out` |
| `ci`            | all of the above in sequence                   | no source changes |
| `ci-acceptance` | `acceptance` job matrix                        | — |

The lint and format checks **change nothing**, unlike `make lint`, which
runs `go fmt` first. Use `make ci-fmt-check` to fail on unformatted code the
way CI does. Use `make fmt` to actually format it.

### Selecting PostgreSQL Versions in CI

**Release always runs all four versions.** `release.yml` has a fixed
`["14", "15", "16", "17"]` matrix and no way to skip a version, so a tag can
never ship on partial coverage.

Only `test.yml` lets you pick. It shows one checkbox per version on
`workflow_dispatch` (`pg14` to `pg17`, all checked by default), and the
`resolve-matrix` job turns the checked boxes into the matrix:

```
Run workflow ▾
  [x] PostgreSQL 14      [x] PostgreSQL 16
  [x] PostgreSQL 15      [x] PostgreSQL 17
```

- **Pull requests** run all four. The inputs only exist for
  `workflow_dispatch`, so every `PG*` variable arrives empty and the
  `[ "$PG14" = "false" ] || selected+=(14)` test keeps it. No check on
  `github.event_name` is needed.
- **Manual runs** (Actions → *Tests* → *Run workflow*) run exactly what you
  check. Unchecking everything fails the run with `select at least one
  PostgreSQL version`. Without that guard, the empty matrix would show up as
  a skipped job, which looks like a pass.

The default version list lives in three places that are **not** connected:
`PG_VERSIONS` in the Makefile, the checkbox list in `test.yml`, and the fixed
matrix in `release.yml`. When you add a PostgreSQL version, update all three.

Caveat: in `test.yml` the Codecov upload is gated on
`matrix.postgres_version == '17'`, so a manual run that unchecks `17` uploads
no coverage.

### Reproducing a CI Failure Locally

When CI fails, run the target that matches the failing step:

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
| `PG_VERSIONS`           | `14 15 16 17`            | PostgreSQL versions to test (Makefile matrix variable). |
| `POSTGRES_IMAGE`        | `postgres:<v>-alpine`    | Container image override. |
| `TF_ACC_TERRAFORM_PATH` | `$(which terraform)`     | Path to the Terraform binary the SDK invokes. |
| `PGHOST`                | (testcontainer)          | If set, acceptance tests skip the container and use an external DB. |
| `PGPORT`                | (testcontainer)          | Port override when `PGHOST` is set. |
| `PGUSER`                | `postgres`               | Username for the external DB case. |
| `PGPASSWORD`            | `postgres`               | Password for the external DB case. |
| `PGDATABASE`            | `postgres`               | Default database. |
| `PGSSLMODE`             | `disable`                | SSL mode for the external DB case. |
| `GOTOOLCHAIN`           | `auto`                   | Go ≥ 1.21 auto-fetches the toolchain declared in `go.mod`. Set to `local` to disable. |

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

All steps of a single test share one container, so keep `-parallel 1`, the
default. Higher values can use up `max_connections` fast.

## Troubleshooting

**`pq: sorry, too many clients already`** — add `-parallel 1`, or run fewer
`resource.TestStep`s at once. The container starts with
`max_connections=500`, but every step opens its own pool.

**`Error: Cannot connect to the Docker daemon`** — run `docker info`;
start Docker and retry.

**`govulncheck` reports stdlib CVEs** — newer Go patch releases fix these.
Raise the version in `go.mod`; the CI runner reads it through `setup-go@v6
with: go-version-file: go.mod`, and local Go ≥ 1.21 fetches the toolchain
for you.

**`tfplugindocs validate` fails after editing templates** — regenerate:
`make docs`, then commit the regenerated `docs/`.
Do **not** hand-edit files in `docs/` — they will be overwritten by the
next `make docs` run (see `CLAUDE.md`).

**`tfplugindocs` fails with `openpgp: key expired`** — the full message is
`unable to download Terraform binary: unable to verify checksums
signature`. `tfplugindocs` needs Terraform to export the provider schema.
With no binary on `PATH`, it downloads one through `hc-install`, and that
download fails PGP checks against HashiCorp's expired signing key. Give it
a binary instead: CI runs `hashicorp/setup-terraform@v4` right before the
validate step. This never happens locally while `terraform` is on your
`PATH`, which is why `make ci-docs` can pass while CI fails.
