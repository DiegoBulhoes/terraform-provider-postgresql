# CLAUDE.md

## Project overview

Terraform provider for PostgreSQL, built with `terraform-plugin-framework`. It manages roles, users, role memberships, databases, schemas, extensions, grants and default privileges.

## Quick commands

```bash
make all          # Full CI: tidy, lint, security, test, build, docs
make build        # Compile provider binary
make test         # Unit tests + acceptance tests (PG 14-18)
make ci           # The checks and unit jobs of .github/workflows/ci.yml
make coverage     # Unit + acceptance coverage against the 90% floor
make lint         # fmt + vet + golangci-lint
make docs         # Generate and validate docs with tfplugindocs
make fmt          # gofmt + goimports
make security     # go tool govulncheck
```

Run a single unit test:
```bash
go test -run TestFunctionName ./test/unit/resource/...
```

Run acceptance tests for a specific PG version:
```bash
POSTGRES_IMAGE=postgres:17-alpine TF_ACC=1 TF_ACC_TERRAFORM_PATH=$(which terraform) \
  go test -tags integration -timeout 600s -count=1 ./...
```

## Architecture

```
internal/
  provider/     # Provider config (host, port, ssl, connection pooling)
  resource/     # Resources: role, user, role_membership, database, schema, extension, grant, default_privileges
  datasource/   # Data sources: role, user, roles, database, schemas, tables, extensions, version, query
  common/       # Shared interfaces (DBTX, Scanner, Rows, Tx, Stmt), helpers, per-database pools, ExecInTx

test/
  acctest/      # Acceptance test infra (testcontainers, shared DB connection)
  mocks/        # Generated GoMock mocks for DBTX, Scanner, Rows, Tx, Stmt
  unit/         # Unit tests (common, resource, datasource, provider)
  integration/  # Acceptance tests (resource, datasource, provider)
```

## Testing conventions

- Unit tests (`test/unit/`) use `go.uber.org/mock` (GoMock) in external test packages, with the `_test` suffix.
- Acceptance tests (`test/integration/`) are guarded by the `//go:build integration` tag and use `testcontainers-go` to start real PostgreSQL containers.
- Tests run against PostgreSQL 14 to 18, and on Terraform 1.5.7 (the oldest supported) as well as the latest.
- Unit and acceptance coverage together must stay at or above 90% (`scripts/coverage.sh`).
- Mocks are generated in `test/mocks/mock_db.go`.
- Mock generation: `go run go.uber.org/mock/mockgen -destination=test/mocks/mock_db.go -package=mocks github.com/DiegoBulhoes/terraform-provider-postgresql/internal/common DBTX,Scanner,Rows,Tx,Stmt`

## Code style

- Go 1.26+ with `terraform-plugin-framework` patterns (not the older SDKv2).
- Database access goes through the `common.DBTX` interface, so it can be mocked.
- Resource and data source types are exported, such as `RoleResource` and `DatabaseDataSource`, so external test packages can reach them.
- Linter config: `.golangci.yml` (errcheck, govet, ineffassign, staticcheck, unused, misspell, gosec, gocritic, unparam), also run on the `integration` build tag.
- Statements that must apply together go through `common.ExecInTx`; a resource attribute `database` reaches its database through `common.OnDatabase`.
- PostgreSQL driver: `github.com/lib/pq`.

## Documentation

- **Do not edit** files in `docs/`. Edit the templates in `templates/` and the examples in `examples/`, then run `make docs`.
