---
page_title: "Getting Started with the PostgreSQL Provider"
subcategory: "Guides"
description: |-
  How to configure the Terraform PostgreSQL provider and create your first user and database.
---

# Getting Started with the PostgreSQL Provider

This guide configures the PostgreSQL provider and creates a first user and database.

## Prerequisites

- Terraform 1.0 or later
- A running PostgreSQL instance (local or remote)
- A PostgreSQL user with enough privileges: a superuser, or a role with `CREATEROLE` and `CREATEDB`

## Provider configuration

Add the provider to your `required_providers` block and configure the connection:

```terraform
terraform {
  required_providers {
    postgresql = {
      source  = "DiegoBulhoes/postgresql"
      version = "~> 0.2"
    }
  }
}

provider "postgresql" {
  host     = "localhost"
  port     = 5432
  username = "postgres"
  password = "secret"
  database = "postgres"
  sslmode  = "prefer"
}
```

### Using environment variables

You can keep credentials out of the config with environment variables. The provider falls back to them when the matching attribute is not set:

```shell
export PGHOST=localhost
export PGPORT=5432
export PGUSER=postgres
export PGPASSWORD=secret
export PGDATABASE=postgres
export PGSSLMODE=prefer
```

With the variables set, the provider block can be empty:

```terraform
provider "postgresql" {}
```

### Connection tuning

These attributes control the connection pool. Every time value is a whole number of seconds.

```terraform
provider "postgresql" {
  host     = "db.example.com"
  username = "postgres"
  password = var.db_password

  connect_timeout      = 30     # seconds to wait for the initial connection
  max_connections      = 10     # default
  max_idle_connections = 5      # default
  conn_max_lifetime    = 1800   # seconds (0 = unlimited)
  conn_max_idle_time   = 300    # seconds (0 = unlimited)
}
```

- `max_connections`: maximum number of open connections to the database. Default: `10`.
- `max_idle_connections`: maximum number of idle connections in the pool. Default: `5`.
- `conn_max_lifetime`: how long a connection can live, in seconds. Older connections are closed, not reused. `0` means no limit. Default: `0`.
- `conn_max_idle_time`: how long a connection can sit idle before it is closed, in seconds. `0` means no limit. Default: `0`.

~> **Note:** On managed PostgreSQL services such as RDS, Cloud SQL, or Azure Database, a user with `CREATEROLE` is enough. The provider sends role flags such as `SUPERUSER` only when they change, so it never asks for superuser rights it does not need.

### SSL configuration

For encrypted connections, set the SSL attributes:

```terraform
provider "postgresql" {
  host        = "db.example.com"
  username    = "postgres"
  password    = var.db_password
  sslmode     = "verify-full"
  sslcert     = "/path/to/client-cert.pem"
  sslkey      = "/path/to/client-key.pem"
  sslrootcert = "/path/to/ca-cert.pem"
}
```

## Create your first user

A PostgreSQL user is a login role:

```terraform
resource "postgresql_user" "app_user" {
  name     = "app_user"
  password = var.app_user_password
}
```

Apply the configuration:

```shell
terraform init
terraform plan
terraform apply
```

## Create your first database

Create a database owned by that user:

```terraform
resource "postgresql_database" "app_db" {
  name  = "my_application"
  owner = postgresql_user.app_user.name
}
```

## Verify the results

Check that both objects exist:

```shell
psql -h localhost -U postgres -c "\du app_user"
psql -h localhost -U postgres -c "\l my_application"
```

## Reading existing resources

Data sources read PostgreSQL objects that already exist:

```terraform
data "postgresql_role" "existing" {
  name = "postgres"
}

data "postgresql_database" "existing" {
  name = "postgres"
}

output "postgres_role_id" {
  value = data.postgresql_role.existing.id
}
```

## Complete example

A full working configuration:

```terraform
terraform {
  required_providers {
    postgresql = {
      source  = "DiegoBulhoes/postgresql"
      version = "~> 0.2"
    }
  }
}

variable "db_password" {
  type      = string
  sensitive = true
}

provider "postgresql" {
  host     = "localhost"
  port     = 5432
  username = "postgres"
  password = var.db_password
  sslmode  = "prefer"
}

resource "postgresql_user" "app_user" {
  name     = "app_user"
  password = var.db_password
}

resource "postgresql_database" "app_db" {
  name  = "my_application"
  owner = postgresql_user.app_user.name
}

resource "postgresql_schema" "app_schema" {
  name     = "app"
  database = postgresql_database.app_db.name
  owner    = postgresql_user.app_user.name
}
```

## Next steps

- [Access Control Guide](access-control): setting up roles and grants.
