# Default table privileges: read-only for future tables
# Required: owner, role, database, object_type, privileges
resource "postgresql_default_privileges" "readonly_tables" {
  owner       = "app_owner"        # Required: role that creates objects
  role        = "readonly"         # Required: role that receives privileges
  database    = "my_application"   # Required
  schema      = "public"           # Optional: if omitted, applies database-wide
  object_type = "table"            # Required: table, sequence, function, or type
  privileges  = ["SELECT"]         # Required
}

# Default table privileges: full CRUD for the app user
resource "postgresql_default_privileges" "app_tables" {
  owner       = "app_owner"
  role        = "app_user"
  database    = "my_application"
  schema      = "public"
  object_type = "table"
  privileges  = ["SELECT", "INSERT", "UPDATE", "DELETE"]
}

# Default sequence privileges (needed for serial/identity columns)
resource "postgresql_default_privileges" "sequences" {
  owner       = "app_owner"
  role        = "app_user"
  database    = "my_application"
  schema      = "public"
  object_type = "sequence"
  privileges  = ["USAGE", "SELECT"]
}

# Default function privileges
resource "postgresql_default_privileges" "functions" {
  owner       = "app_owner"
  role        = "app_user"
  database    = "my_application"
  schema      = "public"
  object_type = "function"
  privileges  = ["EXECUTE"]
}

# Default type privileges
resource "postgresql_default_privileges" "types" {
  owner       = "app_owner"
  role        = "app_user"
  database    = "my_application"
  schema      = "public"
  object_type = "type"
  privileges  = ["USAGE"]
}

# Database-wide defaults (no schema = applies to all schemas)
resource "postgresql_default_privileges" "global_tables" {
  owner       = "app_owner"
  role        = "readonly"
  database    = "my_application"
  object_type = "table"
  privileges  = ["SELECT"]
}

# Complete setup for a schema: tables + sequences + functions
resource "postgresql_default_privileges" "api_tables" {
  owner       = "app_owner"
  role        = "app_user"
  database    = "my_application"
  schema      = "api"
  object_type = "table"
  privileges  = ["SELECT", "INSERT", "UPDATE", "DELETE"]
}

resource "postgresql_default_privileges" "api_sequences" {
  owner       = "app_owner"
  role        = "app_user"
  database    = "my_application"
  schema      = "api"
  object_type = "sequence"
  privileges  = ["USAGE", "SELECT"]
}

resource "postgresql_default_privileges" "api_functions" {
  owner       = "app_owner"
  role        = "app_user"
  database    = "my_application"
  schema      = "api"
  object_type = "function"
  privileges  = ["EXECUTE"]
}
