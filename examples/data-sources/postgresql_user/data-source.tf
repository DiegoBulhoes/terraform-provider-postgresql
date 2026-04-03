# Look up an existing PostgreSQL user
data "postgresql_user" "app" {
  name = "app_user"
}

output "user_oid" {
  value = data.postgresql_user.app.oid
}

output "user_roles" {
  value = data.postgresql_user.app.roles
}
