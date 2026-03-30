# Terraform Provider PostgreSQL

> **Em construcao** — Este provider esta em desenvolvimento ativo e ainda nao possui uma release estavel. A API pode mudar sem aviso previo.

Terraform provider para gerenciar recursos PostgreSQL: roles, databases, schemas, grants e default privileges.

## Requisitos

- [Terraform](https://www.terraform.io/downloads) >= 1.0
- [Go](https://go.dev/dl/) >= 1.25 (para build)
- PostgreSQL >= 12

## Uso

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

O provider tambem aceita configuracao via variaveis de ambiente: `PGHOST`, `PGPORT`, `PGUSER`, `PGPASSWORD`, `PGDATABASE`, `PGSSLMODE`.

## Recursos

### Resources

| Resource | Descricao |
|---|---|
| `postgresql_role` | Gerencia roles (usuarios/grupos) |
| `postgresql_database` | Gerencia databases |
| `postgresql_schema` | Gerencia schemas |
| `postgresql_grant` | Gerencia GRANT de privilegios em objetos |
| `postgresql_default_privileges` | Gerencia ALTER DEFAULT PRIVILEGES |

### Data Sources

| Data Source | Descricao |
|---|---|
| `postgresql_role` | Consulta atributos de um role |
| `postgresql_database` | Consulta atributos de um database |
| `postgresql_schemas` | Lista schemas com filtros |
| `postgresql_query` | Executa SELECT e retorna resultados |

## Exemplos

### Criar role e database

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

### Grant de privilegios

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

### Consultar dados

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

## Desenvolvimento

### Build

```bash
make build
```

### Testes

Os testes usam [testcontainers-go](https://github.com/testcontainers/testcontainers-go) para subir um PostgreSQL automaticamente via Docker.

```bash
# Rodar todos os testes de aceitacao
make testacc

# Com coverage
make testacc-cover

# Gerar HTML de coverage
make cover-html
```

Veja [docs/testing.md](docs/testing.md) para mais detalhes.

### Estrutura

```
.
├── main.go                          # Entrypoint do provider
├── internal/provider/
│   ├── provider.go                  # Configuracao do provider
│   ├── role_resource.go             # Resource: postgresql_role
│   ├── database_resource.go         # Resource: postgresql_database
│   ├── schema_resource.go          # Resource: postgresql_schema
│   ├── grant_resource.go            # Resource: postgresql_grant
│   ├── default_privileges_resource.go # Resource: postgresql_default_privileges
│   ├── role_data_source.go          # Data Source: postgresql_role
│   ├── database_data_source.go      # Data Source: postgresql_database
│   ├── schemas_data_source.go       # Data Source: postgresql_schemas
│   ├── query_data_source.go         # Data Source: postgresql_query
│   └── *_test.go                    # 58 testes de aceitacao
├── docs/
│   └── testing.md                   # Documentacao de testes
└── .github/workflows/
    ├── test.yml                     # CI: lint + testes
    └── release.yml                  # CD: GoReleaser
```

## License

[LICENSE](LICENSE)
