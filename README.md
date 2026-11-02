# Terraform Provider PostgreSQL

> **This provider is under active development.** Some features may change before the first stable release.

Terraform provider for PostgreSQL. It manages roles, users, role memberships, databases, schemas, extensions, grants and default privileges.

## Requirements

- [Terraform](https://www.terraform.io/downloads) >= 1.5 (tested on 1.5.7 and the latest release)
- [Go](https://go.dev/dl/) >= 1.26 (for building)
- PostgreSQL 14 to 18

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

You can also configure the provider with environment variables: `PGHOST`, `PGPORT`, `PGUSER`, `PGPASSWORD`, `PGDATABASE`, `PGSSLMODE`.

## Resources

| Resource | Description |
|---|---|
| [`postgresql_role`](docs/resources/role.md) | Manages roles (permission groups with inline privileges) |
| [`postgresql_user`](docs/resources/user.md) | Manages users (login roles with role memberships) |
| [`postgresql_database`](docs/resources/database.md) | Manages databases |
| [`postgresql_schema`](docs/resources/schema.md) | Manages schemas |
| [`postgresql_grant`](docs/resources/grant.md) | Manages GRANT privileges on objects |
| [`postgresql_default_privileges`](docs/resources/default_privileges.md) | Manages privileges on objects created in the future |
| [`postgresql_role_membership`](docs/resources/role_membership.md) | Manages one role membership |
| [`postgresql_extension`](docs/resources/extension.md) | Manages extensions |

## Data sources

| Data source | Description |
|---|---|
| [`postgresql_role`](docs/data-sources/role.md) | Reads role attributes |
| [`postgresql_user`](docs/data-sources/user.md) | Reads user attributes |
| [`postgresql_roles`](docs/data-sources/roles.md) | Lists roles with filters |
| [`postgresql_database`](docs/data-sources/database.md) | Reads database attributes |
| [`postgresql_schemas`](docs/data-sources/schemas.md) | Lists schemas with filters |
| [`postgresql_tables`](docs/data-sources/tables.md) | Lists tables with filters |
| [`postgresql_extensions`](docs/data-sources/extensions.md) | Lists installed extensions |
| [`postgresql_version`](docs/data-sources/version.md) | Reads the server version |
| [`postgresql_query`](docs/data-sources/query.md) | Runs a SQL query and returns the rows |

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

Every resource and data source is documented in [`docs/`](docs/) and on the [Terraform Registry](https://registry.terraform.io/providers/DiegoBulhoes/postgresql/latest/docs).

## Development

### Setup

```bash
git clone https://github.com/DiegoBulhoes/terraform-provider-postgresql.git
cd terraform-provider-postgresql
make build
```

The Go tools (`golangci-lint`, `goimports`, `tfplugindocs`, `govulncheck`, `actionlint`) are declared in `go.mod` and resolved by `go tool`, so you do not need to install anything by hand.

### Build

```bash
make build
```

### Tests

Tests use [testcontainers-go](https://github.com/testcontainers/testcontainers-go) to start a PostgreSQL container. Docker must be running.

```bash
make test             # Unit + acceptance tests (PG 14 to 18)
```

Details are in [TESTING.md](TESTING.md).

### Lint and format

```bash
make lint   # golangci-lint (via go tool)
make fmt    # gofmt + goimports (via go tool)
```

### Generate documentation

```bash
make docs
```

The docs are generated from `templates/` and `examples/`. Never edit files in `docs/` by hand.

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
    ├── ci.yml                           # Checks, unit, acceptance matrix, coverage floor
    ├── test.yml                         # Runs ci.yml on PRs, main, weekly and by hand
    └── release.yml                      # Runs ci.yml, then GoReleaser and provenance
```

### Claude Code skills

This project ships custom [Claude Code](https://github.com/DiegoBulhoes/claude) skills in `.claude/`:

| Skill | Description |
|---|---|
| `golang` | Writes idiomatic Go, with Terraform provider patterns |
| `terraform` | Writes Terraform and OpenTofu code in the official HashiCorp style |
| `explore` | Explores the repository: analyses code, traces dependencies, reports gaps |

There is also a `terraform-expert` agent for harder provider work.

## License

[LICENSE](LICENSE)
