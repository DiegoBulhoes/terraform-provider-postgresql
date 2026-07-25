---
page_title: "Resource postgresql_grant - terraform-provider-postgresql"
subcategory: "Roles & Permissions"
description: |-
  Manages PostgreSQL GRANT privileges on database objects.
---

# Resource (postgresql_grant)

Manages PostgreSQL GRANT privileges on database objects such as databases, schemas, tables, sequences, and functions.

## Valid Privileges

Privilege names are checked against an allowlist before any SQL is built. These keywords are accepted. Case does not matter, and they are upper-cased for you:

| Category | Keywords |
|----------|----------|
| Wildcards | `ALL`, `ALL PRIVILEGES` |
| Data access | `SELECT`, `INSERT`, `UPDATE`, `DELETE`, `TRUNCATE`, `REFERENCES`, `TRIGGER` |
| Containers | `USAGE`, `CREATE`, `CONNECT`, `TEMPORARY` (alias `TEMP`) |
| Functions | `EXECUTE` |
| Maintenance | `MAINTAIN` |
| Configuration | `SET`, `ALTER SYSTEM` |

Anything else is rejected with an `Invalid privileges` error at plan or apply time. That covers typos like `SELCT` and injection attempts like `"SELECT; DROP"`.

## Drift Detection

The provider compares Terraform state against the real grants in PostgreSQL for:

- `object_type = "database"` — always
- `object_type = "schema"` — always
- `object_type = "table"`, `"sequence"`, `"function"` — **only when `objects = [...]` is specified**

~> **Grants on `ALL` objects skip drift detection.** When `object_type` is `table`, `sequence`, or `function` and `objects` is empty or unset, the provider runs `GRANT ... ON ALL ... IN SCHEMA` and does **not** check privileges per object on refresh. Objects created outside Terraform will not show up as missing. To get drift detection, name the objects: `objects = ["foo", "bar"]`.

## Example Usage

```terraform
# Grant database-level privileges
# Required: role, object_type, privileges
resource "postgresql_grant" "db_connect" {
  role        = "app_user"                           # Required
  database    = "my_application"                     # Optional
  object_type = "database"                           # Required: database, schema, table, sequence, or function
  privileges  = ["CONNECT", "CREATE"]                # Required
}

# Grant schema usage
resource "postgresql_grant" "schema_usage" {
  role        = "app_user"
  schema      = "app_schema"             # Optional
  object_type = "schema"
  privileges  = ["USAGE", "CREATE"]
}

# Grant on all tables in a schema
resource "postgresql_grant" "all_tables" {
  role        = "app_user"
  database    = "my_application"
  schema      = "public"
  object_type = "table"
  privileges  = ["SELECT", "INSERT", "UPDATE", "DELETE"]
}

# Grant on specific tables with grant option
resource "postgresql_grant" "specific_tables" {
  role              = "app_user"
  database          = "my_application"
  schema            = "public"
  object_type       = "table"
  objects           = ["users", "orders"]  # Optional: if empty, grants on ALL objects
  privileges        = ["SELECT", "INSERT"]
  with_grant_option = true                 # Optional, default: false
}

# Read-only access to all tables
resource "postgresql_grant" "readonly_tables" {
  role        = "readonly"
  database    = "my_application"
  schema      = "public"
  object_type = "table"
  privileges  = ["SELECT"]
}

# Grant sequence usage (needed for INSERT with serial/identity columns)
resource "postgresql_grant" "sequences" {
  role        = "app_user"
  database    = "my_application"
  schema      = "public"
  object_type = "sequence"
  privileges  = ["USAGE", "SELECT"]
}

# Grant EXECUTE on all functions in a schema
resource "postgresql_grant" "functions" {
  role        = "app_user"
  database    = "my_application"
  schema      = "public"
  object_type = "function"
  privileges  = ["EXECUTE"]
}

# Full access setup: database + schema + tables + sequences
resource "postgresql_grant" "full_db" {
  role        = "power_user"
  database    = "my_application"
  object_type = "database"
  privileges  = ["CONNECT", "CREATE", "TEMPORARY"]
}

resource "postgresql_grant" "full_schema" {
  role        = "power_user"
  schema      = "public"
  object_type = "schema"
  privileges  = ["USAGE", "CREATE"]
}

resource "postgresql_grant" "full_tables" {
  role        = "power_user"
  database    = "my_application"
  schema      = "public"
  object_type = "table"
  privileges  = ["SELECT", "INSERT", "UPDATE", "DELETE", "TRUNCATE", "REFERENCES", "TRIGGER"]
}

resource "postgresql_grant" "full_sequences" {
  role        = "power_user"
  database    = "my_application"
  schema      = "public"
  object_type = "sequence"
  privileges  = ["USAGE", "SELECT", "UPDATE"]
}
```

### Granting with Delegation (`with_grant_option`)

Set `with_grant_option = true` so the grantee can pass the same privileges on to others. This helps platform-team roles that manage access without being superuser:

```terraform
resource "postgresql_grant" "platform_admin" {
  role              = "platform_admin"
  database          = "myapp"
  object_type       = "database"
  privileges        = ["CONNECT", "CREATE", "TEMPORARY"]
  with_grant_option = true
}
```

<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `object_type` (String) The object type to grant privileges on: database, schema, table, sequence, or function.
- `privileges` (Set of String) The set of privileges to grant (e.g. SELECT, INSERT, USAGE, CREATE).
- `role` (String) The role to which privileges are granted.

### Optional

- `database` (String) The database on which to grant privileges.

~> **Note:** For database-level grants, the provider uses its configured connection. Ensure the provider is configured to connect to the correct database.
- `objects` (List of String) Specific object names to grant on. If empty, grants on all objects of the given type in the schema.
- `schema` (String) The schema on which to grant privileges.
- `timeouts` (Block, Optional) (see [below for nested schema](#nestedblock--timeouts))
- `with_grant_option` (Boolean) Whether the grantee can grant the same privileges to others.

### Read-Only

- `id` (String) Composite identifier: {role}_{object_type}_{database}_{schema}.

<a id="nestedblock--timeouts"></a>
### Nested Schema for `timeouts`

Optional:

- `create` (String) A string that can be [parsed as a duration](https://pkg.go.dev/time#ParseDuration) consisting of numbers and unit suffixes, such as "30s" or "2h45m". Valid time units are "s" (seconds), "m" (minutes), "h" (hours).
- `delete` (String) A string that can be [parsed as a duration](https://pkg.go.dev/time#ParseDuration) consisting of numbers and unit suffixes, such as "30s" or "2h45m". Valid time units are "s" (seconds), "m" (minutes), "h" (hours). Setting a timeout for a Delete operation is only applicable if changes are saved into state before the destroy operation occurs.
- `update` (String) A string that can be [parsed as a duration](https://pkg.go.dev/time#ParseDuration) consisting of numbers and unit suffixes, such as "30s" or "2h45m". Valid time units are "s" (seconds), "m" (minutes), "h" (hours).

## Import

Import a grant as `role/object_type/database/schema`, or as `role/object_type/database` for database-level grants:

```shell
# Database-level grant: role/object_type/database (3 parts, no schema)
terraform import postgresql_grant.db_connect "app_user/database/my_application"

# Schema-level grant: role/object_type/database/schema (4 parts)
terraform import postgresql_grant.schema_usage "app_user/schema/my_application/app_schema"

# Object-level grant (tables, sequences, functions): same 4-part format
terraform import postgresql_grant.tables "app_user/table/my_application/public"
```
