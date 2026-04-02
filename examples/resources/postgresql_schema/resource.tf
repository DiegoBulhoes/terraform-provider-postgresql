# Basic schema
resource "postgresql_schema" "app" {
  name = "app_schema"
}

# Schema with owner
resource "postgresql_schema" "owned" {
  name          = "app_schema"
  owner         = postgresql_role.owner.name
  if_not_exists = true
}
