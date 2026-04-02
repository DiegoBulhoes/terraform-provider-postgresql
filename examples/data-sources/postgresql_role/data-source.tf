# Look up an existing role
data "postgresql_role" "admin" {
  name = "postgres"
}

output "admin_oid" {
  value = data.postgresql_role.admin.oid
}

output "is_superuser" {
  value = data.postgresql_role.admin.superuser
}

# Use a role data source to conditionally create resources
data "postgresql_role" "app" {
  name = "app_user"
}

resource "postgresql_grant" "app_tables" {
  role        = data.postgresql_role.app.name
  database    = "my_application"
  schema      = "public"
  object_type = "table"
  privileges  = data.postgresql_role.app.superuser ? [] : ["SELECT", "INSERT"]
}

# Check role memberships
data "postgresql_role" "developer" {
  name = "developer"
}

output "developer_memberships" {
  value = data.postgresql_role.developer.roles
}

output "developer_can_login" {
  value = data.postgresql_role.developer.login
}

output "developer_can_create_db" {
  value = data.postgresql_role.developer.create_database
}
