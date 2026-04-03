---
page_title: "Importing Existing PostgreSQL Resources into Terraform"
subcategory: "Guides"
description: |-
  Step-by-step guide for importing existing PostgreSQL roles, databases, schemas, grants, and default privileges into Terraform state.
---

# Importing Existing PostgreSQL Resources into Terraform

When adopting Terraform for an existing PostgreSQL infrastructure, you need to import your current resources into Terraform state so that Terraform can manage them going forward without recreating them. This guide covers the import process for every resource type supported by the provider.

## Prerequisites

- The provider must be configured and able to connect to your PostgreSQL instance.
- You must write the corresponding Terraform resource configuration **before** running the import command. Terraform import only updates the state -- it does not generate configuration.
- The PostgreSQL user configured in the provider must have sufficient privileges to read the object being imported.

## General Workflow

The import process follows the same pattern for all resource types:

1. **Write the resource block** in your Terraform configuration.
2. **Run `terraform import`** with the resource address and import ID.
3. **Run `terraform plan`** to verify the imported state matches your configuration.
4. **Adjust the configuration** if the plan shows unexpected changes.

## Importing Roles

**Import ID format:** The role name.

```terraform
resource "postgresql_role" "existing_user" {
  name  = "existing_user"
  login = true
}
```

```shell
terraform import postgresql_role.existing_user existing_user
```

After importing, run `terraform plan` and adjust attributes like `login`, `create_database`, `create_role`, `connection_limit`, and others to match the current state of the role.

> **Note:** The `password` attribute cannot be read from PostgreSQL during import. If the role has a password, add the `password` attribute to your configuration. Terraform will detect a difference and update it on the next apply, or you can use `lifecycle { ignore_changes = [password] }` to leave it unmanaged.

```terraform
resource "postgresql_role" "existing_user" {
  name     = "existing_user"
  login    = true
  password = var.existing_user_password

  lifecycle {
    ignore_changes = [password]
  }
}
```

## Importing Databases

**Import ID format:** The database name.

```terraform
resource "postgresql_database" "legacy_db" {
  name     = "legacy_database"
  owner    = "db_owner"
  encoding = "UTF8"
}
```

```shell
terraform import postgresql_database.legacy_db legacy_database
```

After importing, verify that `owner`, `encoding`, `lc_collate`, `lc_ctype`, `template`, and `tablespace_name` match the existing database. These attributes are read during import and will appear in the state.

## Importing Schemas

**Import ID format:** The schema name. If the schema is in a non-default database, use the format `database.schema`.

```terraform
resource "postgresql_schema" "app_schema" {
  name     = "app"
  database = "mydb"
  owner    = "app_owner"
}
```

```shell
terraform import postgresql_schema.app_schema mydb.app
```

For a schema in the provider's default database:

```shell
terraform import postgresql_schema.app_schema app
```

## Importing Grants

**Import ID format:** `role/object_type/database/schema`

The import ID has four parts separated by `/`:

| Part | Description | Example |
|------|-------------|---------|
| `role` | The role that received the grant | `app_user` |
| `object_type` | The object type | `database`, `schema`, `table`, `sequence`, `function` |
| `database` | The database name | `myapp` |
| `schema` | The schema name (use empty string for database-level grants) | `public` |

### Importing a Database-Level Grant

```terraform
resource "postgresql_grant" "app_connect" {
  role        = "app_user"
  database    = "myapp"
  object_type = "database"
  privileges  = ["CONNECT"]
}
```

```shell
terraform import postgresql_grant.app_connect "app_user/database/myapp/"
```

Note the trailing `/` -- the schema part is empty for database-level grants.

### Importing a Schema-Level Grant

```terraform
resource "postgresql_grant" "app_schema_usage" {
  role        = "app_user"
  database    = "myapp"
  schema      = "public"
  object_type = "schema"
  privileges  = ["USAGE"]
}
```

```shell
terraform import postgresql_grant.app_schema_usage "app_user/schema/myapp/public"
```

### Importing a Table-Level Grant

```terraform
resource "postgresql_grant" "app_tables" {
  role        = "app_user"
  database    = "myapp"
  schema      = "public"
  object_type = "table"
  privileges  = ["SELECT", "INSERT", "UPDATE", "DELETE"]
}
```

```shell
terraform import postgresql_grant.app_tables "app_user/table/myapp/public"
```

### Importing a Sequence-Level Grant

```terraform
resource "postgresql_grant" "app_sequences" {
  role        = "app_user"
  database    = "myapp"
  schema      = "public"
  object_type = "sequence"
  privileges  = ["USAGE", "SELECT"]
}
```

```shell
terraform import postgresql_grant.app_sequences "app_user/sequence/myapp/public"
```

### Importing a Function-Level Grant

```terraform
resource "postgresql_grant" "app_functions" {
  role        = "app_user"
  database    = "myapp"
  schema      = "public"
  object_type = "function"
  privileges  = ["EXECUTE"]
}
```

```shell
terraform import postgresql_grant.app_functions "app_user/function/myapp/public"
```

## Importing Default Privileges

**Import ID format:** `owner/role/database/object_type` or `owner/role/database/schema/object_type`

Default privileges have two import formats depending on whether they are scoped to a specific schema.

| Part | Description | Example |
|------|-------------|---------|
| `owner` | The role that creates objects | `app_owner` |
| `role` | The role that receives default privileges | `app_user` |
| `database` | The database name | `myapp` |
| `schema` | (Optional) The schema name | `public` |
| `object_type` | The object type | `table`, `sequence`, `function`, `type` |

### Importing Schema-Scoped Default Privileges

```terraform
resource "postgresql_default_privileges" "app_tables" {
  owner       = "app_owner"
  role        = "app_user"
  database    = "myapp"
  schema      = "public"
  object_type = "table"
  privileges  = ["SELECT", "INSERT", "UPDATE", "DELETE"]
}
```

```shell
terraform import postgresql_default_privileges.app_tables "app_owner/app_user/myapp/public/table"
```

### Importing Database-Wide Default Privileges (No Schema)

```terraform
resource "postgresql_default_privileges" "readonly_tables" {
  owner       = "app_owner"
  role        = "readonly"
  database    = "myapp"
  object_type = "table"
  privileges  = ["SELECT"]
}
```

```shell
terraform import postgresql_default_privileges.readonly_tables "app_owner/readonly/myapp/table"
```

### More Default Privilege Examples

Sequences:

```shell
terraform import postgresql_default_privileges.app_sequences "app_owner/app_user/myapp/public/sequence"
```

Functions:

```shell
terraform import postgresql_default_privileges.app_functions "app_owner/app_user/myapp/public/function"
```

Types:

```shell
terraform import postgresql_default_privileges.app_types "app_owner/app_user/myapp/public/type"
```

## Import ID Quick Reference

| Resource | Import ID Format | Example |
|----------|------------------|---------|
| `postgresql_role` | `name` | `app_user` |
| `postgresql_database` | `name` | `myapp` |
| `postgresql_schema` | `name` or `database.name` | `mydb.public` |
| `postgresql_grant` | `role/object_type/database/schema` | `app_user/table/myapp/public` |
| `postgresql_default_privileges` | `owner/role/database/object_type` | `app_owner/app_user/myapp/table` |
| `postgresql_default_privileges` (schema-scoped) | `owner/role/database/schema/object_type` | `app_owner/app_user/myapp/public/table` |

## Tips and Troubleshooting

### Run `terraform plan` After Every Import

Always run `terraform plan` after importing a resource to verify that your configuration matches the actual state. If the plan shows changes, update your configuration to match reality before applying anything.

### Import Multiple Resources with a Script

For large environments, you can script the import process:

```shell
#!/bin/bash
set -e

# Roles
terraform import postgresql_role.admin admin
terraform import postgresql_role.app_user app_user
terraform import postgresql_role.readonly readonly

# Databases
terraform import postgresql_database.myapp myapp
terraform import postgresql_database.analytics analytics

# Schemas
terraform import postgresql_schema.public "myapp.public"
terraform import postgresql_schema.api "myapp.api"

# Grants
terraform import postgresql_grant.app_connect "app_user/database/myapp/"
terraform import postgresql_grant.app_tables "app_user/table/myapp/public"

# Default privileges
terraform import postgresql_default_privileges.app_tables "admin/app_user/myapp/public/table"

echo "Import complete. Run 'terraform plan' to verify."
```

### Use `terraform state show` to Inspect Imported State

After importing, inspect the state to see all attribute values:

```shell
terraform state show postgresql_role.existing_user
terraform state show postgresql_grant.app_tables
```

This helps you write your configuration to match the imported state exactly.

### Handling Passwords

PostgreSQL does not expose passwords in a readable form. After importing a role, you have two options:

1. **Set the password in your configuration.** Terraform will detect a difference and update the password on the next apply.
2. **Ignore password changes** using a lifecycle block if you manage passwords outside of Terraform.

## Next Steps

- [Getting Started Guide](getting-started) -- Basic provider setup.
- [Access Control Guide](access-control) -- Roles, grants, and default privileges working together.
- [Managed PostgreSQL Guide](managed-postgresql) -- Connect to cloud-managed PostgreSQL services.
