# Tables app_owner creates later in the app schema are readable by app_reader
resource "postgresql_default_privileges" "reader_tables" {
  owner       = "app_owner"      # Required: the role that creates the objects
  role        = "app_reader"     # Required: the role that receives the privileges
  database    = "my_application" # Optional: defaults to the provider's database
  schema      = "app"            # Optional: without it, every schema
  object_type = "table"          # Required: table, sequence, function, type, or schema
  privileges  = ["SELECT"]       # Required
}

# Sequences, needed for INSERT with serial or identity columns
resource "postgresql_default_privileges" "writer_sequences" {
  owner       = "app_owner"
  role        = "app_writer"
  database    = "my_application"
  schema      = "app"
  object_type = "sequence"
  privileges  = ["USAGE", "SELECT"]
}

# Functions app_owner creates in any schema, with delegation
resource "postgresql_default_privileges" "functions" {
  owner             = "app_owner"
  role              = "app_admin"
  database          = "my_application"
  object_type       = "function"
  privileges        = ["EXECUTE"]
  with_grant_option = true # Optional, default: false
}

# Schemas app_owner creates later: object_type schema takes no schema
resource "postgresql_default_privileges" "schemas" {
  owner       = "app_owner"
  role        = "app_reader"
  database    = "my_application"
  object_type = "schema"
  privileges  = ["USAGE"]
}
