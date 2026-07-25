# CLAUDE.md

## Project Overview

Terraform provider for PostgreSQL, built with `terraform-plugin-framework`. It manages roles, users, databases, schemas, and grants.

## Quick Commands

```bash
make all          # Full CI: tidy, lint, security, test, build, docs
make build        # Compile provider binary
make test         # Unit tests + acceptance tests (PG 14-17)
make lint         # fmt + vet + golangci-lint
make docs         # Generate and validate docs with tfplugindocs
make fmt          # gofmt + goimports
make security     # govulncheck
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
  resource/     # Resources: role, user, database, schema, grant
  datasource/   # Data sources: role, user, roles, database, schemas, tables, extensions, version, query
  common/       # Shared interfaces (DBTX, Scanner, Rows, Tx), helpers, RetryExec

test/
  acctest/      # Acceptance test infra (testcontainers, shared DB connection)
  mocks/        # Generated GoMock mocks for DBTX, Scanner, Rows, Tx
  unit/         # Unit tests (common, resource, datasource, provider)
  integration/  # Acceptance tests (resource, datasource, provider)
```

## Testing Conventions

- **Unit tests** (`test/unit/`): use `go.uber.org/mock` (GoMock) in external test packages, with the `_test` suffix.
- **Acceptance tests** (`test/integration/`): guarded by the `//go:build integration` tag. They use `testcontainers-go` to start real PostgreSQL containers.
- Tests run against PostgreSQL 14, 15, 16, 17.
- Mocks generated in `test/mocks/mock_db.go`.
- Mock generation: `mockgen -destination=test/mocks/mock_db.go -package=mocks github.com/DiegoBulhoes/terraform-provider-postgresql/internal/common DBTX,Scanner,Rows,Tx`

## Code Style

- Go 1.26+ with `terraform-plugin-framework` patterns (not the older SDKv2).
- Database access goes through the `common.DBTX` interface, so it can be mocked.
- Resource and data source types are exported, such as `RoleResource` and `DatabaseDataSource`, so external test packages can reach them.
- Linter config: `.golangci.yml` (errcheck, govet, ineffassign, staticcheck, unused, misspell).
- PostgreSQL driver: `github.com/lib/pq`.

## Documentation

- **Do not edit** files in `docs/`. Edit the templates in `templates/` and the examples in `examples/`, then run `make docs`.
