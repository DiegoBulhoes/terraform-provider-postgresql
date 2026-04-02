data "postgresql_database" "main" {
  name = "postgres"
}

output "db_owner" {
  value = data.postgresql_database.main.owner
}

output "db_encoding" {
  value = data.postgresql_database.main.encoding
}
