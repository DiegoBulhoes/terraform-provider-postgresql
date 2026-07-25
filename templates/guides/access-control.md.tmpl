---
page_title: "Access Control with Roles and Grants"
subcategory: "Guides"
description: |-
  A complete guide to managing PostgreSQL access control using roles and grants with Terraform.
---

# Access Control with Roles and Grants

This guide demonstrates a realistic access control setup for a PostgreSQL application database. You will create an owner user, a read-only role, and an application user, then grant appropriate privileges — using a DRY `for_each` pattern that scales cleanly to many schemas.

## Overview

A common PostgreSQL access pattern uses three types of roles:

1. **Owner role** -- Owns the database and schemas. Creates tables and other objects. Used during migrations.
2. **Read-only role** -- Can read all data but cannot modify anything. Used for reporting, analytics, and debugging.
3. **Application role** -- Can read and write data but cannot alter schema structure. Used by the running application.

~> **Important gotchas covered below.**
>
> - Grants on `ALL TABLES`/`ALL SEQUENCES`/`ALL FUNCTIONS` affect only *existing* objects. Use `ALTER DEFAULT PRIVILEGES` for future objects (see Step 6).
> - The `PUBLIC` pseudo-role has `CONNECT` on new databases and `CREATE` on the `public` schema by default. Revoke those if you want a closed-by-default posture (see Step 2).
> - `postgresql_grant` on `ALL` objects (empty `objects` list on `table`/`sequence`/`function`) skips per-object drift detection. Pass `objects = [...]` when drift matters.

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

~> **Production note.** Passwords end up in Terraform state. Use a state backend with encryption at rest (e.g. S3 with SSE-KMS, GCS with CMEK, or Terraform Cloud) and restrict read access. For human users (like the `analyst` below), prefer IAM/SSO-backed authentication over passwords where the PostgreSQL deployment supports it.

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

~> **Alternative: inline `privilege` blocks on `postgresql_role`.** Instead of managing many `postgresql_grant` resources for a permission group, you can declare grants directly on the role. This trades flexibility for conciseness:
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
> The separate-`postgresql_grant` approach used below is more verbose but lets each grant be managed, imported, and drift-detected independently.

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

By default PostgreSQL grants `CONNECT` on new databases and `CREATE` on the `public` schema to the `PUBLIC` pseudo-role, meaning every role on the server can connect and create objects. For a closed-by-default posture, revoke those before your explicit grants take effect. The `postgresql_query` data source runs arbitrary SQL when `allow_destructive = true`:

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

~> **Run this once, early.** Revoking PUBLIC's default privileges is a one-time hardening step. If the revoke has already been applied, re-running it is a no-op. Schedule it before Steps 3–6 so your explicit CONNECT/USAGE grants aren't shadowed by the permissive defaults.

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

Using `for_each` over the schema map keeps the configuration DRY regardless of how many schemas you add later:

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

~> **Drift detection limitation.** The grants above omit `objects = [...]`, which means they apply to *all* tables/sequences/functions in the schema (`GRANT ... ON ALL TABLES IN SCHEMA`). The provider **does not** check per-object drift for `ALL`-style grants — it can't practically enumerate every object on every refresh. If you need drift detection on specific objects, list them explicitly with `objects = ["orders", "invoices", ...]`.

## Step 6: Grant Privileges on *Future* Objects

Step 5 covers existing tables, but PostgreSQL won't automatically grant those same privileges on tables the owner creates *later*. Use `ALTER DEFAULT PRIVILEGES` so new objects inherit the pattern:

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

`ALTER DEFAULT PRIVILEGES FOR ROLE ${owner}` is scoped to objects created *by that role*. Always name the role explicitly — the default (current user running the `ALTER`) is rarely what you want. See [ALTER DEFAULT PRIVILEGES](https://www.postgresql.org/docs/current/sql-alterdefaultprivileges.html) for the full syntax.

## Delegating Grants with `with_grant_option`

If a role needs to grant its privileges to others (useful for a platform team that manages access without being superuser), set `with_grant_option = true`:

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

This configuration implements a layered, DRY access control model:

| User / Role | Type | Database | Schemas (USAGE) | Tables | Sequences | Future objects |
|-------------|------|----------|-----------------|--------|-----------|----------------|
| `app_owner` | User (owner) | owns | owns | owns | owns | N/A (creator) |
| `app_readonly` | Role (group) | CONNECT | all managed | SELECT (all) | -- | SELECT via `ALTER DEFAULT PRIVILEGES` |
| `app_service` | User | CONNECT | `public`, `api` | CRUD | USAGE, SELECT | CRUD + sequences via `ALTER DEFAULT PRIVILEGES` |
| `analyst` | User | inherits from `app_readonly` | inherits | inherits | inherits | inherits |
| `PUBLIC` | Pseudo-role | revoked (Step 2) | revoked on `public` | -- | -- | -- |

The `for_each` pattern over `local.schemas` means adding a new schema takes a single line in the `locals` block — every grant adjusts automatically.

## Next Steps

- [Getting Started Guide](getting-started) -- Basic provider setup.
