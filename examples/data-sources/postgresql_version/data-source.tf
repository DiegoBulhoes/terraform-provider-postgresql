# Get PostgreSQL server version
data "postgresql_version" "current" {}

output "pg_version" {
  value = data.postgresql_version.current.version
}

output "pg_major" {
  value = data.postgresql_version.current.major
}
