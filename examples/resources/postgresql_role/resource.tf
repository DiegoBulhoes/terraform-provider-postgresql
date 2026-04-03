# Minimal: only "name" is required
resource "postgresql_role" "basic" {
  name = "basic_role"
}

# Role with inline privileges on all tables in a schema
resource "postgresql_role" "readonly" {
  name = "readonly"

  privilege {
    privileges  = ["SELECT"]
    object_type = "table"
    schema      = "public"
  }
}

# Role with multiple privilege blocks
resource "postgresql_role" "readwrite" {
  name = "readwrite"

  privilege {
    privileges  = ["SELECT", "INSERT", "UPDATE", "DELETE"]
    object_type = "table"
    schema      = "public"
  }

  privilege {
    privileges  = ["USAGE", "SELECT"]
    object_type = "sequence"
    schema      = "public"
  }
}

# Role with schema-level privileges
resource "postgresql_role" "schema_admin" {
  name            = "schema_admin"
  create_database = true

  privilege {
    privileges  = ["CREATE", "USAGE"]
    object_type = "schema"
    schema      = "public"
  }
}

# Role with database-level privileges
resource "postgresql_role" "db_connect" {
  name = "db_connect"

  privilege {
    privileges  = ["CONNECT"]
    object_type = "database"
    database    = "myapp"
  }
}

# Role with privileges on specific tables
resource "postgresql_role" "reports" {
  name = "reports"

  privilege {
    privileges  = ["SELECT"]
    object_type = "table"
    schema      = "public"
    objects     = ["orders", "products", "customers"]
  }
}
