# Testing

## Prerequisites

- Go 1.25+
- Docker (for testcontainers)

## How it works

Acceptance tests use [testcontainers-go](https://github.com/testcontainers/testcontainers-go) to spin up a PostgreSQL 16 container automatically. No manual database setup is needed.

If `PGHOST` is already set, the container is skipped and tests use the external database.

## Running tests

```bash
# All tests (unit + acceptance)
TF_ACC=1 go test ./... -timeout 600s

# Unit tests only (no Docker needed)
go test ./...

# Specific test
TF_ACC=1 go test ./internal/resource/ -run "TestAccPostgresqlRole_basic"

# Using Makefile
make testacc
```

## Coverage

```bash
TF_ACC=1 go test ./... -timeout 600s -coverprofile=coverage.out
go tool cover -func=coverage.out
go tool cover -html=coverage.out -o coverage.html
```

## Test structure

Tests are co-located with source files. Each package has:

- **Unit tests** (`go-sqlmock`) — error paths, no DB needed
- **Acceptance tests** (`TF_ACC=1`) — full flow with real PostgreSQL

```
internal/
├── common/          helpers_test.go
├── datasource/      *_data_source_test.go, mock_test.go
├── resource/        *_resource_test.go, mock_test.go, crud_mock_test.go
└── provider/        provider_test.go, provider_validation_test.go
```

## Environment variables

| Variable       | Default        | Description                                     |
| -------------- | -------------- | ----------------------------------------------- |
| `TF_ACC`       | —              | Required for acceptance tests                   |
| `PGHOST`       | (auto)         | PostgreSQL host. If set, skips testcontainer     |
| `PGPORT`       | (auto)         | PostgreSQL port                                 |
| `PGUSER`       | `postgres`     | Username                                        |
| `PGPASSWORD`   | `postgres`     | Password                                        |
| `PGDATABASE`   | `postgres`     | Default database                                |
| `PGSSLMODE`    | `disable`      | SSL mode                                        |

## Troubleshooting

**`pq: sorry, too many clients already`** — Use `-parallel 1`. The container starts with `max_connections=500` but each test step creates its own connections.

**Container won't start** — Check Docker is running: `docker info`
