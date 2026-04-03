# List all roles with login privilege
data "postgresql_roles" "login_roles" {
  login_only = true
}

output "login_roles" {
  value = data.postgresql_roles.login_roles.roles
}

# List roles matching a pattern
data "postgresql_roles" "app_roles" {
  like_pattern = "app_%"
}

# List roles excluding system roles
data "postgresql_roles" "custom_roles" {
  not_like_pattern = "pg_%"
}
