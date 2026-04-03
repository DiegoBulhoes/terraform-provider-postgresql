# List all tables in the public schema
data "postgresql_tables" "public" {
  schema = "public"
}

output "public_tables" {
  value = data.postgresql_tables.public.tables
}

# List only base tables matching a pattern
data "postgresql_tables" "app_tables" {
  schema       = "public"
  like_pattern = "app_%"
  table_type   = "BASE TABLE"
}

# List views in a schema
data "postgresql_tables" "views" {
  schema     = "public"
  table_type = "VIEW"
}
