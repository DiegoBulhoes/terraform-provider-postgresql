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
