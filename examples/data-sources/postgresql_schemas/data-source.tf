# List all user schemas
data "postgresql_schemas" "user_schemas" {
  include_system_schemas = false
}

output "schema_names" {
  value = [for s in data.postgresql_schemas.user_schemas.schemas : s.name]
}
