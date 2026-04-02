# Look up database properties
data "postgresql_database" "main" {
  name = "postgres"
}

output "db_owner" {
  value = data.postgresql_database.main.owner
}

output "db_encoding" {
  value = data.postgresql_database.main.encoding
}

# Use database info to create a schema in an existing database
data "postgresql_database" "app" {
  name = "my_application"
}

resource "postgresql_schema" "api" {
  name     = "api"
  database = data.postgresql_database.app.name
  owner    = data.postgresql_database.app.owner
}

# Check database configuration
data "postgresql_database" "legacy" {
  name = "legacy_app"
}

output "legacy_collation" {
  value = data.postgresql_database.legacy.lc_collate
}

output "legacy_is_template" {
  value = data.postgresql_database.legacy.is_template
}

output "legacy_connection_limit" {
  value = data.postgresql_database.legacy.connection_limit
}
