# Minimal user with login
resource "postgresql_user" "basic" {
  name     = "app_user"
  password = "changeme"
}

# User with role memberships
resource "postgresql_role" "readonly" {
  name = "readonly"

  privilege {
    privileges  = ["SELECT"]
    object_type = "table"
    schema      = "public"
  }
}

resource "postgresql_user" "developer" {
  name     = "developer"
  password = "devpass"
  roles    = [postgresql_role.readonly.name]
}

# User with multiple role memberships
resource "postgresql_role" "writers" {
  name = "writers"

  privilege {
    privileges  = ["SELECT", "INSERT", "UPDATE", "DELETE"]
    object_type = "table"
    schema      = "public"
  }
}

resource "postgresql_user" "full_access" {
  name     = "full_access_user"
  password = "fullpass"
  roles    = [
    postgresql_role.readonly.name,
    postgresql_role.writers.name,
  ]
}

# Admin user with elevated privileges
resource "postgresql_user" "admin" {
  name             = "db_admin"
  password         = "supersecret"
  create_database  = true
  create_role      = true
  connection_limit = 10
  valid_until      = "2030-12-31T23:59:59Z"
}

# Replication user
resource "postgresql_user" "replicator" {
  name        = "replicator"
  password    = "replpass"
  replication = true
}

# Service account with limited connections
resource "postgresql_user" "service" {
  name             = "api_service"
  password         = "svcpass"
  connection_limit = 5
}
