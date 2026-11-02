---
page_title: "Resource postgresql_grant - postgresql"
subcategory: "Roles & Permissions"
description: |-
  Manages PostgreSQL GRANT privileges on database objects.
---

# Resource (postgresql_grant)

Manages PostgreSQL GRANT privileges on databases, schemas, tables, sequences, and functions.

## Valid privileges

The provider checks privilege names against an allowlist before it builds any SQL. It accepts these keywords in any case and upper-cases them:

| Category | Keywords |
|----------|----------|
| Wildcards | `ALL`, `ALL PRIVILEGES` |
| Data access | `SELECT`, `INSERT`, `UPDATE`, `DELETE`, `TRUNCATE`, `REFERENCES`, `TRIGGER` |
| Containers | `USAGE`, `CREATE`, `CONNECT`, `TEMPORARY` (alias `TEMP`) |
| Functions | `EXECUTE` |
| Maintenance | `MAINTAIN` |
| Configuration | `SET`, `ALTER SYSTEM` |

Any other name fails with an `Invalid privileges` error at plan or apply time, whether a typo like `SELCT` or an injection attempt like `"SELECT; DROP"`.

## Drift detection

On refresh, the provider reads the privileges the role holds on every object of the grant: each name in `objects`, or, when `objects` is empty, every object of the type in the schema. A privilege missing on any of them, such as on a table created after the apply, shows up as a change, and the next apply grants it again.

The comparison follows PostgreSQL's spelling, so `select`, `ALL` and `TEMP` do not cause a diff. `ALL` stands for every privilege the server knows for the object type.

## Functions

Name a function alone to cover all its overloads, or with its argument types to cover one: `objects = ["refresh_cache", "lookup(integer)"]`. The provider resolves each name on the server, so a function that does not exist fails the apply.

## Database

Schema, table, sequence and function grants run in the database set in `database`; without it, in the provider's database. A database grant always runs on the provider's connection.

~> **Destroying a grant on `ALL` objects** revokes the role's privileges on every object of the type in the schema, including privileges another `postgresql_grant` resource manages on a single object.

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

### Granting with delegation (`with_grant_option`)

Set `with_grant_option = true` so the grantee can pass the same privileges on to others. Platform team roles use this to manage access without being superuser:

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
- `privileges` (Set of String) The set of privileges to grant (e.g. SELECT, INSERT, USAGE, CREATE). Case-insensitive; ALL stands for every privilege of the object type.
- `role` (String) The role to which privileges are granted.

### Optional

- `database` (String) The database of the grant. Required when object_type is database. Schema, table, sequence and function grants run in this database, which defaults to the provider's database.
- `objects` (List of String) Names of the tables, sequences or functions to grant on. A function named alone covers all its overloads; with argument types, as in f(integer), it covers one. If empty, the grant covers all objects of the type in the schema, and destroying it revokes the privileges on all of them.
- `schema` (String) The schema on which to grant privileges.
- `timeouts` (Block, Optional) (see [below for nested schema](#nestedblock--timeouts))
- `with_grant_option` (Boolean) Whether the grantee can grant the same privileges to others.

### Read-Only

- `id` (String) The import ID: role/object_type/database[/schema[/object1,object2]].

<a id="nestedblock--timeouts"></a>
### Nested Schema for `timeouts`

Optional:

- `create` (String) A string that can be [parsed as a duration](https://pkg.go.dev/time#ParseDuration) consisting of numbers and unit suffixes, such as "30s" or "2h45m". Valid time units are "s" (seconds), "m" (minutes), "h" (hours).
- `delete` (String) A string that can be [parsed as a duration](https://pkg.go.dev/time#ParseDuration) consisting of numbers and unit suffixes, such as "30s" or "2h45m". Valid time units are "s" (seconds), "m" (minutes), "h" (hours). Setting a timeout for a Delete operation is only applicable if changes are saved into state before the destroy operation occurs.
- `read` (String) A string that can be [parsed as a duration](https://pkg.go.dev/time#ParseDuration) consisting of numbers and unit suffixes, such as "30s" or "2h45m". Valid time units are "s" (seconds), "m" (minutes), "h" (hours). Read operations occur during any refresh or planning operation when refresh is enabled.
- `update` (String) A string that can be [parsed as a duration](https://pkg.go.dev/time#ParseDuration) consisting of numbers and unit suffixes, such as "30s" or "2h45m". Valid time units are "s" (seconds), "m" (minutes), "h" (hours).

## Import

Import a grant as `role/object_type/database/schema`, or as `role/object_type/database` for database-level grants. For a grant on named objects, add the object names, comma-separated. The privileges are read from the server:

```shell
# Database-level grant: role/object_type/database (3 parts, no schema)
terraform import postgresql_grant.db_connect "app_user/database/my_application"

# Schema-level grant: role/object_type/database/schema (4 parts)
terraform import postgresql_grant.schema_usage "app_user/schema/my_application/app_schema"

# Grant on all objects of a type: same 4-part format
terraform import postgresql_grant.tables "app_user/table/my_application/public"

# Grant on named objects: object names, comma-separated, as a fifth part
terraform import postgresql_grant.orders "app_user/table/my_application/public/orders,order_items"
```
