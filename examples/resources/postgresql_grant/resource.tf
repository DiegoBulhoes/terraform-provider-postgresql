# Grant database privileges
resource "postgresql_grant" "db_connect" {
  role        = "app_user"
  database    = "my_application"
  object_type = "database"
  privileges  = ["CONNECT", "CREATE"]
}

# Grant schema privileges
resource "postgresql_grant" "schema_usage" {
  role        = "app_user"
  schema      = "app_schema"
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
  objects           = ["users", "orders"]
  privileges        = ["SELECT", "INSERT"]
  with_grant_option = true
}
