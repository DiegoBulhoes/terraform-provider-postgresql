# Testing

## Prerequisites

- Go 1.26+
- Docker (for testcontainers)

## How it works

Acceptance tests use [testcontainers-go](https://github.com/testcontainers/testcontainers-go) to spin up a PostgreSQL container automatically. No manual database setup is needed.

Acceptance tests are guarded by `//go:build integration` — they only compile with `-tags integration`. This keeps `govulncheck` clean from test-only dependencies (testcontainers/docker).

If `PGHOST` is already set, the container is skipped and tests use the external database.

## Running tests

```bash
# Unit tests only (no Docker needed)
make test

# Acceptance tests (PG 14, 15, 16, 17 sequentially)
make testacc

# Acceptance tests for specific PG versions
make testacc PG_VERSIONS="16 17"

# With coverage
make testacc-cover
make cover-html
```

## Test structure

Tests are co-located with source files. Each package has:

- **Unit tests** (`go.uber.org/mock`) — error paths, no DB needed
- **Acceptance tests** (`-tags integration`) — full flow with real PostgreSQL

```
internal/
├── common/          helpers_test.go
├── datasource/      *_data_source_test.go, mock_test.go
├── resource/        *_resource_test.go, mock_test.go, crud_mock_test.go
└── provider/        provider_test.go, provider_validation_test.go
```

## Environment variables

| Variable         | Default    | Description                                 |
| ---------------- | ---------- | ------------------------------------------- |
| `TF_ACC`         | —          | Required for acceptance tests               |
| `PG_VERSIONS`    | `14 15 16 17` | PostgreSQL versions to test (Makefile)   |
| `POSTGRES_IMAGE` | `postgres:16-alpine` | Container image override           |
| `PGHOST`         | (auto)     | PostgreSQL host. If set, skips testcontainer |
| `PGPORT`         | (auto)     | PostgreSQL port                             |
| `PGUSER`         | `postgres` | Username                                    |
| `PGPASSWORD`     | `postgres` | Password                                    |
| `PGDATABASE`     | `postgres` | Default database                            |
| `PGSSLMODE`      | `disable`  | SSL mode                                    |

## Troubleshooting

**`pq: sorry, too many clients already`** — Use `-parallel 1`. The container starts with `max_connections=500` but each test step creates its own connections.

**Container won't start** — Check Docker is running: `docker info`
