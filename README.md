# Terraform Provider PostgreSQL 🚧

> 🚧 **This provider is under active development.** Some features may change before the first stable release.

Terraform provider for managing PostgreSQL resources: roles, databases, schemas, grants, and default privileges.

## Requirements

- [Terraform](https://www.terraform.io/downloads) >= 1.0
- [Go](https://go.dev/dl/) >= 1.25 (for building)
- PostgreSQL >= 12

## Installation

```hcl
terraform {
  required_providers {
    postgresql = {
      source  = "DiegoBulhoes/postgresql"
      version = "~> 0.1"
    }
  }
}
```

## Usage

```hcl
provider "postgresql" {
  host     = "localhost"
  port     = 5432
  username = "postgres"
  password = "postgres"
  database = "postgres"
  sslmode  = "disable"
}
```

The provider also accepts configuration via environment variables: `PGHOST`, `PGPORT`, `PGUSER`, `PGPASSWORD`, `PGDATABASE`, `PGSSLMODE`.

## Resources

| Resource | Description |
|---|---|
| [`postgresql_role`](docs/resources/role.md) | Manages roles (users/groups) |
| [`postgresql_database`](docs/resources/database.md) | Manages databases |
| [`postgresql_schema`](docs/resources/schema.md) | Manages schemas |
| [`postgresql_grant`](docs/resources/grant.md) | Manages GRANT privileges on objects |
| [`postgresql_default_privileges`](docs/resources/default_privileges.md) | Manages ALTER DEFAULT PRIVILEGES |

## Data Sources

| Data Source | Description |
|---|---|
| [`postgresql_role`](docs/data-sources/role.md) | Reads role attributes |
| [`postgresql_roles`](docs/data-sources/roles.md) | Lists roles with filters |
| [`postgresql_database`](docs/data-sources/database.md) | Reads database attributes |
| [`postgresql_schemas`](docs/data-sources/schemas.md) | Lists schemas with filters |
| [`postgresql_tables`](docs/data-sources/tables.md) | Lists tables with filters |
| [`postgresql_extensions`](docs/data-sources/extensions.md) | Lists installed extensions |
| [`postgresql_version`](docs/data-sources/version.md) | Reads server version info |
| [`postgresql_query`](docs/data-sources/query.md) | Executes a SQL query and returns results |

## Examples

### Create a role and database

```hcl
resource "postgresql_role" "app" {
  name            = "app_user"
  login           = true
  password        = "secret"
  create_database = false
}

resource "postgresql_database" "app" {
  name     = "app_db"
  owner    = postgresql_role.app.name
  encoding = "UTF8"
}
```

### Grant privileges

```hcl
resource "postgresql_schema" "app" {
  name  = "app_schema"
  owner = postgresql_role.app.name
}

resource "postgresql_grant" "app_schema" {
  role        = postgresql_role.app.name
  object_type = "schema"
  schema      = postgresql_schema.app.name
  privileges  = ["USAGE", "CREATE"]
}

resource "postgresql_default_privileges" "tables" {
  owner       = postgresql_role.app.name
  role        = postgresql_role.app.name
  database    = postgresql_database.app.name
  schema      = postgresql_schema.app.name
  object_type = "table"
  privileges  = ["SELECT", "INSERT", "UPDATE", "DELETE"]
}
```

### Query data

```hcl
data "postgresql_role" "current" {
  name = "postgres"
}

data "postgresql_schemas" "app" {
  like_pattern           = "app_%"
  include_system_schemas = false
}

data "postgresql_query" "version" {
  database = "postgres"
  query    = "SELECT version() AS pg_version"
}
```

## Documentation

Full documentation for each resource and data source is available in the [`docs/`](docs/) directory and on the [Terraform Registry](https://registry.terraform.io/providers/DiegoBulhoes/postgresql/latest/docs).

## Development

### Setup

```bash
git clone https://github.com/DiegoBulhoes/terraform-provider-postgresql.git
cd terraform-provider-postgresql
make build
```

That's it. All Go tools (`golangci-lint`, `goimports`, `tfplugindocs`) are declared in `go.mod` and resolved automatically via `go tool` — no manual installation needed.

### Build

```bash
make build
```

### Tests

Tests use [testcontainers-go](https://github.com/testcontainers/testcontainers-go) to automatically spin up a PostgreSQL instance via Docker. Requires Docker running.

```bash
make test             # Unit tests only (no Docker needed)
make testacc          # Acceptance tests (PG 14, 15, 16, 17)
make testacc-cover    # With coverage
make cover-html       # Generate HTML coverage report
```

See [TESTING.md](TESTING.md) for more details.

### Lint & Format

```bash
make lint   # golangci-lint (via go tool)
make fmt    # gofmt + goimports (via go tool)
```

### Generate documentation

```bash
make docs
```

Documentation is generated from templates in `templates/` and examples in `examples/`. Do not edit files in `docs/` directly.

### Project structure

```
.
├── main.go                              # Provider entrypoint
├── internal/
│   ├── provider/provider.go             # Provider configuration
│   ├── resource/                        # Resources
│   ├── datasource/                      # Data Sources
│   └── common/                          # Shared helpers
├── docs/                                # Generated documentation (do not edit)
├── templates/                           # Documentation templates
├── examples/                            # HCL examples used in docs
└── .github/workflows/
    ├── test.yml                         # CI: lint + tests
    └── release.yml                      # CD: GoReleaser
```

## License

[LICENSE](LICENSE)
