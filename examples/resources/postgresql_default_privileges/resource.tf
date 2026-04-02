# Default table privileges in a schema
resource "postgresql_default_privileges" "tables" {
  owner       = "app_owner"
  role        = "app_reader"
  database    = "my_application"
  schema      = "public"
  object_type = "table"
  privileges  = ["SELECT"]
}

# Default sequence privileges database-wide
resource "postgresql_default_privileges" "sequences" {
  owner       = "app_owner"
  role        = "app_user"
  database    = "my_application"
  object_type = "sequence"
  privileges  = ["USAGE", "SELECT"]
}

# Default function privileges
resource "postgresql_default_privileges" "functions" {
  owner       = "app_owner"
  role        = "app_user"
  database    = "my_application"
  schema      = "app_schema"
  object_type = "function"
  privileges  = ["EXECUTE"]
}
