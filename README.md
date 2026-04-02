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
| [`postgresql_database`](docs/data-sources/database.md) | Reads database attributes |
| [`postgresql_schemas`](docs/data-sources/schemas.md) | Lists schemas with filters |
| [`postgresql_query`](docs/data-sources/query.md) | Executes a SELECT query and returns results |

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

### Build

```bash
make build
```

### Tests

Tests use [testcontainers-go](https://github.com/testcontainers/testcontainers-go) to automatically spin up a PostgreSQL instance via Docker.

```bash
# Run all acceptance tests
make testacc

# With coverage
make testacc-cover

# Generate coverage HTML report
make cover-html
```

See [TESTING.md](TESTING.md) for more details.

### Generate documentation

```bash
# Install tfplugindocs
go install github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs@latest

# Generate docs from templates and schema
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
