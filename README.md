# Terraform Provider PostgreSQL 🚧

> 🚧 **This provider is under active development.** Some features may change before the first stable release.

Terraform provider for managing PostgreSQL resources: roles, users, databases, schemas, and grants.

## Requirements

- [Terraform](https://www.terraform.io/downloads) >= 1.0
- [Go](https://go.dev/dl/) >= 1.26 (for building)
- PostgreSQL >= 14

## Installation

```hcl
terraform {
  required_providers {
    postgresql = {
      source  = "DiegoBulhoes/postgresql"
      version = "~> 0.2"
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
| [`postgresql_role`](docs/resources/role.md) | Manages roles (permission groups with inline privileges) |
| [`postgresql_user`](docs/resources/user.md) | Manages users (login roles with role memberships) |
| [`postgresql_database`](docs/resources/database.md) | Manages databases |
| [`postgresql_schema`](docs/resources/schema.md) | Manages schemas |
| [`postgresql_grant`](docs/resources/grant.md) | Manages GRANT privileges on objects |

## Data Sources

| Data Source | Description |
|---|---|
| [`postgresql_role`](docs/data-sources/role.md) | Reads role attributes |
| [`postgresql_user`](docs/data-sources/user.md) | Reads user attributes |
| [`postgresql_roles`](docs/data-sources/roles.md) | Lists roles with filters |
| [`postgresql_database`](docs/data-sources/database.md) | Reads database attributes |
| [`postgresql_schemas`](docs/data-sources/schemas.md) | Lists schemas with filters |
| [`postgresql_tables`](docs/data-sources/tables.md) | Lists tables with filters |
| [`postgresql_extensions`](docs/data-sources/extensions.md) | Lists installed extensions |
| [`postgresql_version`](docs/data-sources/version.md) | Reads server version info |
| [`postgresql_query`](docs/data-sources/query.md) | Executes a SQL query and returns results |

## Examples

### Create a role, user, and database

```hcl
resource "postgresql_role" "app_role" {
  name = "app_role"

  privilege {
    privileges  = ["SELECT", "INSERT", "UPDATE", "DELETE"]
    object_type = "table"
    schema      = "public"
  }
}

resource "postgresql_user" "app" {
  name     = "app_user"
  password = "secret"
  roles    = [postgresql_role.app_role.name]
}

resource "postgresql_database" "app" {
  name     = "app_db"
  owner    = postgresql_user.app.name
  encoding = "UTF8"
}
```

### Grant privileges

```hcl
resource "postgresql_schema" "app" {
  name  = "app_schema"
  owner = postgresql_user.app.name
}

resource "postgresql_grant" "app_schema" {
  role        = postgresql_user.app.name
  object_type = "schema"
  schema      = postgresql_schema.app.name
  privileges  = ["USAGE", "CREATE"]
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
make test             # Unit + acceptance tests (PG 14, 15, 16, 17)
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
├── test/
│   ├── acctest/                         # Acceptance test infrastructure
│   ├── mocks/                           # Generated GoMock mocks
│   ├── unit/                            # Unit tests (common, resource, datasource, provider)
│   └── integration/                     # Acceptance tests (resource, datasource, provider)
├── docs/                                # Generated documentation (do not edit)
├── templates/                           # Documentation templates
├── examples/                            # HCL examples used in docs
└── .github/workflows/
    ├── test.yml                         # CI: lint + tests
    └── release.yml                      # CD: GoReleaser
```

### Claude Code Skills

This project uses custom [Claude Code](https://github.com/DiegoBulhoes/claude) skills (`.claude/`) to assist with development:

| Skill | Description |
|---|---|
| `golang` | Go code generation following idiomatic conventions, with Terraform provider-specific patterns |
| `terraform` | Terraform/OpenTofu IaC code generation following HashiCorp official style guide |
| `explore` | Repository explorer for codebase analysis, dependency tracing, and gap reporting |

An agent configuration (`terraform-expert`) is also available for advanced Terraform provider development tasks.

## License

[LICENSE](LICENSE)
