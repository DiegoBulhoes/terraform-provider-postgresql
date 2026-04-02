# List all user schemas (excluding system schemas)
data "postgresql_schemas" "user_schemas" {
  include_system_schemas = false
}

output "schema_names" {
  value = [for s in data.postgresql_schemas.user_schemas.schemas : s.name]
}

# Filter schemas by prefix
data "postgresql_schemas" "app_schemas" {
  like_pattern = "app_%"
}

output "app_schema_details" {
  value = [for s in data.postgresql_schemas.app_schemas.schemas : {
    name  = s.name
    owner = s.owner
  }]
}

# Exclude test schemas
data "postgresql_schemas" "production" {
  not_like_pattern       = "%_test"
  include_system_schemas = false
}

# List schemas from a specific database
data "postgresql_schemas" "other_db" {
  database               = "analytics"
  include_system_schemas = false
}

# Grant SELECT on all tables in every app schema
data "postgresql_schemas" "grantable" {
  like_pattern           = "app_%"
  include_system_schemas = false
}

resource "postgresql_grant" "reader_per_schema" {
  for_each = {
    for s in data.postgresql_schemas.grantable.schemas : s.name => s
  }

  role        = "readonly_user"
  schema      = each.value.name
  object_type = "table"
  privileges  = ["SELECT"]
}
