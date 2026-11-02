# Testing

How to run the test suite locally, what each Make target does, and how to
reproduce the CI pipeline before you push.

## Contents

- [Prerequisites](#prerequisites)
- [Test layout](#test-layout)
- [Running tests](#running-tests)
  - [Unit tests only (fast, no Docker)](#unit-tests-only-fast-no-docker)
  - [Acceptance tests (Docker + PostgreSQL)](#acceptance-tests-docker--postgresql)
  - [Everything (the full `make all` pipeline)](#everything-the-full-make-all-pipeline)
- [CI parity](#ci-parity)
  - [Target map](#target-map)
  - [Reproducing a CI failure locally](#reproducing-a-ci-failure-locally)
- [Environment variables](#environment-variables)
- [How acceptance tests work](#how-acceptance-tests-work)
- [Troubleshooting](#troubleshooting)

## Prerequisites

- Go ≥ 1.26.10, the toolchain pinned in `go.mod`. If your system has an
  older version, `GOTOOLCHAIN=auto`, the default since Go 1.21, fetches the
  right one.
- Docker (acceptance tests start a PostgreSQL container with
  [testcontainers-go](https://github.com/testcontainers/testcontainers-go)).
- Terraform ≥ 1.5 on `$PATH`. Acceptance tests call it through
  `TF_ACC_TERRAFORM_PATH=$(which terraform)`, which the Makefile sets for you.

The Go tools (`golangci-lint`, `goimports`, `tfplugindocs`, `govulncheck`,
`actionlint`) are declared in `go.mod` and resolved by `go tool` and the Make
targets, so you do not need to install anything by hand.

## Test layout

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

## Running tests

### Unit tests only (fast, no Docker)

```bash
make ci-unit                               # with race + coverage
go test -race ./test/unit/...              # same, without the Make wrapper
go test -run TestReadRole ./test/unit/...  # a single test
```

`make ci-unit` writes `coverage-unit.out` and prints the total at the end.

### Acceptance tests (Docker + PostgreSQL)

```bash
# Full matrix (PG 14 to 18)
make ci-acceptance

# Subset — override PG_VERSIONS
make ci-acceptance PG_VERSIONS="16 17"

# Single version, explicit
POSTGRES_IMAGE=postgres:17-alpine TF_ACC=1 \
  TF_ACC_TERRAFORM_PATH=$(which terraform) \
  go test -tags integration -timeout 600s -count=1 ./test/integration/...
```

`make ci-acceptance` iterates over `$(PG_VERSIONS)` (default `14 15 16 17 18`)
and stops at the first failing version via `|| exit 1`.

### Everything (the full `make all` pipeline)

```bash
make all   # tidy → lint → security → test → build → docs
```

Note: `make test` runs the unit tests and the full acceptance matrix, so
it needs Docker and Terraform. For a fast loop, use `make ci` or
`make ci-unit`, which run unit tests only.

## CI parity

The checks live in `.github/workflows/ci.yml`, a reusable workflow. `test.yml`
calls it on pull requests, on pushes to `main`, every Monday and by hand;
`release.yml` calls it before releasing. `make ci` mirrors its `checks` and
`unit` jobs and stops at the first problem, like CI does:

```bash
make ci
```

### Target map

| `make` target   | Step in `.github/workflows/ci.yml`          | Modifies files? |
|-----------------|---------------------------------------------|-----------------|
| `ci-tidy`       | `Check go.mod and go.sum are tidy`          | no |
| `ci-vet`        | `Run go vet` (with and without `integration`) | no |
| `ci-fmt-check`  | `Check formatting`                          | no |
| `ci-lint`       | `Run golangci-lint`                         | no |
| `ci-actions`    | `Lint workflows` (actionlint)               | no |
| `ci-docs`       | `Validate documentation`                    | no |
| `ci-vuln`       | `Run govulncheck`                           | no |
| `ci-unit`       | `unit` job                                  | writes `coverage-unit.out` |
| `ci`            | all of the above in sequence                | no source changes |
| `ci-acceptance` | `acceptance` job matrix                     | writes `coverage-acceptance-<version>.out` |
| `coverage`      | `coverage` job: unit + acceptance against the floor | n/a |

The lint and format checks change nothing, unlike `make lint`, which
runs `go fmt` first. govulncheck, actionlint, golangci-lint and tfplugindocs
are `tool` entries in `go.mod`, so CI and your machine run the same versions.

### Coverage floor

The `coverage` job merges the unit profile and the acceptance profile of
PostgreSQL 17 with `scripts/coverage.sh` and fails below 90%. Unit tests
reach the error paths a real server cannot produce; acceptance tests cover
the rest. A manual run without PostgreSQL 17 has no acceptance profile, so the
floor applies to the unit profile alone.

### Selecting PostgreSQL versions in CI

A release always runs every version. `release.yml` calls `ci.yml` with its
default list, so a tag can never ship on partial coverage. Every version also
runs with the newest Terraform, and PostgreSQL 17 once more with Terraform
1.5.7, the oldest supported.

Only `test.yml` lets you pick. It shows one checkbox per version on
`workflow_dispatch` (`pg14` to `pg18`, all checked by default), and the
`resolve-matrix` job turns the checked boxes into the matrix:

```
Run workflow ▾
  [x] PostgreSQL 14      [x] PostgreSQL 17
  [x] PostgreSQL 15      [x] PostgreSQL 18
  [x] PostgreSQL 16
```

- Pull requests, pushes and the weekly run use every version. The inputs
  only exist for `workflow_dispatch`, so every `PG*` variable arrives empty
  and the `[ "$PG14" = "false" ] || selected+=(14)` test keeps it.
- Manual runs (Actions > *Tests* > *Run workflow*) run exactly what you
  check. Unchecking everything fails the run with `select at least one
  PostgreSQL version`.

The version list lives in three places that are not connected:
`PG_VERSIONS` in the Makefile, the checkboxes in `test.yml`, and the
`pg_versions` default in `ci.yml`. When you add a PostgreSQL version, update
all three.

### Supply chain

- Actions are pinned by commit SHA, with the version in a comment.
- Dependabot proposes Go modules and actions weekly, only once a release is
  15 days old (`cooldown` in `.github/dependabot.yml`).
- In `release.yml`, only the GoReleaser job can write; it signs the checksums
  with GPG and attests the build provenance of the archives.

### Reproducing a CI failure locally

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

### Running the workflows locally

[act](https://github.com/nektos/act) runs `test.yml`, and through it
`ci.yml`, in Docker. The runner image needs what `ubuntu-latest` has and the
jobs use: Node, the Docker CLI, git, `gcc` (for `-race`) and `unzip` (for
`setup-terraform`). The jobs reach the PostgreSQL containers on the host, so
they need the host network:

```bash
act workflow_dispatch -W .github/workflows/test.yml \
  -P ubuntu-latest=<runner image> --network host \
  --artifact-server-path /tmp/act-artifacts \
  --input pg14=false --input pg15=false --input pg16=false --input pg17=false
```

- The Codecov step is skipped under act (`ACT` is set), so a local run uploads
  nothing.
- act's artifact server does not accept `upload-artifact@v7` or
  `download-artifact@v8` yet (nektos/act#6114). For a local run, point a copy
  of `ci.yml` at v4 of both; keep v7 and v8 in the repository.
- The release cannot run under act: it needs the GPG key and GitHub's OIDC
  token for the provenance. To check the build and the signature, run
  GoReleaser with a throwaway key:
  `goreleaser release --snapshot --clean --skip=publish`.

## Environment variables

| Variable                | Default                  | Description |
|-------------------------|--------------------------|-------------|
| `TF_ACC`                | none                     | Must be set for acceptance tests to run (Terraform SDK convention). The Makefile sets it. |
| `PG_VERSIONS`           | `14 15 16 17 18`         | PostgreSQL versions to test (Makefile matrix variable). |
| `POSTGRES_IMAGE`        | `postgres:<v>-alpine`    | Container image override. |
| `TF_ACC_TERRAFORM_PATH` | `$(which terraform)`     | Path to the Terraform binary the SDK invokes. |
| `PGHOST`                | (testcontainer)          | If set, acceptance tests skip the container and use an external DB. |
| `PGPORT`                | (testcontainer)          | Port override when `PGHOST` is set. |
| `PGUSER`                | `postgres`               | Username for the external DB case. |
| `PGPASSWORD`            | `postgres`               | Password for the external DB case. |
| `PGDATABASE`            | `postgres`               | Default database. |
| `PGSSLMODE`             | `disable`                | SSL mode for the external DB case. |
| `GOTOOLCHAIN`           | `auto`                   | Go ≥ 1.21 auto-fetches the toolchain declared in `go.mod`. Set to `local` to disable. |

## How acceptance tests work

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

### `pq: sorry, too many clients already`

Add `-parallel 1`, or run fewer
`resource.TestStep`s at once. The container starts with
`max_connections=500`, but every step opens its own pool.

### `Error: Cannot connect to the Docker daemon`

Run `docker info`;
start Docker and retry.

### `govulncheck` reports stdlib CVEs

Newer Go patch releases fix these.
Raise the version in `go.mod`; the CI runner reads it through `setup-go@v6
with: go-version-file: go.mod`, and local Go ≥ 1.21 fetches the toolchain
for you.

### `tfplugindocs` reports missing documentation for every resource

It
takes the provider name from the directory name, so a clone in a directory
not named `terraform-provider-postgresql` breaks it. The Make targets and CI
pass `--provider-name postgresql`; do the same when you run it by hand.

### `tfplugindocs validate` fails after editing templates

Regenerate with `make docs`, then commit the regenerated `docs/`.
Do **not** hand-edit files in `docs/`: the next `make docs` run overwrites
them (see `CLAUDE.md`).

### `tfplugindocs` fails with `openpgp: key expired`

The full message is
`unable to download Terraform binary: unable to verify checksums
signature`. `tfplugindocs` needs Terraform to export the provider schema.
With no binary on `PATH`, it downloads one through `hc-install`, and that
download fails PGP checks against HashiCorp's expired signing key. Give it
a binary instead: CI runs `hashicorp/setup-terraform@v4` right before the
validate step. This never happens locally while `terraform` is on your
`PATH`, which is why `make ci-docs` can pass while CI fails.
