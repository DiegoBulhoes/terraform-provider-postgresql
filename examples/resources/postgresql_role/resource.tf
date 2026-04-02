# Basic role with login
resource "postgresql_role" "app" {
  name     = "app_user"
  login    = true
  password = "changeme"
}

# Role with privileges
resource "postgresql_role" "admin" {
  name             = "db_admin"
  login            = true
  password         = "supersecret"
  create_database  = true
  create_role      = true
  connection_limit = 10
  valid_until      = "2025-12-31T23:59:59Z"
}

# Role with memberships
resource "postgresql_role" "readonly" {
  name  = "readonly"
  login = false
}

resource "postgresql_role" "developer" {
  name     = "developer"
  login    = true
  password = "devpass"
  roles    = [postgresql_role.readonly.name]
}
