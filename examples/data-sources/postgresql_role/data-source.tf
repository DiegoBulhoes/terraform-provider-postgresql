data "postgresql_role" "admin" {
  name = "postgres"
}

output "admin_oid" {
  value = data.postgresql_role.admin.oid
}

output "is_superuser" {
  value = data.postgresql_role.admin.superuser
}
