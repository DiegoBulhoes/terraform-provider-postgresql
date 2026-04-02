# Minimal: only "name" is required
resource "postgresql_schema" "app" {
  name = "app_schema" # Required
}

# Schema with owner
resource "postgresql_schema" "owned" {
  name  = "api"                       # Required
  owner = postgresql_role.owner.name  # Optional (defaults to current user)
}

# Schema with IF NOT EXISTS (useful for shared environments)
resource "postgresql_schema" "safe" {
  name          = "shared_schema"  # Required
  if_not_exists = true             # Optional, default: false
}

# Schema in a specific database
resource "postgresql_schema" "other_db" {
  name     = "reports"                          # Required
  database = postgresql_database.analytics.name # Optional (defaults to provider database)
  owner    = postgresql_role.owner.name         # Optional
}

# Multiple schemas for a microservices architecture
resource "postgresql_schema" "users" {
  name  = "users"
  owner = postgresql_role.owner.name
}

resource "postgresql_schema" "orders" {
  name  = "orders"
  owner = postgresql_role.owner.name
}

resource "postgresql_schema" "payments" {
  name  = "payments"
  owner = postgresql_role.owner.name
}
