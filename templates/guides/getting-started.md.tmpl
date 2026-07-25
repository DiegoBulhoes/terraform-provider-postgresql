---
page_title: "Getting Started with the PostgreSQL Provider"
subcategory: "Guides"
description: |-
  How to configure the Terraform PostgreSQL provider and create your first user and database.
---

# Getting Started with the PostgreSQL Provider

This guide shows you how to configure the PostgreSQL provider and create your first resources.

## Prerequisites

- Terraform 1.0 or later
- A running PostgreSQL instance (local or remote)
- A PostgreSQL user with enough privileges: a superuser, or a role with `CREATEROLE` and `CREATEDB`

## Provider Configuration

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

### Using Environment Variables

Instead of writing credentials in the config, you can use environment variables. The provider falls back to them when the matching attribute is not set:

```shell
export PGHOST=localhost
export PGPORT=5432
export PGUSER=postgres
export PGPASSWORD=secret
export PGDATABASE=postgres
export PGSSLMODE=prefer
```

With the variables set, the provider block gets much shorter:

```terraform
provider "postgresql" {}
```

### Connection Tuning

The provider has several attributes for connection management. Every time value is a whole number of **seconds**.

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

- `max_connections` -- Maximum number of open connections to the database. Default: `10`.
- `max_idle_connections` -- Maximum number of idle connections in the pool. Default: `5`.
- `conn_max_lifetime` -- How long a connection can live, in seconds. Older connections are closed instead of reused. `0` means no limit. Default: `0`.
- `conn_max_idle_time` -- How long a connection can sit idle before it is closed, in seconds. `0` means no limit. Default: `0`.

~> **Note:** On managed PostgreSQL services like RDS, Cloud SQL, or Azure Database, set `superuser = false`. The provider then skips the operations that need superuser rights:
>
> ```terraform
> provider "postgresql" {
>   # ...
>   superuser = false
> }
> ```

### SSL Configuration

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

## Create Your First User

Users in PostgreSQL are login roles. Create one:

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

## Create Your First Database

Now create a database owned by the user you just created:

```terraform
resource "postgresql_database" "app_db" {
  name  = "my_application"
  owner = postgresql_user.app_user.name
}
```

## Verify the Results

After the apply, check that both objects exist:

```shell
psql -h localhost -U postgres -c "\du app_user"
psql -h localhost -U postgres -c "\l my_application"
```

## Reading Existing Resources

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

## Complete Example

Here is a full working configuration:

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

## Next Steps

- [Access Control Guide](access-control) -- How to set up roles and grants.
