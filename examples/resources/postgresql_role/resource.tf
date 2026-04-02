# Minimal: only "name" is required
resource "postgresql_role" "basic" {
  name = "basic_user"
}

# Role with login and password
resource "postgresql_role" "app" {
  name     = "app_user"   # Required
  login    = true          # Optional, default: false
  password = "changeme"    # Optional
}

# Role with privileges
resource "postgresql_role" "admin" {
  name             = "db_admin"       # Required
  login            = true              # Optional
  password         = "supersecret"     # Optional
  create_database  = true              # Optional, default: false
  create_role      = true              # Optional, default: false
  connection_limit = 10                # Optional, default: -1 (unlimited)
  valid_until      = "2025-12-31T23:59:59Z" # Optional
}

# Group role (no login) used to manage permissions
resource "postgresql_role" "readonly" {
  name  = "readonly"
  login = false
}

# Role inheriting permissions from a group
resource "postgresql_role" "developer" {
  name     = "developer"
  login    = true
  password = "devpass"
  roles    = [postgresql_role.readonly.name] # Optional: list of group memberships
}

# Replication role for streaming replication
resource "postgresql_role" "replicator" {
  name        = "replicator"
  login       = true
  password    = "replpass"
  replication = true # Optional, default: false
}

# Service account with limited connections
resource "postgresql_role" "service" {
  name             = "api_service"
  login            = true
  password         = "svcpass"
  connection_limit = 5
}

# Role inheriting from multiple groups
resource "postgresql_role" "writers" {
  name  = "writers"
  login = false
}

resource "postgresql_role" "full_access" {
  name     = "full_access_user"
  login    = true
  password = "fullpass"
  roles    = [
    postgresql_role.readonly.name,
    postgresql_role.writers.name,
  ]
}
