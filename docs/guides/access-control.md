---
page_title: "Access Control with Roles, Grants, and Default Privileges"
subcategory: "Guides"
description: |-
  A complete guide to managing PostgreSQL access control using roles, grants, and default privileges with Terraform.
---

# Access Control with Roles, Grants, and Default Privileges

This guide demonstrates a realistic access control setup for a PostgreSQL application database. You will create an owner role, a read-only role, and an application role, then grant appropriate privileges and configure default privileges so future objects inherit the correct permissions.

## Overview

A common PostgreSQL access pattern uses three types of roles:

1. **Owner role** -- Owns the database and schemas. Creates tables and other objects. Used during migrations.
2. **Read-only role** -- Can read all data but cannot modify anything. Used for reporting, analytics, and debugging.
3. **Application role** -- Can read and write data but cannot alter schema structure. Used by the running application.

## Step 1: Create the Roles

```terraform
# Owner role: runs migrations, creates tables
resource "postgresql_role" "owner" {
  name            = "app_owner"
  login           = true
  password        = var.owner_password
  create_database = true
}

# Read-only group role (no login -- individual users inherit from it)
resource "postgresql_role" "readonly" {
  name  = "app_readonly"
  login = false
}

# Application role: used by the running service
resource "postgresql_role" "app" {
  name     = "app_service"
  login    = true
  password = var.app_password
}

# A human user who inherits the readonly role
resource "postgresql_role" "analyst" {
  name     = "analyst"
  login    = true
  password = var.analyst_password
  roles    = [postgresql_role.readonly.name]
}
```

## Step 2: Create the Database and Schemas

```terraform
resource "postgresql_database" "app" {
  name     = "myapp"
  owner    = postgresql_role.owner.name
  template = "template0"
  encoding = "UTF8"
}

resource "postgresql_schema" "public_schema" {
  name     = "public"
  database = postgresql_database.app.name
  owner    = postgresql_role.owner.name
}

resource "postgresql_schema" "api" {
  name     = "api"
  database = postgresql_database.app.name
  owner    = postgresql_role.owner.name
}

resource "postgresql_schema" "internal" {
  name     = "internal"
  database = postgresql_database.app.name
  owner    = postgresql_role.owner.name
}
```

## Step 3: Grant Database-Level Privileges

All roles need the ability to connect to the database:

```terraform
resource "postgresql_grant" "readonly_connect" {
  role        = postgresql_role.readonly.name
  database    = postgresql_database.app.name
  object_type = "database"
  privileges  = ["CONNECT"]
}

resource "postgresql_grant" "app_connect" {
  role        = postgresql_role.app.name
  database    = postgresql_database.app.name
  object_type = "database"
  privileges  = ["CONNECT"]
}
```

## Step 4: Grant Schema-Level Privileges

```terraform
# Read-only: USAGE on all schemas
resource "postgresql_grant" "readonly_usage_public" {
  role        = postgresql_role.readonly.name
  database    = postgresql_database.app.name
  schema      = "public"
  object_type = "schema"
  privileges  = ["USAGE"]
}

resource "postgresql_grant" "readonly_usage_api" {
  role        = postgresql_role.readonly.name
  database    = postgresql_database.app.name
  schema      = "api"
  object_type = "schema"
  privileges  = ["USAGE"]
}

resource "postgresql_grant" "readonly_usage_internal" {
  role        = postgresql_role.readonly.name
  database    = postgresql_database.app.name
  schema      = "internal"
  object_type = "schema"
  privileges  = ["USAGE"]
}

# Application: USAGE on public and api schemas only
resource "postgresql_grant" "app_usage_public" {
  role        = postgresql_role.app.name
  database    = postgresql_database.app.name
  schema      = "public"
  object_type = "schema"
  privileges  = ["USAGE"]
}

resource "postgresql_grant" "app_usage_api" {
  role        = postgresql_role.app.name
  database    = postgresql_database.app.name
  schema      = "api"
  object_type = "schema"
  privileges  = ["USAGE"]
}
```

## Step 5: Grant Object-Level Privileges

Grant privileges on all existing tables, sequences, and functions:

```terraform
# Read-only: SELECT on all tables in all schemas
resource "postgresql_grant" "readonly_tables_public" {
  role        = postgresql_role.readonly.name
  database    = postgresql_database.app.name
  schema      = "public"
  object_type = "table"
  privileges  = ["SELECT"]
}

resource "postgresql_grant" "readonly_tables_api" {
  role        = postgresql_role.readonly.name
  database    = postgresql_database.app.name
  schema      = "api"
  object_type = "table"
  privileges  = ["SELECT"]
}

resource "postgresql_grant" "readonly_tables_internal" {
  role        = postgresql_role.readonly.name
  database    = postgresql_database.app.name
  schema      = "internal"
  object_type = "table"
  privileges  = ["SELECT"]
}

# Application: full CRUD on tables in public and api schemas
resource "postgresql_grant" "app_tables_public" {
  role        = postgresql_role.app.name
  database    = postgresql_database.app.name
  schema      = "public"
  object_type = "table"
  privileges  = ["SELECT", "INSERT", "UPDATE", "DELETE"]
}

resource "postgresql_grant" "app_tables_api" {
  role        = postgresql_role.app.name
  database    = postgresql_database.app.name
  schema      = "api"
  object_type = "table"
  privileges  = ["SELECT", "INSERT", "UPDATE", "DELETE"]
}

# Application: sequence usage (needed for serial/identity columns)
resource "postgresql_grant" "app_sequences_public" {
  role        = postgresql_role.app.name
  database    = postgresql_database.app.name
  schema      = "public"
  object_type = "sequence"
  privileges  = ["USAGE", "SELECT"]
}

resource "postgresql_grant" "app_sequences_api" {
  role        = postgresql_role.app.name
  database    = postgresql_database.app.name
  schema      = "api"
  object_type = "sequence"
  privileges  = ["USAGE", "SELECT"]
}
```

## Step 6: Set Up Default Privileges

Default privileges ensure that objects created in the future by the owner role automatically grant the correct permissions. Without this, new tables created by `app_owner` would not be accessible to other roles.

```terraform
# ---- Read-only defaults ----

# Future tables: SELECT
resource "postgresql_default_privileges" "readonly_tables_public" {
  owner       = postgresql_role.owner.name
  role        = postgresql_role.readonly.name
  database    = postgresql_database.app.name
  schema      = "public"
  object_type = "table"
  privileges  = ["SELECT"]
}

resource "postgresql_default_privileges" "readonly_tables_api" {
  owner       = postgresql_role.owner.name
  role        = postgresql_role.readonly.name
  database    = postgresql_database.app.name
  schema      = "api"
  object_type = "table"
  privileges  = ["SELECT"]
}

resource "postgresql_default_privileges" "readonly_tables_internal" {
  owner       = postgresql_role.owner.name
  role        = postgresql_role.readonly.name
  database    = postgresql_database.app.name
  schema      = "internal"
  object_type = "table"
  privileges  = ["SELECT"]
}

# ---- Application defaults ----

# Future tables: full CRUD
resource "postgresql_default_privileges" "app_tables_public" {
  owner       = postgresql_role.owner.name
  role        = postgresql_role.app.name
  database    = postgresql_database.app.name
  schema      = "public"
  object_type = "table"
  privileges  = ["SELECT", "INSERT", "UPDATE", "DELETE"]
}

resource "postgresql_default_privileges" "app_tables_api" {
  owner       = postgresql_role.owner.name
  role        = postgresql_role.app.name
  database    = postgresql_database.app.name
  schema      = "api"
  object_type = "table"
  privileges  = ["SELECT", "INSERT", "UPDATE", "DELETE"]
}

# Future sequences: USAGE and SELECT
resource "postgresql_default_privileges" "app_sequences_public" {
  owner       = postgresql_role.owner.name
  role        = postgresql_role.app.name
  database    = postgresql_database.app.name
  schema      = "public"
  object_type = "sequence"
  privileges  = ["USAGE", "SELECT"]
}

resource "postgresql_default_privileges" "app_sequences_api" {
  owner       = postgresql_role.owner.name
  role        = postgresql_role.app.name
  database    = postgresql_database.app.name
  schema      = "api"
  object_type = "sequence"
  privileges  = ["USAGE", "SELECT"]
}

# Future functions: EXECUTE
resource "postgresql_default_privileges" "app_functions_public" {
  owner       = postgresql_role.owner.name
  role        = postgresql_role.app.name
  database    = postgresql_database.app.name
  schema      = "public"
  object_type = "function"
  privileges  = ["EXECUTE"]
}
```

## Variables

Define the sensitive variables used above:

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
```

## Summary

This configuration implements a layered access control model:

| Role | Database | Schema | Tables | Sequences | Functions |
|------|----------|--------|--------|-----------|-----------|
| `app_owner` | Owner | Owner | Owner | Owner | Owner |
| `app_readonly` | CONNECT | USAGE (all) | SELECT (all) | -- | -- |
| `app_service` | CONNECT | USAGE (public, api) | CRUD (public, api) | USAGE, SELECT | EXECUTE |
| `analyst` | Inherits from `app_readonly` | Inherits | Inherits | Inherits | Inherits |

Default privileges ensure this model is maintained as new objects are created by the owner role.

## Next Steps

- [Getting Started Guide](getting-started) -- Basic provider setup.
- [Importing Resources Guide](importing-resources) -- Import existing grants and privileges into Terraform state.
