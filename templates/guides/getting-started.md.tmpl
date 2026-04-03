---
page_title: "Getting Started with the PostgreSQL Provider"
subcategory: "Guides"
description: |-
  Learn how to configure the Terraform PostgreSQL provider and create your first role and database.
---

# Getting Started with the PostgreSQL Provider

This guide walks you through configuring the PostgreSQL provider and creating your first managed resources.

## Prerequisites

- Terraform 1.0 or later
- A running PostgreSQL instance (local or remote)
- A PostgreSQL user with sufficient privileges (typically a superuser or a role with `CREATEROLE` and `CREATEDB`)

## Provider Configuration

Add the provider to your `required_providers` block and configure the connection:

```terraform
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
  password = "secret"
  database = "postgres"
  sslmode  = "prefer"
}
```

### Using Environment Variables

Instead of hardcoding credentials, you can use environment variables. The provider reads these as fallback values when the corresponding attribute is not set:

```shell
export PGHOST=localhost
export PGPORT=5432
export PGUSER=postgres
export PGPASSWORD=secret
export PGDATABASE=postgres
export PGSSLMODE=prefer
```

With environment variables set, the provider block can be simplified:

```terraform
provider "postgresql" {}
```

### Connection Tuning

The provider supports several attributes for connection management:

```terraform
provider "postgresql" {
  host     = "db.example.com"
  username = "postgres"
  password = var.db_password

  connect_timeout      = 30
  max_connections      = 4
  max_idle_connections  = 2
  conn_max_lifetime    = "30m"
  conn_max_idle_time   = "5m"
}
```

- `max_connections` -- Maximum number of open connections to the database.
- `max_idle_connections` -- Maximum number of idle connections in the pool.
- `conn_max_lifetime` -- Maximum amount of time a connection may be reused.
- `conn_max_idle_time` -- Maximum amount of time a connection may sit idle before being closed.

### SSL Configuration

For encrypted connections, configure the SSL-related attributes:

```terraform
provider "postgresql" {
  host       = "db.example.com"
  username   = "postgres"
  password   = var.db_password
  sslmode    = "verify-full"
  sslcert    = "/path/to/client-cert.pem"
  sslkey     = "/path/to/client-key.pem"
  sslrootcert = "/path/to/ca-cert.pem"
}
```

## Create Your First Role

Roles in PostgreSQL represent users and groups. Create a simple login role:

```terraform
resource "postgresql_role" "app_user" {
  name     = "app_user"
  login    = true
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

Now create a database owned by the role you just created:

```terraform
resource "postgresql_database" "app_db" {
  name  = "my_application"
  owner = postgresql_role.app_user.name
}
```

## Verify the Results

After applying, you can verify the resources were created:

```shell
psql -h localhost -U postgres -c "\du app_user"
psql -h localhost -U postgres -c "\l my_application"
```

## Reading Existing Resources

You can also use data sources to read existing PostgreSQL objects:

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

Here is a complete working configuration that ties everything together:

```terraform
terraform {
  required_providers {
    postgresql = {
      source  = "DiegoBulhoes/postgresql"
      version = "~> 0.1"
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

resource "postgresql_role" "app_user" {
  name     = "app_user"
  login    = true
  password = "changeme"
}

resource "postgresql_database" "app_db" {
  name  = "my_application"
  owner = postgresql_role.app_user.name
}

resource "postgresql_schema" "app_schema" {
  name     = "app"
  database = postgresql_database.app_db.name
  owner    = postgresql_role.app_user.name
}
```

## Next Steps

- [Access Control Guide](access-control) -- Learn how to set up roles, grants, and default privileges.
- [Managed PostgreSQL Guide](managed-postgresql) -- Connect to AWS RDS, GCP Cloud SQL, or Azure Database for PostgreSQL.
- [Importing Resources Guide](importing-resources) -- Import existing PostgreSQL infrastructure into Terraform.
