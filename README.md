# Terraform Provider PostgreSQL

> **Work in progress** — This provider is under active development and does not have a stable release yet. The API may change without notice.

Terraform provider for managing PostgreSQL resources: roles, databases, schemas, grants, and default privileges.

## Requirements

- [Terraform](https://www.terraform.io/downloads) >= 1.0
- [Go](https://go.dev/dl/) >= 1.25 (for building)
- PostgreSQL >= 12

## Usage

```hcl
terraform {
  required_providers {
    postgresql = {
      source  = "DiegoBulhoes/postgresql"
      version = "~> 0.1"
    }
  }
}

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

### Resources

| Resource | Description |
|---|---|
| `postgresql_role` | Manages roles (users/groups) |
| `postgresql_database` | Manages databases |
| `postgresql_schema` | Manages schemas |
| `postgresql_grant` | Manages GRANT privileges on objects |
| `postgresql_default_privileges` | Manages ALTER DEFAULT PRIVILEGES |

### Data Sources

| Data Source | Description |
|---|---|
| `postgresql_role` | Reads role attributes |
| `postgresql_database` | Reads database attributes |
| `postgresql_schemas` | Lists schemas with filters |
| `postgresql_query` | Executes a SELECT query and returns results |

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

See [docs/testing.md](docs/testing.md) for more details.

### Project structure

```
.
├── main.go                            # Provider entrypoint
├── internal/provider/
│   ├── provider.go                    # Provider configuration
│   ├── role_resource.go               # Resource: postgresql_role
│   ├── database_resource.go           # Resource: postgresql_database
│   ├── schema_resource.go             # Resource: postgresql_schema
│   ├── grant_resource.go              # Resource: postgresql_grant
│   ├── default_privileges_resource.go # Resource: postgresql_default_privileges
│   ├── role_data_source.go            # Data Source: postgresql_role
│   ├── database_data_source.go        # Data Source: postgresql_database
│   ├── schemas_data_source.go         # Data Source: postgresql_schemas
│   ├── query_data_source.go           # Data Source: postgresql_query
│   └── *_test.go                      # 58 acceptance tests
├── docs/
│   └── testing.md                     # Testing documentation
└── .github/workflows/
    ├── test.yml                       # CI: lint + tests
    └── release.yml                    # CD: GoReleaser
```

## License

[LICENSE](LICENSE)
