# Minimal: only "name" is required
resource "postgresql_database" "mydb" {
  name = "my_application" # Required (forces new resource if changed)
}

# Database with dedicated owner
resource "postgresql_role" "owner" {
  name            = "app_owner"
  login           = true
  create_database = true
}

resource "postgresql_database" "configured" {
  name              = "my_app_db"                # Required
  owner             = postgresql_role.owner.name  # Optional
  template          = "template0"                 # Optional, default: template0
  encoding          = "UTF8"                      # Optional, default: UTF8
  lc_collate        = "en_US.UTF-8"              # Optional
  lc_ctype          = "en_US.UTF-8"              # Optional
  tablespace_name   = "pg_default"               # Optional, default: pg_default
  connection_limit  = 100                         # Optional, default: -1 (unlimited)
  allow_connections = true                        # Optional, default: true
  is_template       = false                       # Optional, default: false
}

# Template database that can be cloned
resource "postgresql_database" "template" {
  name              = "app_template"
  owner             = postgresql_role.owner.name
  is_template       = true
  allow_connections = false
}

# Database for testing with restricted connections
resource "postgresql_database" "test" {
  name             = "test_db"
  owner            = postgresql_role.owner.name
  connection_limit = 10
}

# Analytics database
resource "postgresql_database" "analytics" {
  name     = "analytics"
  owner    = postgresql_role.owner.name
  encoding = "UTF8"
}
