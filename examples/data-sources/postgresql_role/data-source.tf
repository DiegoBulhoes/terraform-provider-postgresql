# Look up the built-in superuser role
data "postgresql_role" "admin" {
  name = "postgres"
}

output "admin_oid" {
  value = data.postgresql_role.admin.oid
}

output "is_superuser" {
  value = data.postgresql_role.admin.superuser
}

# Look up a permission group role
data "postgresql_role" "readonly" {
  name = "readonly"
}

output "readonly_oid" {
  value = data.postgresql_role.readonly.oid
}

output "readonly_can_create_db" {
  value = data.postgresql_role.readonly.create_database
}
