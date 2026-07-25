---
page_title: "Access Control with Roles and Grants"
subcategory: "Guides"
description: |-
  How to manage PostgreSQL access control with roles and grants in Terraform.
---

# Access Control with Roles and Grants

This guide shows a realistic access control setup for a PostgreSQL application database. You create an owner user, a read-only role, and an application user, then grant the privileges each one needs. The `for_each` pattern used here works for any number of schemas.

## Overview

A common PostgreSQL access pattern uses three types of roles:

1. **Owner role** -- Owns the database and schemas. Creates tables and other objects. Used during migrations.
2. **Read-only role** -- Can read all data but cannot modify anything. Used for reporting, analytics, and debugging.
3. **Application role** -- Can read and write data but cannot alter schema structure. Used by the running application.

~> **Watch out for these three things.**
>
> - Grants on `ALL TABLES`/`ALL SEQUENCES`/`ALL FUNCTIONS` affect only *existing* objects. For future objects, use `ALTER DEFAULT PRIVILEGES` (see Step 6).
> - By default, the `PUBLIC` pseudo-role has `CONNECT` on new databases and `CREATE` on the `public` schema. Revoke both if you want everything closed by default (see Step 2).
> - `postgresql_grant` on `ALL` objects (an empty `objects` list on `table`/`sequence`/`function`) does not check drift per object. Pass `objects = [...]` when you need it.

## Variables

Define the sensitive variables up front:

```terraform
variable "owner_password" {
  type      = string
  sensitive = true
}

variable "app_password" {
  type      = string
  sensitive = true
}

variable "analyst_password" {
  type      = string
  sensitive = true
}

locals {
  database_name = "myapp"
  # Schemas managed by the owner. The readonly role gets USAGE+SELECT on
  # all of them; the application role gets USAGE+CRUD only on the ones
  # marked `app = true`.
  schemas = {
    public   = { app = true }
    api      = { app = true }
    internal = { app = false }
  }
  app_schemas      = { for k, v in local.schemas : k => v if v.app }
  readonly_schemas = local.schemas
}
```

~> **Production note.** Passwords end up in Terraform state. Use a state backend that encrypts data at rest (S3 with SSE-KMS, GCS with CMEK, or Terraform Cloud) and limit who can read it. For people, like the `analyst` below, use IAM or SSO login instead of a password when your PostgreSQL supports it.

## Step 1: Create the Users and Roles

In PostgreSQL, `postgresql_user` represents login users and `postgresql_role` represents permission groups (NOLOGIN). Users can be members of roles to inherit their privileges.

```terraform
# Owner user: runs migrations, creates tables
resource "postgresql_user" "owner" {
  name            = "app_owner"
  password        = var.owner_password
  create_database = true
}

# Read-only group role (permission group -- individual users inherit from it)
resource "postgresql_role" "readonly" {
  name = "app_readonly"
}

# Application user: used by the running service
resource "postgresql_user" "app" {
  name     = "app_service"
  password = var.app_password
}

# A human user who inherits the readonly role
resource "postgresql_user" "analyst" {
  name     = "analyst"
  password = var.analyst_password
  roles    = [postgresql_role.readonly.name]
}
```

~> **Alternative: inline `privilege` blocks on `postgresql_role`.** Instead of many `postgresql_grant` resources for one permission group, you can declare the grants on the role itself. This is shorter but less flexible:
>
> ```terraform
> resource "postgresql_role" "readonly" {
>   name = "app_readonly"
>
>   privilege {
>     object_type = "database"
>     database    = "myapp"
>     privileges  = ["CONNECT"]
>   }
>   privilege {
>     object_type = "schema"
>     schema      = "public"
>     privileges  = ["USAGE"]
>   }
> }
> ```
>
> The separate `postgresql_grant` approach used below is longer, but each grant can be managed, imported, and checked for drift on its own.

## Step 2: Create the Database and Schemas

```terraform
resource "postgresql_database" "app" {
  name     = local.database_name
  owner    = postgresql_user.owner.name
  template = "template0"
  encoding = "UTF8"
}

resource "postgresql_schema" "app" {
  for_each = local.schemas

  name     = each.key
  database = postgresql_database.app.name
  owner    = postgresql_user.owner.name
}
```

### Revoke default `PUBLIC` privileges (closed-by-default)

By default, PostgreSQL grants `CONNECT` on new databases and `CREATE` on the `public` schema to the `PUBLIC` pseudo-role. That means every role on the server can connect and create objects. To close this, revoke both before your own grants take effect. The `postgresql_query` data source runs any SQL when `allow_destructive = true`:

```terraform
data "postgresql_query" "revoke_public_defaults" {
  database          = postgresql_database.app.name
  allow_destructive = true

  query = <<-SQL
    REVOKE CONNECT ON DATABASE ${postgresql_database.app.name} FROM PUBLIC;
    REVOKE CREATE  ON SCHEMA   public                         FROM PUBLIC;
  SQL
}
```

~> **Run this once, early.** Revoking PUBLIC's default privileges is a one-time step. Running it again does nothing. Do it before Steps 3-6, so the permissive defaults don't hide the CONNECT and USAGE grants you set yourself.

## Step 3: Grant Database-Level Privileges

```terraform
resource "postgresql_grant" "connect" {
  for_each = {
    readonly = postgresql_role.readonly.name
    app      = postgresql_user.app.name
  }

  role        = each.value
  database    = postgresql_database.app.name
  object_type = "database"
  privileges  = ["CONNECT"]
}
```

## Step 4: Grant Schema-Level Privileges (USAGE)

`for_each` over the schema map keeps the config short, no matter how many schemas you add later:

```terraform
# Readonly: USAGE on every schema
resource "postgresql_grant" "readonly_schema_usage" {
  for_each = local.readonly_schemas

  role        = postgresql_role.readonly.name
  database    = postgresql_database.app.name
  schema      = each.key
  object_type = "schema"
  privileges  = ["USAGE"]
}

# Application: USAGE only on the schemas it needs (app = true)
resource "postgresql_grant" "app_schema_usage" {
  for_each = local.app_schemas

  role        = postgresql_user.app.name
  database    = postgresql_database.app.name
  schema      = each.key
  object_type = "schema"
  privileges  = ["USAGE"]
}
```

## Step 5: Grant Object-Level Privileges (existing objects)

```terraform
# Readonly: SELECT on all existing tables in every managed schema
resource "postgresql_grant" "readonly_tables" {
  for_each = local.readonly_schemas

  role        = postgresql_role.readonly.name
  database    = postgresql_database.app.name
  schema      = each.key
  object_type = "table"
  privileges  = ["SELECT"]
}

# Application: full CRUD on tables in its schemas
resource "postgresql_grant" "app_tables" {
  for_each = local.app_schemas

  role        = postgresql_user.app.name
  database    = postgresql_database.app.name
  schema      = each.key
  object_type = "table"
  privileges  = ["SELECT", "INSERT", "UPDATE", "DELETE"]
}

# Application: sequence access (needed for serial/identity columns)
resource "postgresql_grant" "app_sequences" {
  for_each = local.app_schemas

  role        = postgresql_user.app.name
  database    = postgresql_database.app.name
  schema      = each.key
  object_type = "sequence"
  privileges  = ["USAGE", "SELECT"]
}
```

~> **Drift detection limit.** The grants above leave out `objects = [...]`, so they apply to *all* tables, sequences, and functions in the schema (`GRANT ... ON ALL TABLES IN SCHEMA`). For these `ALL` grants the provider **does not** check drift per object, because it would have to list every object on every refresh. To get drift detection on specific objects, name them: `objects = ["orders", "invoices", ...]`.

## Step 6: Grant Privileges on *Future* Objects

Step 5 covers tables that already exist. PostgreSQL will not grant the same privileges on tables the owner creates *later*. Use `ALTER DEFAULT PRIVILEGES` so new objects follow the same rules:

```terraform
data "postgresql_query" "default_privileges" {
  database          = postgresql_database.app.name
  allow_destructive = true

  query = <<-SQL
    -- Readonly: SELECT on future tables in every managed schema
    %{for schema in keys(local.readonly_schemas)~}
    ALTER DEFAULT PRIVILEGES FOR ROLE ${postgresql_user.owner.name}
      IN SCHEMA ${schema}
      GRANT SELECT ON TABLES TO ${postgresql_role.readonly.name};
    %{endfor~}

    -- App: CRUD on future tables, USAGE/SELECT on future sequences
    %{for schema in keys(local.app_schemas)~}
    ALTER DEFAULT PRIVILEGES FOR ROLE ${postgresql_user.owner.name}
      IN SCHEMA ${schema}
      GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO ${postgresql_user.app.name};
    ALTER DEFAULT PRIVILEGES FOR ROLE ${postgresql_user.owner.name}
      IN SCHEMA ${schema}
      GRANT USAGE, SELECT ON SEQUENCES TO ${postgresql_user.app.name};
    %{endfor~}
  SQL

  depends_on = [
    postgresql_schema.app,
  ]
}
```

`ALTER DEFAULT PRIVILEGES FOR ROLE ${owner}` only applies to objects created *by that role*. Always name the role. The default is the user running the `ALTER`, which is rarely what you want. See [ALTER DEFAULT PRIVILEGES](https://www.postgresql.org/docs/current/sql-alterdefaultprivileges.html) for the full syntax.

## Delegating Grants with `with_grant_option`

If a role needs to pass its privileges on to others, set `with_grant_option = true`. This is useful for a platform team that manages access without being superuser:

```terraform
resource "postgresql_grant" "platform_admin" {
  role              = "platform_admin"
  database          = postgresql_database.app.name
  object_type       = "database"
  privileges        = ["CONNECT", "CREATE", "TEMPORARY"]
  with_grant_option = true
}
```

## Summary

This setup gives you a layered access control model:

| User / Role | Type | Database | Schemas (USAGE) | Tables | Sequences | Future objects |
|-------------|------|----------|-----------------|--------|-----------|----------------|
| `app_owner` | User (owner) | owns | owns | owns | owns | N/A (creator) |
| `app_readonly` | Role (group) | CONNECT | all managed | SELECT (all) | -- | SELECT via `ALTER DEFAULT PRIVILEGES` |
| `app_service` | User | CONNECT | `public`, `api` | CRUD | USAGE, SELECT | CRUD + sequences via `ALTER DEFAULT PRIVILEGES` |
| `analyst` | User | inherits from `app_readonly` | inherits | inherits | inherits | inherits |
| `PUBLIC` | Pseudo-role | revoked (Step 2) | revoked on `public` | -- | -- | -- |

With `for_each` over `local.schemas`, adding a schema takes one line in the `locals` block. Every grant follows automatically.

## Next Steps

- [Getting Started Guide](getting-started) -- Basic provider setup.
